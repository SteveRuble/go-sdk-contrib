// Package amplitude provides an OpenFeature provider for Amplitude Experiment.
//
// This package allows you to use Amplitude Experiment (https://amplitude.com/docs/experiment)
// as a feature flag provider through the OpenFeature Go SDK. It wraps the
// Amplitude Experiment Go SDK (https://amplitude.com/docs/sdks/experiment-sdks/experiment-go)
// and supports both local and remote evaluation modes.
//
// # Installation
//
// Install the provider using go get:
//
//	go get github.com/open-feature/go-sdk-contrib/providers/amplitude
//
// # Quick Start
//
// Create a provider and register it with the OpenFeature SDK:
//
//	import (
//	    "context"
//	    "fmt"
//
//	    amplitude "github.com/open-feature/go-sdk-contrib/providers/amplitude"
//	    "github.com/open-feature/go-sdk/openfeature"
//	)
//
//	func main() {
//	    // Create a provider with local evaluation (default)
//	    provider, err := amplitude.New(context.Background(), "your-deployment-key")
//	    if err != nil {
//	        panic(err)
//	    }
//
//	    // Register the provider with OpenFeature (this calls Init internally)
//	    if err := openfeature.SetProviderAndWait(provider); err != nil {
//	        panic(err)
//	    }
//	    defer openfeature.Shutdown()
//
//	    // Create a client
//	    client := openfeature.NewClient("my-app")
//
//	    // Evaluate a flag
//	    evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
//	        "country": "US",
//	    })
//	    enabled, err := client.BooleanValue(context.Background(), "my-feature-flag", false, evalCtx)
//	    if err != nil {
//	        fmt.Printf("Error evaluating flag: %v\n", err)
//	    }
//
//	    if enabled {
//	        // Feature is enabled
//	    }
//	}
//
// # Provider Configuration
//
// The provider is created using [New] or [NewFromConfig]. The [New] function accepts
// a deployment key and optional configuration options:
//
//   - [WithLocalConfig]: Configure local evaluation settings
//   - [WithRemoteConfig]: Configure remote evaluation settings
//   - [WithRemoteEvaluationCache]: Provide a cache for remote evaluation results
//   - [WithKeyMap]: Customize the mapping of evaluation context keys to Amplitude user fields
//   - [WithTrackingEnabled]: Enable event and exposure tracking via Amplitude Analytics
//   - [WithUserNormalizer]: Apply custom transformations to user context before evaluation
//   - [WithEventNormalizer]: Apply custom transformations to events before tracking
//
// # Local vs Remote Evaluation
//
// The Amplitude Go SDK supports two evaluation modes. See the Amplitude documentation
// for detailed information on choosing between them.
//
// Local Evaluation (default): The provider downloads all flag rules from the server
// and evaluates them locally. This is faster but requires more memory if you have
// large cohorts. Use [WithLocalConfig] to configure local evaluation settings.
// See https://amplitude.com/docs/feature-experiment/local-evaluation for details.
//
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithLocalConfig(local.Config{
//	        Debug: true,
//	    }),
//	)
//
// Remote Evaluation: The provider makes a round-trip to Amplitude servers for each
// evaluation. This is needed for ID resolution, user enrichment, or sticky bucketing
// (as distinct from consistent bucketing, which works with both modes).
// Use [WithRemoteConfig] to enable remote evaluation.
// See https://amplitude.com/docs/sdks/experiment-sdks/experiment-go#remote-evaluation for details.
//
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithRemoteConfig(remote.Config{}),
//	)
//
// # Caching for Remote Evaluation
//
// When using remote evaluation, Amplitude returns all flag results for a user in a
// single request. You can cache these results to avoid redundant server calls:
//
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithRemoteConfig(remote.Config{}),
//	    amplitude.WithRemoteEvaluationCache(myCache),
//	)
//
// The cache must implement the [Cache] interface.
//
// # Evaluation Context Mapping
//
// The provider maps OpenFeature evaluation context keys to Amplitude user fields.
// The [openfeature.TargetingKey] is automatically mapped to the Amplitude user_id.
//
// Standard Amplitude user fields are recognized with various naming conventions.
// For example, "device_id", "deviceId", "device-id", and "DeviceID" all map to
// the Amplitude device_id field. See [DefaultKeyMap] for the complete list.
//
// Keys that don't match any known Amplitude field are added to the user's
// UserProperties map.
//
// You can customize the key mapping using [WithKeyMap]:
//
//	customKeyMap := amplitude.DefaultKeyMap()
//	customKeyMap["my_user_id"] = amplitude.KeyUserID
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithKeyMap(customKeyMap),
//	)
//
// # Payload Typing
//
// In Amplitude, each variant can have a JSON payload. This provider interprets
// the payload based on the evaluation method called:
//
//   - [Provider.BooleanEvaluation]: Expects a JSON boolean (true/false)
//   - [Provider.StringEvaluation]: Expects a JSON string ("foo")
//   - [Provider.IntEvaluation]: Expects a JSON number (42) or string ("42")
//   - [Provider.FloatEvaluation]: Expects a JSON number (3.14)
//   - [Provider.ObjectEvaluation]: Expects a JSON object ({"key": "value"})
//
// If the payload cannot be unmarshalled to the requested type, the provider
// returns an error and the default value.
//
// # Special Cases
//
// The default variant (returned when rollout is 0%) is interpreted as the
// zero value for the requested type: false for bool, 0 for int/float,
// empty string for string, and nil for object.
//
// If a variant has no payload and is not the default variant:
//   - For boolean evaluation, it returns true
//   - For other types, it returns an error
//
// # Amplitude User Fields
//
// The following Amplitude user fields can be set via the evaluation context:
//
//   - [KeyUserID]: Primary user identifier (mapped from TargetingKey)
//   - [KeyDeviceID]: Device identifier
//   - [KeyCountry]: User's country
//   - [KeyRegion]: User's region/state
//   - [KeyCity]: User's city
//   - [KeyDma]: Designated Market Area
//   - [KeyLanguage]: User's language preference
//   - [KeyPlatform]: Platform (iOS, Android, Web, etc.)
//   - [KeyVersion]: Application version
//   - [KeyOs]: Operating system
//   - [KeyDeviceManufacturer]: Device manufacturer
//   - [KeyDeviceBrand]: Device brand
//   - [KeyDeviceModel]: Device model
//   - [KeyCarrier]: Mobile carrier
//   - [KeyLibrary]: SDK library identifier
//   - [KeyUserProperties]: Custom user properties (map[string]any)
//   - [KeyGroups]: Group memberships (map[string][]string)
//   - [KeyGroupProperties]: Group properties (map[string]map[string]any)
//   - [KeyCohortIDs]: Cohort IDs for targeting (map[string]struct{})
//   - [KeyGroupCohortIDSet]: Group cohort IDs (map[string]map[string]map[string]struct{})
//
// # Event Tracking
//
// The provider implements the [openfeature.Tracker] interface, allowing you to send
// tracking events to Amplitude Analytics. This is essential for experimentation powered
// by feature flags. To enable tracking, configure the provider with an analytics config:
//
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithTrackingEnabled(analytics.Config{
//	        APIKey: "your-amplitude-api-key", // This is your project key, not the deployment key
//	    }),
//	)
//	openfeature.SetProviderAndWait(provider)
//	client := openfeature.NewDefaultClient()
//
// When tracking is enabled:
//   - Exposure events are automatically sent when flags are evaluated
//   - You can send custom tracking events via the client's Track method
//   - Assignment events are tracked for local evaluation
//
// See https://amplitude.com/docs/feature-experiment/under-the-hood/event-tracking for details.
//
// # Tracking Event Details and Revenue
//
// Use the client's Track method to send tracking events. The value passed to
// [openfeature.NewTrackingEventDetails] is mapped to the Amplitude event's Revenue field:
//
//	// Track a purchase event with revenue
//	evalCtx := openfeature.NewEvaluationContext("user-123", nil)
//	details := openfeature.NewTrackingEventDetails(99.99).
//	    Add("currency", "USD").
//	    Add("product_id", "SKU-12345")
//	client.Track(ctx, "purchase-completed", evalCtx, details)
//
// The value parameter (99.99 in this example) is interpreted as revenue and will be
// set on the Amplitude event's Revenue field. If the value is 0, the Revenue field
// is not set. If you want the value to end up in a different field, provide a custom
// event normalizer using [WithEventNormalizer] and move the value there.
//
// Additional attributes added via Add() are mapped using the same key mapping logic
// as the evaluation context. Unmapped keys are placed in the event's EventProperties.
//
// # User Normalizer
//
// For advanced user context transformation beyond key mapping, use [WithUserNormalizer].
// The normalizer function is called after key mapping has been applied, allowing you
// to modify the Amplitude User before evaluation:
//
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithUserNormalizer(func(ctx context.Context, normCtx amplitude.UserNormalizationContext) error {
//	        // Add computed properties
//	        if normCtx.User.UserProperties == nil {
//	            normCtx.User.UserProperties = make(map[string]any)
//	        }
//	        normCtx.User.UserProperties["computed_tier"] = computeTier(normCtx.EvaluationContext)
//	        return nil
//	    }),
//	)
//
// The [UserNormalizationContext] provides access to both the original evaluation context
// and the partially-built Amplitude User. Return an error to abort the evaluation.
//
// # Event Normalizer
//
// For advanced event transformation, use [WithEventNormalizer]. The normalizer function
// is called after key mapping has been applied to tracking events:
//
//	provider, err := amplitude.New(ctx, "deployment-key",
//	    amplitude.WithTrackingEnabled(analytics.Config{APIKey: "..."}),
//	    amplitude.WithEventNormalizer(func(ctx context.Context, normCtx amplitude.EventNormalizationContext) error {
//	        // Add user or group property mutations to the event
//	        normCtx.Event.UserProperties = map[analytics.IdentityOp]map[string]any{
//	            analytics.IdentityOpSet: {"last_action": normCtx.TrackingKey},
//	        }
//	        return nil
//	    }),
//	)
//
// The [EventNormalizationContext] provides access to the evaluation context, tracking key,
// tracking event details, and the partially-built Amplitude Event. Return an error to
// abort tracking for that event.
package amplitude
