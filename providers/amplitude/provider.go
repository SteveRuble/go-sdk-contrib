// Package amplitude provides an OpenFeature provider implementation
// for the Amplitude Experiment SDK, supporting both local and remote evaluation.
//
// # Default "off" Variant Handling
//
// When a user is not included in a feature flag's rollout, Amplitude returns a
// variant with Key="off" and no payload. This provider detects this case and
// returns the caller-provided default value with Reason=DefaultReason.
//
// # Boolean Flag Evaluation
//
// For boolean flags, evaluation follows this precedence:
//  1. If the payload can be unmarshalled to a boolean, that value is used
//  2. Otherwise, falls back to variant key logic:
//     - If the variant key is "off", the default value is returned
//     - If the variant key is anything else, true is returned
//
// This allows explicit boolean payloads to control the value, while still
// supporting simple on/off toggles where any non-off variant means enabled.
//
// # Typed Values via Payload
//
// For non-boolean types (string, int64, float64, interface{}), this provider
// extracts flag values from the Amplitude Variant's Payload field, which contains
// the JSON-decoded value configured in the Amplitude Experiment console. The
// payload is unmarshalled to the expected type.
//
// # User Context Mapping
//
// The OpenFeature evaluation context is mapped to an Amplitude User as follows:
//   - TargetingKey, UserID, UserId, user_id -> User.UserId
//   - DeviceID, DeviceId, device_id -> User.DeviceId
//   - All other keys -> User.UserProperties
//
// Either UserId or DeviceId must be present for evaluation to succeed.
package amplitude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	of "github.com/open-feature/go-sdk/openfeature"
)

// Provider is an OpenFeature provider implementation for Amplitude.
type Provider struct {
	config Config
	state  of.State
	client clientAdapter
}

const (
	providerNotReady = "Amplitude provider not ready"
	generalError     = "Amplitude general error"

	// variantKeyOff is the variant key returned by Amplitude when a user
	// is not included in a feature flag's rollout.
	variantKeyOff = "off"
)

// New creates a new [Provider] from a deployment key and options.
func New(ctx context.Context, deploymentKey string, options ...Option) (*Provider, error) {
	config := Config{
		DeploymentKey: deploymentKey,
	}
	for _, option := range options {
		option(&config)
	}
	return NewFromConfig(ctx, config)
}

// NewFromConfig creates a new [Provider] from a [Config].
func NewFromConfig(_ context.Context, config Config) (*Provider, error) {
	if config.DeploymentKey == "" {
		return nil, errors.New("you must provide a deployment key")
	}

	provider := &Provider{
		state:  of.NotReadyState,
		config: config,
	}

	// Allow injecting a test client adapter for testing
	if config.testClientAdapter != nil {
		provider.client = config.testClientAdapter
		return provider, nil
	}

	switch {
	case config.LocalConfig != nil && config.RemoteConfig != nil:
		return nil, errors.New("you cannot configure the provider to use both local and remote evaluation at the same time")
	case config.RemoteConfig != nil:
		provider.client = newClientAdapterRemote(config.DeploymentKey, config.getRemoteConfig())
	default:
		provider.client = newClientAdapterLocal(config.DeploymentKey, config.getLocalConfig())
	}

	return provider, nil
}

// Init initializes the Amplitude Experiment provider.
// This must be called before using the provider.
// For local evaluation, this starts the flag config polling.
// For remote evaluation, this is a no-op as fetching happens per-request.
func (p *Provider) Init(_ of.EvaluationContext) error {
	// Only local client needs to be started
	startErr := p.client.Start()
	if startErr != nil {
		p.state = of.ErrorState
		return startErr
	}

	p.state = of.ReadyState
	return nil
}

// Shutdown shuts down the Amplitude Experiment provider.
// Note: The Amplitude local evaluation client does not have an explicit Close method.
// It manages its own lifecycle via internal goroutines.
func (p *Provider) Shutdown() {
	// TODO: Investigate if there's a way to properly stop the Amplitude client.
	// The local.Client doesn't expose a Stop/Close method in the current SDK version.
	p.state = of.NotReadyState
}

// Hooks returns empty slice as provider does not have any hooks.
func (p *Provider) Hooks() []of.Hook {
	return []of.Hook{}
}

// Metadata returns value of Metadata (name of current service, exposed to openfeature sdk).
func (p *Provider) Metadata() of.Metadata {
	return of.Metadata{
		Name: "Amplitude",
	}
}

// BooleanEvaluation evaluates a boolean feature flag.
// If the payload can be unmarshalled to a boolean, that value is used.
// Otherwise, falls back to variant key logic: "off" returns the default value,
// any other variant key returns true.
func (p *Provider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, evalCtx of.FlattenedContext) of.BoolResolutionDetail {
	variant, resErr := p.evaluateFlag(ctx, flag, evalCtx)
	if resErr != nil {
		return of.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: *resErr,
				Reason:          of.ErrorReason,
			},
		}
	}

	// nil variant indicates "off" - return default value
	if variant == nil || variant.Key == variantKeyOff {
		return of.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	// If the payload was a boolean, return it directly:
	if castType, ok := variant.Payload.(bool); ok {
		return of.BoolResolutionDetail{
			Value: castType,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	}

	// Any other variant value means "enabled", as documented in the README.md
	return of.BoolResolutionDetail{
		Value: true,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Variant:      variant.Key,
			FlagMetadata: variantMetadata(variant),
		},
	}
}

// StringEvaluation evaluates a string feature flag.
func (p *Provider) StringEvaluation(ctx context.Context, flag string, defaultValue string, evalCtx of.FlattenedContext) of.StringResolutionDetail {
	variant, resErr := p.evaluateFlag(ctx, flag, evalCtx)
	if resErr != nil {
		return of.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: *resErr,
				Reason:          of.ErrorReason,
			},
		}
	}

	// nil variant indicates "off" - return default value
	if variant == nil {
		return of.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	if castType, ok := variant.Payload.(string); ok {
		return of.StringResolutionDetail{
			Value: castType,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	}

	return of.StringResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason: of.DefaultReason,
		},
	}
}

// FloatEvaluation evaluates a float feature flag.
func (p *Provider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, evalCtx of.FlattenedContext) of.FloatResolutionDetail {
	variant, resErr := p.evaluateFlag(ctx, flag, evalCtx)
	if resErr != nil {
		return of.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: *resErr,
				Reason:          of.ErrorReason,
			},
		}
	}

	// nil variant indicates "off" - return default value
	if variant == nil {
		return of.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	// Extract the value from the payload:
	switch castType := variant.Payload.(type) {
	case float64:
		return of.FloatResolutionDetail{
			Value: castType,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	// The Amplitude SDK does not currently invoke `UseNumber` on the JSON decoder,
	// but if it starts doing it in the future we should handle it correctly.
	case json.Number:
		value, err := castType.Float64()
		if err != nil {
			return of.FloatResolutionDetail{
				Value: defaultValue,
				ProviderResolutionDetail: of.ProviderResolutionDetail{
					ResolutionError: of.NewTypeMismatchResolutionError(err.Error()),
					Reason:          of.ErrorReason,
				},
			}
		}
		return of.FloatResolutionDetail{
			Value: value,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	case nil:
		return of.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}
	return of.FloatResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason: of.ErrorReason,
			ResolutionError: of.NewTypeMismatchResolutionError(
				fmt.Sprintf("FloatEvaluation type error for %s, payload is %T "+
					"(automatically unmarshalled from JSON configured in the Amplitude console for this flag)",
					flag, variant.Payload)),
		},
	}
}

// IntEvaluation evaluates an integer feature flag.
func (p *Provider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, evalCtx of.FlattenedContext) of.IntResolutionDetail {
	variant, resErr := p.evaluateFlag(ctx, flag, evalCtx)
	if resErr != nil {
		return of.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: *resErr,
				Reason:          of.ErrorReason,
			},
		}
	}

	// nil variant indicates "off" - return default value
	if variant == nil {
		return of.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	switch castType := variant.Payload.(type) {
	// JSON numbers are automatically unmarshalled to float64,
	// so we need to convert them to int64.
	case float64:
		return of.IntResolutionDetail{
			Value: int64(castType),
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	// The Amplitude SDK does not currently invoke `UseNumber` on the JSON decoder,
	// but if it starts doing it in the future we should handle it correctly.
	case json.Number:
		value, err := castType.Int64()
		if err != nil {
			return of.IntResolutionDetail{
				Value: defaultValue,
				ProviderResolutionDetail: of.ProviderResolutionDetail{
					ResolutionError: of.NewTypeMismatchResolutionError(err.Error()),
					Reason:          of.ErrorReason,
				},
			}
		}
		return of.IntResolutionDetail{
			Value: value,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	// Sometimes users may need to represent a number as a string,
	// (e.g. to avoid floating point precision issues).
	// We should handle this correctly.
	case string:
		value, err := strconv.ParseInt(castType, 10, 64)
		if err != nil {
			return of.IntResolutionDetail{
				Value: defaultValue,
				ProviderResolutionDetail: of.ProviderResolutionDetail{
					ResolutionError: of.NewTypeMismatchResolutionError(err.Error()),
					Reason:          of.ErrorReason,
				},
			}
		}
		return of.IntResolutionDetail{
			Value: value,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	case nil:
		return of.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	return of.IntResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason: of.ErrorReason,
			ResolutionError: of.NewTypeMismatchResolutionError(
				fmt.Sprintf("IntEvaluation type error for %s, payload is %T "+
					"(automatically unmarshalled from JSON configured in the Amplitude console for this flag)",
					flag, variant.Payload)),
		},
	}
}

// ObjectEvaluation evaluates an object/JSON feature flag.
func (p *Provider) ObjectEvaluation(ctx context.Context, flag string, defaultValue any, evalCtx of.FlattenedContext) of.InterfaceResolutionDetail {
	variant, resErr := p.evaluateFlag(ctx, flag, evalCtx)
	if resErr != nil {
		return of.InterfaceResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: *resErr,
				Reason:          of.ErrorReason,
			},
		}
	}

	// nil variant indicates "off" - return default value
	if variant == nil {
		return of.InterfaceResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	// For object evaluation, return the payload directly as it's already the correct type.
	result := variant.Payload
	if result == nil {
		result = defaultValue
	}

	return of.InterfaceResolutionDetail{
		Value: result,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Variant:      variant.Key,
			FlagMetadata: variantMetadata(variant),
		},
	}
}

// evaluateFlag evaluates a flag for the given context and returns the variant.
// Returns nil variant (with no error) when the variant key is "off", indicating
// that the caller should use the default value.
// Returns a resolution error if something goes wrong.
func (p *Provider) evaluateFlag(ctx context.Context, flag string, evalCtx of.FlattenedContext) (*experiment.Variant, *of.ResolutionError) {
	if p.state != of.ReadyState {
		resErr := p.stateError()
		return nil, &resErr
	}

	user, userErr := p.toAmplitudeUser(evalCtx)
	if userErr != nil {
		resErr := of.NewInvalidContextResolutionError(userErr.Error())
		return nil, &resErr
	}

	variants, evalErr := p.client.Evaluate(ctx, user, []string{flag})
	if evalErr != nil {
		resErr := of.NewGeneralResolutionError(evalErr.Error())
		return nil, &resErr
	}

	variant, ok := variants[flag]
	if !ok {
		resErr := of.NewFlagNotFoundResolutionError(fmt.Sprintf("flag %s not found", flag))
		return nil, &resErr
	}

	// When variant key is "off", Amplitude indicates the user is not in the rollout.
	// Return nil to signal that the default value should be used.
	if variant.Key == variantKeyOff {
		return nil, nil
	}

	return &variant, nil
}

// stateError returns the appropriate resolution error based on provider state.
func (p *Provider) stateError() of.ResolutionError {
	if p.state == of.NotReadyState {
		return of.NewProviderNotReadyResolutionError(providerNotReady)
	}
	return of.NewGeneralResolutionError(generalError)
}

// variantMetadata returns the standard metadata for a variant.
func variantMetadata(variant *experiment.Variant) map[string]any {
	return map[string]any{
		"key":   variant.Key,
		"value": variant.Value,
	}
}

// toAmplitudeUser converts an OpenFeature evaluation context to an Amplitude User.
func (p *Provider) toAmplitudeUser(evalCtx of.FlattenedContext) (*experiment.User, error) {
	userMap := make(map[Key]any)
	keyMap := p.config.getKeyMap()
	for key, val := range evalCtx {
		resolvedKey, ok := keyMap[key]
		if ok {
			userMap[resolvedKey] = val
		} else {
			userProperties, ok := userMap[KeyUserProperties].(map[string]any)
			if !ok {
				userProperties = make(map[string]any)
				userMap[KeyUserProperties] = userProperties
			}
			userProperties[key] = val
		}
	}
	userMapJSON, err := json.Marshal(userMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user map: %w", err)
	}

	var user experiment.User
	err = json.Unmarshal(userMapJSON, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user map: %w", err)
	}

	if user.UserId == "" && user.DeviceId == "" {
		return nil, fmt.Errorf("context must contain a %s, %s, or %s", of.TargetingKey, KeyUserID, KeyDeviceID)
	}

	return &user, nil
}
