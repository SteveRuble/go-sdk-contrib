package amplitude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	analytics "github.com/amplitude/analytics-go/amplitude"
	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
	"github.com/amplitude/experiment-go-server/pkg/logger"
	of "github.com/open-feature/go-sdk/openfeature"
)

// Compile-time interface checks.
var (
	_ of.FeatureProvider = (*Provider)(nil)
	_ of.StateHandler    = (*Provider)(nil)
	_ of.Tracker         = (*Provider)(nil)
)

// Provider is an OpenFeature provider implementation for Amplitude.
type Provider struct {
	config            Config
	state             of.State
	evaluationContext of.EvaluationContext
	client            clientAdapter
	logger            *logger.Logger
	analyticsClient   analytics.Client
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
		provider.logger = logger.New(config.RemoteConfig.LogLevel, config.RemoteConfig.LoggerProvider)
	default:
		localCfg := config.getLocalConfig()
		// Ensure that if the user provided an analytics config, 
		// we use it for the assignment config no matter how the user configured it
		if config.AnalyticsConfig == nil && localCfg.AssignmentConfig != nil {
			config.AnalyticsConfig = &analytics.Config{}
		} else if config.AnalyticsConfig != nil && localCfg.AssignmentConfig == nil {
			localCfg.AssignmentConfig = &local.AssignmentConfig{
				Config: *config.AnalyticsConfig,
			}
		}
		provider.client = newClientAdapterLocal(config.DeploymentKey, config.getLocalConfig())
		provider.logger = logger.New(config.LocalConfig.LogLevel, config.LocalConfig.LoggerProvider)
	}

	if provider.logger == nil {
		provider.logger = logger.New(logger.Error, logger.NewDefault())
	}

	if provider.config.AnalyticsConfig != nil {
		provider.analyticsClient = analytics.NewClient(*provider.config.AnalyticsConfig)
	}

	return provider, nil
}

// Init initializes the Amplitude Experiment provider.
// This must be called before using the provider.
// For local evaluation, this starts the flag config polling.
// For remote evaluation, this is a no-op as fetching happens per-request.
// The evaluation context passed is not used by this provider.
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

// Status returns the current state of the provider.
func (p *Provider) Status() of.State {
	return p.state
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

	switch castType := variant.Payload.(type) {
	case string:
		return of.StringResolutionDetail{
			Value: castType,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Variant:      variant.Key,
				FlagMetadata: variantMetadata(variant),
			},
		}
	case nil:
		return of.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason: of.DefaultReason,
			},
		}
	}

	return of.StringResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason: of.ErrorReason,
			ResolutionError: of.NewTypeMismatchResolutionError(
				fmt.Sprintf("StringEvaluation type error for %s, payload is %T "+
					"(automatically unmarshalled from JSON configured in the Amplitude console for this flag)",
					flag, variant.Payload)),
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

// Track sends a tracking event to Amplitude. This implements the [of.Tracker] interface.
// If the analytics client is not configured, this is a no-op.
func (p *Provider) Track(ctx context.Context, trackingEventName string, evalCtx of.EvaluationContext, details of.TrackingEventDetails) {

	if p.analyticsClient == nil {
		return
	}

	event, err := p.toAmplitudeEvent(ctx, trackingEventName, evalCtx, details)
	if err != nil {
		p.logger.Error("failed to create event: %w", err)
		return
	}

	p.analyticsClient.Track(event)
}

func (p *Provider) toAmplitudeEvent(ctx context.Context, trackingEventName string, evalCtx of.EvaluationContext, details of.TrackingEventDetails) (analytics.Event, error) {
	attributes := evalCtx.Attributes()
	if evalCtx.TargetingKey() != "" {
		attributes[string(KeyUserID)] = evalCtx.TargetingKey()
	}

	var event analytics.Event

	eventMap, _ := p.normalizeContext(attributes)
	eventMapJSON, err := json.Marshal(eventMap)
	if err != nil {
		return event, fmt.Errorf("failed to marshal event map: %w", err)
	}

	err = json.Unmarshal(eventMapJSON, &event)
	if err != nil {
		return event, fmt.Errorf("failed to unmarshal event map: %w", err)
	}

	detailsMap, extraEventProperties  := p.normalizeContext(details.Attributes())
	detailsMapJSON, err := json.Marshal(detailsMap)
	if err != nil {
		return event, fmt.Errorf("failed to marshal details map: %w", err)	
	}
	err = json.Unmarshal(detailsMapJSON, &event)
	if err != nil {
		return event, fmt.Errorf("failed to unmarshal event details map: %w", err)
	}
	if event.EventProperties == nil {
		event.EventProperties = make(map[string]any, len(extraEventProperties))
	}
	for k, v := range extraEventProperties {
		event.EventProperties[k] = v
	}

	// Assign the direct fields which may not have been set from the context or details.
	event.UserID = evalCtx.TargetingKey()
	event.EventType = trackingEventName

	// Map the TrackingEventDetails value to the Amplitude revenue field.
	// The OpenFeature spec indicates that the value parameter in NewTrackingEventDetails
	// represents a monetary value, typically revenue.
	if details.Value() != 0 {
		event.Revenue = details.Value()
	}

	if p.config.EventNormalizer != nil {
		err = p.config.EventNormalizer(ctx, EventNormalizationContext{
			EvaluationContext: evalCtx,
			TrackingKey:       trackingEventName,
			Event:             &event,
			TrackingEventDetails: details,
		})
		if err != nil {
			return event, fmt.Errorf("failed to normalize event: %w", err)
		}
	}

	return event, nil
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

	user, userErr := p.toAmplitudeUser(ctx, evalCtx)
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

	// Create the tracking event details for the exposure event.
	// These fields are based on the documentation at 
	// https://amplitude.com/docs/feature-experiment/under-the-hood/event-tracking#exposure-events
	if p.analyticsClient != nil {
		p.analyticsClient.Track(analytics.Event{
			EventType: "$exposure",
			UserID: user.UserId,
			EventProperties: map[string]any{
				"flag_key": flag,
				"variant": variant.Key,
				"metadata": variant.Metadata,
			},
		})
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
func (p *Provider) toAmplitudeUser(ctx context.Context, evalCtx of.FlattenedContext) (*experiment.User, error) {
	userMap, userProperties := p.normalizeContext( evalCtx)
	userMapJSON, err := json.Marshal(userMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user map: %w", err)
	}

	var user experiment.User
	err = json.Unmarshal(userMapJSON, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user map: %w", err)
	}

	// Ensure that we include the user properties if the context explicitly contained
	// a `user_properties` key, as well as including any attributes from the context
	// which didn't map to a canonical key.
	if user.UserProperties == nil && len(userProperties) > 0 {
		user.UserProperties = make(map[string]any, len(userProperties))
	}
	for k, v := range userProperties {
		user.UserProperties[k] = v
	}

	if p.config.UserNormalizer != nil {
		err = p.config.UserNormalizer(ctx, UserNormalizationContext{
			EvaluationContext: evalCtx,
			User:              &user,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to normalize user: %w", err)
		}
	}

	if user.UserId == "" && user.DeviceId == "" {
		return nil, fmt.Errorf("context must contain a %s, %s, or %s", of.TargetingKey, KeyUserID, KeyDeviceID)
	}

	return &user, nil
}


// normalizeContext normalizes the context map into an Amplitude User or Event.
// It returns a map of the normalized keys and a map of the extra keys.
// The extra keys are the keys that were not found in the key map.
func (p *Provider) normalizeContext(contextMap map[string]any) (normalized map[Key]any, extra map[string]any) {
	normalizedMap := make(map[Key]any, len(contextMap)+1)
	extraMap := make(map[string]any)
	keyMap := p.config.getKeyMap()
	for key, val := range contextMap {
		resolvedKey, ok := keyMap[key]
		if ok {
			normalizedMap[resolvedKey] = val
		} else {
			extraMap[key] = val
		}
	}
	return normalizedMap, extraMap
}
