package amplitude

import (
	"context"
	"errors"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
)

// mockClientAdapter is a mock implementation of clientAdapter for testing.
type mockClientAdapter struct {
	// StartFunc is called when Start is called. If nil, Start returns nil.
	StartFunc func() error
	// StopFunc is called when Stop is called. If nil, Stop returns nil.
	StopFunc func() error
	// EvaluateFunc is called when Evaluate is called.
	// If nil, Evaluate returns an empty map and nil error.
	EvaluateFunc func(ctx context.Context, user *experiment.User, flagKeys []string) (map[string]experiment.Variant, error)

	// startCalled tracks if Start was called.
	startCalled bool
	// stopCalled tracks if Stop was called.
	stopCalled bool
	// evaluateCalls tracks all calls to Evaluate.
	evaluateCalls []mockEvaluateCall
}

// mockEvaluateCall records the arguments to an Evaluate call.
type mockEvaluateCall struct {
	Ctx      context.Context
	User     *experiment.User
	FlagKeys []string
}

// Start implements clientAdapter.
func (m *mockClientAdapter) Start() error {
	m.startCalled = true
	if m.StartFunc != nil {
		return m.StartFunc()
	}
	return nil
}

// Stop implements clientAdapter.
func (m *mockClientAdapter) Stop() error {
	m.stopCalled = true
	if m.StopFunc != nil {
		return m.StopFunc()
	}
	return nil
}

// Evaluate implements clientAdapter.
func (m *mockClientAdapter) Evaluate(ctx context.Context, user *experiment.User, flagKeys []string) (map[string]experiment.Variant, error) {
	m.evaluateCalls = append(m.evaluateCalls, mockEvaluateCall{
		Ctx:      ctx,
		User:     user,
		FlagKeys: flagKeys,
	})
	if m.EvaluateFunc != nil {
		return m.EvaluateFunc(ctx, user, flagKeys)
	}
	return map[string]experiment.Variant{}, nil
}

// Verify mockClientAdapter implements clientAdapter.
var _ clientAdapter = (*mockClientAdapter)(nil)

// Common error for testing.
var errMockEvaluate = errors.New("mock evaluate error")
var errMockStart = errors.New("mock start error")

// Helper to create a variant with specific properties.
func makeVariant(key string, value string, payload any) experiment.Variant {
	return experiment.Variant{
		Key:     key,
		Value:   value,
		Payload: payload,
	}
}

