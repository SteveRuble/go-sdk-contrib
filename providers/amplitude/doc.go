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
// Create a provider and use it to evaluate feature flags:
//
//	import (
//	    "context"
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
//	    // Initialize the provider
//	    if err := provider.Init(openfeature.EvaluationContext{}); err != nil {
//	        panic(err)
//	    }
//	    defer provider.Shutdown()
//
//	    // Evaluate a flag
//	    evalCtx := openfeature.FlattenedContext{
//	        openfeature.TargetingKey: "user-123",
//	    }
//	    result := provider.BooleanEvaluation(context.Background(), "my-feature-flag", false, evalCtx)
//
//	    if result.Value {
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
package amplitude
