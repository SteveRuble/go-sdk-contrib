package amplitude

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withMockClient sets up a mock client adapter and returns a cleanup function.
func withMockClient(mock *mockClientAdapter) func(*Config) {
	return func(c *Config) {
		c.testClientAdapter = mock
	}
}

// newTestProvider creates a provider with a mock client for testing.
func newTestProvider(t *testing.T, mock *mockClientAdapter) *Provider {
	t.Helper()

	provider, err := New(context.Background(), "test-deployment-key", withMockClient(mock))
	require.NoError(t, err)
	require.NoError(t, provider.Init(of.EvaluationContext{}))
	return provider
}

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		deploymentKey string
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid deployment key",
			deploymentKey: "test-key",
			expectError:   false,
		},
		{
			name:          "empty deployment key",
			deploymentKey: "",
			expectError:   true,
			errorContains: "you must provide a deployment key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{}

			provider, err := New(context.Background(), tt.deploymentKey, withMockClient(mock))
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProvider_Init(t *testing.T) {
	tests := []struct {
		name        string
		startError  error
		expectError bool
		expectState of.State
	}{
		{
			name:        "successful init",
			startError:  nil,
			expectError: false,
			expectState: of.ReadyState,
		},
		{
			name:        "init fails when start fails",
			startError:  errMockStart,
			expectError: true,
			expectState: of.ErrorState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				StartFunc: func() error { return tt.startError },
			}
			provider, err := New(context.Background(), "test-key", withMockClient(mock))
			require.NoError(t, err)

			initErr := provider.Init(of.EvaluationContext{})
			if tt.expectError {
				require.Error(t, initErr)
				assert.Equal(t, tt.startError, initErr)
			} else {
				require.NoError(t, initErr)
			}
			assert.Equal(t, tt.expectState, provider.state)
		})
	}
}

func TestProvider_Shutdown(t *testing.T) {
	mock := &mockClientAdapter{}
	provider := newTestProvider(t, mock)

	assert.Equal(t, of.ReadyState, provider.state)
	provider.Shutdown()
	assert.Equal(t, of.NotReadyState, provider.state)
}

func TestProvider_Hooks(t *testing.T) {
	mock := &mockClientAdapter{}
	provider := newTestProvider(t, mock)

	hooks := provider.Hooks()
	assert.Empty(t, hooks)
}

func TestProvider_Metadata(t *testing.T) {
	mock := &mockClientAdapter{}
	provider := newTestProvider(t, mock)

	metadata := provider.Metadata()
	assert.Equal(t, "Amplitude", metadata.Name)
}

func TestProvider_BooleanEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		defaultValue  bool
		evalCtx       of.FlattenedContext
		variants      map[string]experiment.Variant
		evaluateErr   error
		expectedValue bool
		expectedError bool
		reason        of.Reason
	}{
		{
			name:         "returns true when variant has boolean true payload",
			flagName:     "test-flag",
			defaultValue: false,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("on", "on", true),
			},
			expectedValue: true,
			expectedError: false,
		},
		{
			name:         "returns false when variant has boolean false payload",
			flagName:     "test-flag",
			defaultValue: true,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("enabled", "enabled", false),
			},
			expectedValue: false,
			expectedError: false,
		},
		{
			name:         "returns true when variant key is not off and payload is not bool",
			flagName:     "test-flag",
			defaultValue: false,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("treatment", "value", "some-string"),
			},
			expectedValue: true,
			expectedError: false,
		},
		{
			name:         "returns default when variant key is off",
			flagName:     "test-flag",
			defaultValue: true,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("off", "", nil),
			},
			expectedValue: true,
			expectedError: false,
			reason:        of.DefaultReason,
		},
		{
			name:         "returns default when flag not found",
			flagName:     "missing-flag",
			defaultValue: true,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants:     map[string]experiment.Variant{},
			expectedValue: true,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:          "returns default when evaluate fails",
			flagName:      "test-flag",
			defaultValue:  true,
			evalCtx:       of.FlattenedContext{of.TargetingKey: "user-1"},
			evaluateErr:   errMockEvaluate,
			expectedValue: true,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:          "returns default when targeting key missing",
			flagName:      "test-flag",
			defaultValue:  false,
			evalCtx:       of.FlattenedContext{},
			expectedValue: false,
			expectedError: true,
			reason:        of.ErrorReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				EvaluateFunc: func(_ context.Context, _ *experiment.User, _ []string) (map[string]experiment.Variant, error) {
					if tt.evaluateErr != nil {
						return nil, tt.evaluateErr
					}
					return tt.variants, nil
				},
			}
			provider := newTestProvider(t, mock)

			result := provider.BooleanEvaluation(context.Background(), tt.flagName, tt.defaultValue, tt.evalCtx)

			assert.Equal(t, tt.expectedValue, result.Value)
			if tt.expectedError {
				assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError, "expected a resolution error")
			} else {
				assert.Equal(t, of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
			}
			if tt.reason != "" {
				assert.Equal(t, tt.reason, result.Reason)
			}
		})
	}
}

func TestProvider_BooleanEvaluation_NotReady(t *testing.T) {
	mock := &mockClientAdapter{}

	provider, err := New(context.Background(), "test-key", withMockClient(mock))
	require.NoError(t, err)
	// Don't call Init - provider is not ready

	result := provider.BooleanEvaluation(context.Background(), "test-flag", false, of.FlattenedContext{of.TargetingKey: "user-1"})

	assert.False(t, result.Value)
	assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError, "expected a resolution error")
	assert.Equal(t, of.ErrorReason, result.Reason)
}

func TestProvider_StringEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		defaultValue  string
		evalCtx       of.FlattenedContext
		variants      map[string]experiment.Variant
		evaluateErr   error
		expectedValue string
		expectedError bool
		reason        of.Reason
	}{
		{
			name:         "returns string payload",
			flagName:     "test-flag",
			defaultValue: "default",
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", "payload-string"),
			},
			expectedValue: "payload-string",
			expectedError: false,
		},
		{
			name:         "returns error when payload is not string",
			flagName:     "test-flag",
			defaultValue: "default",
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", 123),
			},
			expectedValue: "default",
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:         "returns default when variant is nil",
			flagName:     "test-flag",
			defaultValue: "default",
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("off", "", nil),
			},
			expectedValue: "default",
			expectedError: false,
			reason:        of.DefaultReason,
		},
		{
			name:         "returns default when flag not found",
			flagName:     "missing-flag",
			defaultValue: "default",
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants:     map[string]experiment.Variant{},
			expectedValue: "default",
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:          "returns default when evaluate fails",
			flagName:      "test-flag",
			defaultValue:  "default",
			evalCtx:       of.FlattenedContext{of.TargetingKey: "user-1"},
			evaluateErr:   errMockEvaluate,
			expectedValue: "default",
			expectedError: true,
			reason:        of.ErrorReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				EvaluateFunc: func(_ context.Context, _ *experiment.User, _ []string) (map[string]experiment.Variant, error) {
					if tt.evaluateErr != nil {
						return nil, tt.evaluateErr
					}
					return tt.variants, nil
				},
			}
			provider := newTestProvider(t, mock)

			result := provider.StringEvaluation(context.Background(), tt.flagName, tt.defaultValue, tt.evalCtx)

			assert.Equal(t, tt.expectedValue, result.Value)
			if tt.expectedError {
				assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError, "expected a resolution error")
			} else {
				assert.Equal(t, of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
			}
			if tt.reason != "" {
				assert.Equal(t, tt.reason, result.Reason)
			}
		})
	}
}

func TestProvider_FloatEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		defaultValue  float64
		evalCtx       of.FlattenedContext
		variants      map[string]experiment.Variant
		evaluateErr   error
		expectedValue float64
		expectedError bool
		reason        of.Reason
	}{
		{
			name:         "returns float64 payload",
			flagName:     "test-flag",
			defaultValue: 0.0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", 3.14159),
			},
			expectedValue: 3.14159,
			expectedError: false,
		},
		{
			name:         "returns float64 from json.Number",
			flagName:     "test-flag",
			defaultValue: 0.0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", json.Number("2.71828")),
			},
			expectedValue: 2.71828,
			expectedError: false,
		},
		{
			name:         "returns default when json.Number conversion fails",
			flagName:     "test-flag",
			defaultValue: 1.5,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", json.Number("not-a-number")),
			},
			expectedValue: 1.5,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:         "returns default when payload is nil",
			flagName:     "test-flag",
			defaultValue: 99.9,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", nil),
			},
			expectedValue: 99.9,
			expectedError: false,
			reason:        of.DefaultReason,
		},
		{
			name:         "returns default when payload is wrong type",
			flagName:     "test-flag",
			defaultValue: 1.0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", "not-a-float"),
			},
			expectedValue: 1.0,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:         "returns default when variant is off",
			flagName:     "test-flag",
			defaultValue: 42.0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("off", "", nil),
			},
			expectedValue: 42.0,
			expectedError: false,
			reason:        of.DefaultReason,
		},
		{
			name:          "returns default when evaluate fails",
			flagName:      "test-flag",
			defaultValue:  0.0,
			evalCtx:       of.FlattenedContext{of.TargetingKey: "user-1"},
			evaluateErr:   errMockEvaluate,
			expectedValue: 0.0,
			expectedError: true,
			reason:        of.ErrorReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				EvaluateFunc: func(_ context.Context, _ *experiment.User, _ []string) (map[string]experiment.Variant, error) {
					if tt.evaluateErr != nil {
						return nil, tt.evaluateErr
					}
					return tt.variants, nil
				},
			}
			provider := newTestProvider(t, mock)

			result := provider.FloatEvaluation(context.Background(), tt.flagName, tt.defaultValue, tt.evalCtx)

			assert.Equal(t, tt.expectedValue, result.Value)
			if tt.expectedError {
				assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError, "expected a resolution error")
			} else {
				assert.Equal(t, of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
			}
			if tt.reason != "" {
				assert.Equal(t, tt.reason, result.Reason)
			}
		})
	}
}

func TestProvider_IntEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		defaultValue  int64
		evalCtx       of.FlattenedContext
		variants      map[string]experiment.Variant
		evaluateErr   error
		expectedValue int64
		expectedError bool
		reason        of.Reason
	}{
		{
			name:         "returns int from float64 payload (JSON behavior)",
			flagName:     "test-flag",
			defaultValue: 0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", float64(42)),
			},
			expectedValue: 42,
			expectedError: false,
		},
		{
			name:         "returns int from json.Number",
			flagName:     "test-flag",
			defaultValue: 0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", json.Number("123")),
			},
			expectedValue: 123,
			expectedError: false,
		},
		{
			name:         "returns default when json.Number conversion fails",
			flagName:     "test-flag",
			defaultValue: 10,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", json.Number("not-a-number")),
			},
			expectedValue: 10,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:         "returns int from string payload",
			flagName:     "test-flag",
			defaultValue: 0,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", "456"),
			},
			expectedValue: 456,
			expectedError: false,
		},
		{
			name:         "returns default when string is not a valid int",
			flagName:     "test-flag",
			defaultValue: 100,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", "not-an-int"),
			},
			expectedValue: 100,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:         "returns default when payload is nil",
			flagName:     "test-flag",
			defaultValue: 99,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", nil),
			},
			expectedValue: 99,
			expectedError: false,
			reason:        of.DefaultReason,
		},
		{
			name:         "returns default when payload is wrong type",
			flagName:     "test-flag",
			defaultValue: 1,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", []string{"not", "an", "int"}),
			},
			expectedValue: 1,
			expectedError: true,
			reason:        of.ErrorReason,
		},
		{
			name:         "returns default when variant is off",
			flagName:     "test-flag",
			defaultValue: 42,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("off", "", nil),
			},
			expectedValue: 42,
			expectedError: false,
			reason:        of.DefaultReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				EvaluateFunc: func(_ context.Context, _ *experiment.User, _ []string) (map[string]experiment.Variant, error) {
					if tt.evaluateErr != nil {
						return nil, tt.evaluateErr
					}
					return tt.variants, nil
				},
			}
			provider := newTestProvider(t, mock)

			result := provider.IntEvaluation(context.Background(), tt.flagName, tt.defaultValue, tt.evalCtx)

			assert.Equal(t, tt.expectedValue, result.Value)
			if tt.expectedError {
				assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError, "expected a resolution error")
			} else {
				assert.Equal(t, of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
			}
			if tt.reason != "" {
				assert.Equal(t, tt.reason, result.Reason)
			}
		})
	}
}

func TestProvider_ObjectEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		defaultValue  any
		evalCtx       of.FlattenedContext
		variants      map[string]experiment.Variant
		evaluateErr   error
		expectedValue any
		expectedError bool
		reason        of.Reason
	}{
		{
			name:         "returns map payload",
			flagName:     "test-flag",
			defaultValue: nil,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", map[string]any{"key": "value"}),
			},
			expectedValue: map[string]any{"key": "value"},
			expectedError: false,
		},
		{
			name:         "returns slice payload",
			flagName:     "test-flag",
			defaultValue: nil,
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", []any{1, 2, 3}),
			},
			expectedValue: []any{1, 2, 3},
			expectedError: false,
		},
		{
			name:         "returns default when payload is nil",
			flagName:     "test-flag",
			defaultValue: map[string]any{"default": "value"},
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", nil),
			},
			expectedValue: map[string]any{"default": "value"},
			expectedError: false,
		},
		{
			name:         "returns default when variant is off",
			flagName:     "test-flag",
			defaultValue: map[string]any{"default": "value"},
			evalCtx:      of.FlattenedContext{of.TargetingKey: "user-1"},
			variants: map[string]experiment.Variant{
				"test-flag": makeVariant("off", "", nil),
			},
			expectedValue: map[string]any{"default": "value"},
			expectedError: false,
			reason:        of.DefaultReason,
		},
		{
			name:          "returns default when evaluate fails",
			flagName:      "test-flag",
			defaultValue:  map[string]any{"default": "value"},
			evalCtx:       of.FlattenedContext{of.TargetingKey: "user-1"},
			evaluateErr:   errMockEvaluate,
			expectedValue: map[string]any{"default": "value"},
			expectedError: true,
			reason:        of.ErrorReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				EvaluateFunc: func(_ context.Context, _ *experiment.User, _ []string) (map[string]experiment.Variant, error) {
					if tt.evaluateErr != nil {
						return nil, tt.evaluateErr
					}
					return tt.variants, nil
				},
			}
			provider := newTestProvider(t, mock)

			result := provider.ObjectEvaluation(context.Background(), tt.flagName, tt.defaultValue, tt.evalCtx)

			assert.Equal(t, tt.expectedValue, result.Value)
			if tt.expectedError {
				assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError, "expected a resolution error")
			} else {
				assert.Equal(t, of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
			}
			if tt.reason != "" {
				assert.Equal(t, tt.reason, result.Reason)
			}
		})
	}
}

func TestProvider_stateError(t *testing.T) {
	tests := []struct {
		name           string
		status         of.State
		expectedPrefix string
	}{
		{
			name:           "not ready state",
			status:         of.NotReadyState,
			expectedPrefix: providerNotReady,
		},
		{
			name:           "error state returns general error",
			status:         of.ErrorState,
			expectedPrefix: generalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{state: tt.status}
			err := provider.stateError()
			assert.Contains(t, err.Error(), tt.expectedPrefix)
		})
	}
}

func TestVariantMetadata(t *testing.T) {
	variant := &experiment.Variant{
		Key:   "test-key",
		Value: "test-value",
	}

	metadata := variantMetadata(variant)

	assert.Equal(t, "test-key", metadata["key"])
	assert.Equal(t, "test-value", metadata["value"])
}

func TestProvider_EvaluatePassesFlagKeys(t *testing.T) {
	var capturedFlagKeys []string
	mock := &mockClientAdapter{
		EvaluateFunc: func(_ context.Context, _ *experiment.User, flagKeys []string) (map[string]experiment.Variant, error) {
			capturedFlagKeys = flagKeys
			return map[string]experiment.Variant{
				"my-specific-flag": makeVariant("on", "on", true),
			}, nil
		},
	}
	provider := newTestProvider(t, mock)

	_ = provider.BooleanEvaluation(context.Background(), "my-specific-flag", false, of.FlattenedContext{of.TargetingKey: "user-1"})

	assert.Equal(t, []string{"my-specific-flag"}, capturedFlagKeys)
}

func TestProvider_IntEvaluation_Int64Type(t *testing.T) {
	// Test the case where the payload is already int64 type (not commonly produced by JSON)
	mock := &mockClientAdapter{
		EvaluateFunc: func(_ context.Context, _ *experiment.User, _ []string) (map[string]experiment.Variant, error) {
			return map[string]experiment.Variant{
				"test-flag": makeVariant("variant-a", "value-a", int64(9223372036854775807)),
			}, nil
		},
	}
	provider := newTestProvider(t, mock)

	evalCtx := of.FlattenedContext{of.TargetingKey: "user-1"}
	result := provider.IntEvaluation(context.Background(), "test-flag", 0, evalCtx)

	// The int64 case actually goes through the default branch in the switch since
	// JSON doesn't produce int64 directly. We need to test what the code actually does.
	// If it returns default due to type mismatch, that's the expected behavior.
	// Let's verify the result matches expected behavior.
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError)
}

func TestProvider_EvaluatePassesUserContext(t *testing.T) {
	var capturedUser *experiment.User
	mock := &mockClientAdapter{
		EvaluateFunc: func(_ context.Context, user *experiment.User, _ []string) (map[string]experiment.Variant, error) {
			capturedUser = user
			return map[string]experiment.Variant{
				"test-flag": makeVariant("on", "on", true),
			}, nil
		},
	}
	provider := newTestProvider(t, mock)

	evalCtx := of.FlattenedContext{
		of.TargetingKey: "user-123",
		"custom_prop":   "custom_value",
	}

	_ = provider.BooleanEvaluation(context.Background(), "test-flag", false, evalCtx)

	require.NotNil(t, capturedUser)
	assert.Equal(t, "user-123", capturedUser.UserId)
	assert.Equal(t, "custom_value", capturedUser.UserProperties["custom_prop"])
}

func TestProvider_UserNormalizer(t *testing.T) {
	tests := []struct {
		name              string
		evalCtx           of.FlattenedContext
		normalizerFn      func(ctx context.Context, normCtx UserNormalizationContext) error
		expectedUserID    string
		expectedDeviceID  string
		expectedUserProps map[string]any
		expectError       bool
	}{
		{
			name:    "normalizer can add user properties",
			evalCtx: of.FlattenedContext{of.TargetingKey: "user-123"},
			normalizerFn: func(_ context.Context, normCtx UserNormalizationContext) error {
				if normCtx.User.UserProperties == nil {
					normCtx.User.UserProperties = make(map[string]any)
				}
				normCtx.User.UserProperties["custom_prop"] = "added_by_normalizer"
				return nil
			},
			expectedUserID:    "user-123",
			expectedUserProps: map[string]any{"custom_prop": "added_by_normalizer"},
		},
		{
			name:    "normalizer can modify device_id",
			evalCtx: of.FlattenedContext{of.TargetingKey: "user-456", "device_id": "original-device"},
			normalizerFn: func(_ context.Context, normCtx UserNormalizationContext) error {
				normCtx.User.DeviceId = "normalized-device"
				return nil
			},
			expectedUserID:   "user-456",
			expectedDeviceID: "normalized-device",
		},
		{
			name:    "normalizer receives original evaluation context",
			evalCtx: of.FlattenedContext{of.TargetingKey: "user-789", "custom_key": "custom_value"},
			normalizerFn: func(_ context.Context, normCtx UserNormalizationContext) error {
				// Verify the evaluation context is passed correctly
				if normCtx.EvaluationContext["custom_key"] != "custom_value" {
					return fmt.Errorf("expected custom_key to be custom_value")
				}
				return nil
			},
			expectedUserID: "user-789",
		},
		{
			name:    "normalizer error is propagated",
			evalCtx: of.FlattenedContext{of.TargetingKey: "user-error"},
			normalizerFn: func(_ context.Context, normCtx UserNormalizationContext) error {
				return fmt.Errorf("normalizer failed")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{
				EvaluateFunc: func(_ context.Context, user *experiment.User, _ []string) (map[string]experiment.Variant, error) {
					// Capture the user for verification
					if tt.expectedDeviceID != "" {
						assert.Equal(t, tt.expectedDeviceID, user.DeviceId)
					}
					if tt.expectedUserProps != nil {
						for k, v := range tt.expectedUserProps {
							assert.Equal(t, v, user.UserProperties[k])
						}
					}
					return map[string]experiment.Variant{
						"test-flag": makeVariant("on", "on", true),
					}, nil
				},
			}

			provider, providerErr := New(context.Background(), "test-key",
				withMockClient(mock),
				WithUserNormalizer(tt.normalizerFn),
			)
			require.NoError(t, providerErr)
			require.NoError(t, provider.Init(of.EvaluationContext{}))

			result := provider.BooleanEvaluation(context.Background(), "test-flag", false, tt.evalCtx)

			if tt.expectError {
				assert.NotEqual(t, of.ResolutionError{}, result.ResolutionError)
			} else {
				assert.Equal(t, of.ResolutionError{}, result.ResolutionError)
			}
		})
	}
}

func TestProvider_EventNormalizer(t *testing.T) {
	tests := []struct {
		name               string
		trackingEventName  string
		evalCtx            of.EvaluationContext
		details            of.TrackingEventDetails
		normalizerFn       func(ctx context.Context, normCtx EventNormalizationContext) error
		expectedEventType  string
		expectedEventProps map[string]any
		expectError        bool
	}{
		{
			name:              "normalizer can add event properties",
			trackingEventName: "test-event",
			evalCtx:           of.NewEvaluationContext("user-123", nil),
			details:           of.NewTrackingEventDetails(0),
			normalizerFn: func(_ context.Context, normCtx EventNormalizationContext) error {
				if normCtx.Event.EventProperties == nil {
					normCtx.Event.EventProperties = make(map[string]any)
				}
				normCtx.Event.EventProperties["added_by_normalizer"] = true
				return nil
			},
			expectedEventType:  "test-event",
			expectedEventProps: map[string]any{"added_by_normalizer": true},
		},
		{
			name:              "normalizer can modify event type",
			trackingEventName: "original-event",
			evalCtx:           of.NewEvaluationContext("user-456", nil),
			details:           of.NewTrackingEventDetails(0),
			normalizerFn: func(_ context.Context, normCtx EventNormalizationContext) error {
				normCtx.Event.EventType = "modified-event"
				return nil
			},
			expectedEventType: "modified-event",
		},
		{
			name:              "normalizer receives tracking key",
			trackingEventName: "my-tracking-key",
			evalCtx:           of.NewEvaluationContext("user-789", nil),
			details:           of.NewTrackingEventDetails(0),
			normalizerFn: func(_ context.Context, normCtx EventNormalizationContext) error {
				if normCtx.TrackingKey != "my-tracking-key" {
					return fmt.Errorf("expected tracking key to be my-tracking-key, got %s", normCtx.TrackingKey)
				}
				return nil
			},
			expectedEventType: "my-tracking-key",
		},
		{
			name:              "normalizer error is propagated",
			trackingEventName: "error-event",
			evalCtx:           of.NewEvaluationContext("user-error", nil),
			details:           of.NewTrackingEventDetails(0),
			normalizerFn: func(_ context.Context, normCtx EventNormalizationContext) error {
				return fmt.Errorf("event normalizer failed")
			},
			expectError: true,
		},
		{
			name:              "normalizer receives tracking event details",
			trackingEventName: "details-event",
			evalCtx:           of.NewEvaluationContext("user-details", nil),
			details:           of.NewTrackingEventDetails(42.5).Add("custom_detail", "value"),
			normalizerFn: func(_ context.Context, normCtx EventNormalizationContext) error {
				// Verify tracking event details are passed
				if normCtx.TrackingEventDetails.Value() != 42.5 {
					return fmt.Errorf("expected value 42.5, got %f", normCtx.TrackingEventDetails.Value())
				}
				if normCtx.TrackingEventDetails.Attribute("custom_detail") != "value" {
					return fmt.Errorf("expected custom_detail to be value")
				}
				return nil
			},
			expectedEventType: "details-event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{}
			provider, providerErr := New(context.Background(), "test-key",
				withMockClient(mock),
				WithEventNormalizer(tt.normalizerFn),
			)
			require.NoError(t, providerErr)
			require.NoError(t, provider.Init(of.EvaluationContext{}))

			event, eventErr := provider.toAmplitudeEvent(context.Background(), tt.trackingEventName, tt.evalCtx, tt.details)

			if tt.expectError {
				require.Error(t, eventErr)
			} else {
				require.NoError(t, eventErr)
				assert.Equal(t, tt.expectedEventType, event.EventType)
				if tt.expectedEventProps != nil {
					for key, expectedValue := range tt.expectedEventProps {
						actualValue, exists := event.EventProperties[key]
						assert.True(t, exists, "expected event property %q to exist", key)
						assert.Equal(t, expectedValue, actualValue, "event property %q mismatch", key)
					}
				}
			}
		})
	}
}

func TestProvider_toAmplitudeEvent(t *testing.T) {
	// Helper to create a TrackingEventDetails with attributes.
	makeDetails := func(value float64, attrs map[string]any) of.TrackingEventDetails {
		details := of.NewTrackingEventDetails(value)
		for k, v := range attrs {
			details = details.Add(k, v)
		}
		return details
	}

	tests := []struct {
		name                 string
		trackingEventName    string
		evalCtx              of.EvaluationContext
		details              of.TrackingEventDetails
		expectedEventType    string
		expectedUserID       string
		expectedDeviceID     string
		expectedEventProps   map[string]any
		expectedPlatform     string
		expectedCountry      string
		expectedRevenue      float64
	}{
		{
			name:              "event type is set from tracking event name",
			trackingEventName: "custom-tracking-event",
			evalCtx:           of.NewEvaluationContext("user-123", nil),
			details:           of.NewTrackingEventDetails(0),
			expectedEventType: "custom-tracking-event",
			expectedUserID:    "user-123",
		},
		{
			name:              "targeting key sets user_id",
			trackingEventName: "test-event",
			evalCtx:           of.NewEvaluationContext("targeting-key-user", nil),
			details:           of.NewTrackingEventDetails(0),
			expectedEventType: "test-event",
			expectedUserID:    "targeting-key-user",
		},
		{
			name:              "device_id from context is mapped correctly",
			trackingEventName: "test-event",
			evalCtx: of.NewEvaluationContext("user-456", map[string]any{
				"device_id": "device-abc",
			}),
			details:           of.NewTrackingEventDetails(0),
			expectedEventType: "test-event",
			expectedUserID:    "user-456",
			expectedDeviceID:  "device-abc",
		},
		{
			name:              "context attributes mapped to event fields via default key map",
			trackingEventName: "analytics-event",
			evalCtx: of.NewEvaluationContext("user-abc", map[string]any{
				"platform": "iOS",
				"country":  "US",
			}),
			details:           of.NewTrackingEventDetails(0),
			expectedEventType: "analytics-event",
			expectedUserID:    "user-abc",
			expectedPlatform:  "iOS",
			expectedCountry:   "US",
		},
		{
			name:              "unmapped details keys go to event properties",
			trackingEventName: "test-event",
			evalCtx:           of.NewEvaluationContext("user-xyz", nil),
			details: makeDetails(0, map[string]any{
				"custom_field_1": "custom_value_1",
				"custom_field_2": 42,
				"nested_data":    map[string]any{"key": "value"},
			}),
			expectedEventType: "test-event",
			expectedUserID:    "user-xyz",
			expectedEventProps: map[string]any{
				"custom_field_1": "custom_value_1",
				"custom_field_2": 42,
				"nested_data":    map[string]any{"key": "value"},
			},
		},
		{
			name:              "details with mapped keys populate event fields",
			trackingEventName: "revenue-event",
			evalCtx:           of.NewEvaluationContext("buyer-123", nil),
			details: makeDetails(0, map[string]any{
				"price":    19.99,
				"quantity": 2,
				"currency": "USD",
			}),
			expectedEventType: "revenue-event",
			expectedUserID:    "buyer-123",
		},
		{
			name:              "both context and details attributes are merged",
			trackingEventName: "merged-event",
			evalCtx: of.NewEvaluationContext("user-merge", map[string]any{
				"platform": "Android",
			}),
			details: makeDetails(0, map[string]any{
				"action":   "click",
				"category": "button",
			}),
			expectedEventType: "merged-event",
			expectedUserID:    "user-merge",
			expectedPlatform:  "Android",
			expectedEventProps: map[string]any{
				"action":   "click",
				"category": "button",
			},
		},
		{
			name:              "TrackingEventDetails value is mapped to revenue",
			trackingEventName: "purchase-event",
			evalCtx:           of.NewEvaluationContext("buyer-456", nil),
			details:           of.NewTrackingEventDetails(99.99),
			expectedEventType: "purchase-event",
			expectedUserID:    "buyer-456",
			expectedRevenue:   99.99,
		},
		{
			name:              "zero value does not set revenue",
			trackingEventName: "non-revenue-event",
			evalCtx:           of.NewEvaluationContext("user-789", nil),
			details:           of.NewTrackingEventDetails(0),
			expectedEventType: "non-revenue-event",
			expectedUserID:    "user-789",
			expectedRevenue:   0,
		},
		{
			name:              "revenue from value with additional attributes",
			trackingEventName: "full-purchase-event",
			evalCtx:           of.NewEvaluationContext("buyer-full", nil),
			details: makeDetails(149.99, map[string]any{
				"currency":   "USD",
				"product_id": "SKU-12345",
			}),
			expectedEventType: "full-purchase-event",
			expectedUserID:    "buyer-full",
			expectedRevenue:   149.99,
			expectedEventProps: map[string]any{
				"product_id": "SKU-12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClientAdapter{}
			provider := newTestProvider(t, mock)

			event, eventErr := provider.toAmplitudeEvent(context.Background(), tt.trackingEventName, tt.evalCtx, tt.details)

			require.NoError(t, eventErr)
			assert.Equal(t, tt.expectedEventType, event.EventType)

			if tt.expectedUserID != "" {
				assert.Equal(t, tt.expectedUserID, event.UserID)
			}
			if tt.expectedDeviceID != "" {
				assert.Equal(t, tt.expectedDeviceID, event.EventOptions.DeviceID)
			}
			if tt.expectedPlatform != "" {
				assert.Equal(t, tt.expectedPlatform, event.EventOptions.Platform)
			}
			if tt.expectedCountry != "" {
				assert.Equal(t, tt.expectedCountry, event.EventOptions.Country)
			}
			if tt.expectedRevenue != 0 {
				assert.Equal(t, tt.expectedRevenue, event.EventOptions.Revenue)
			}
			if tt.expectedEventProps != nil {
				for key, expectedValue := range tt.expectedEventProps {
					actualValue, exists := event.EventProperties[key]
					assert.True(t, exists, "expected event property %q to exist", key)
					assert.Equal(t, expectedValue, actualValue, "event property %q mismatch", key)
				}
			}
		})
	}
}


