# Unofficial Amplitude OpenFeature GO Provider

[Amplitude](https://amplitude.com/) OpenFeature Provider can provide usage for Amplitude via the OpenFeature Go SDK.

# Installation

To use the provider, you'll need to install the Amplitude provider. You can install the packages using the following command

```shell
go get github.com/open-feature/go-sdk-contrib/providers/amplitude
```

## Concepts

### Payloads and Typing

In Amplitude, a flag variant may be given a JSON payload. 
This package interprets the payload by unmarshalling it to the requested type. 
This means that you should configure the payload as the raw JSON representation of the value you want.
Example payloads:

- `bool`
  ```json
  true
  ```
- `string`
  ```json
  "foo"
  ```
- `int` (supports formatting the number as a string)
  ```json
  42
  ```
  or 
  ```json
  "9007199254740999"
  ```
- `float`
  ```json
  42.31
  ```
- `map[string]any` or `[]any`
  ```json
  {
    "foo": "bar"
  }
  ```

If the payload cannot be unmarshalled to the requested type, the provider returns an error, except in the special cases below.

#### Special Cases

* The default "off" variant (the variant you always get when rollout is at 0%) 
  is interpreted as the zero value of the requested type (`false`, `0`, `0.0`, `""`, or `nil`)
* If a variant has no payload and is not the default variant:
  * If a `bool` is requested it is interpreted as `true`.
  * Otherwise the provider returns an error.

#### Default Values

The default value passed to the `Evaluate*` method of the provider will only be returned
if the flag is not defined or not available.

### Local vs Remote Evaluation

The [Amplitude Go SDK](https://amplitude.com/docs/sdks/experiment-sdks/experiment-go) 
supports [local](https://amplitude.com/docs/feature-experiment/local-evaluation)
and [remote](https://amplitude.com/docs/sdks/experiment-sdks/experiment-go#remote-evaluation) 
evaluation.

You can configure this provider to use Local or Remote evaluation
using the options `WithLocalEvaluation` or `WithRemoteEvaluation`.
The default is local evaluation, because it has fewer configuration settings
and is more performant in time (but not space).

#### Remote Evaluation

Remote evaluation supports more capabilities,
at the expense of a round-trip to the Amplitude servers on each evaluation.
It's needed if you are using ID resolution, user enrichment, or sticky bucketing
(as distinct from consistent bucketing, which works with local and remote evaluation).
See the documentation for details.

The Amplitude system is a little unusual in that the default behavior
is to evaluate all available flags against the given user 
and return all the results. 
This allows you to speed things up by caching the results for a given user
and skipping subsequent calls to the server. 
The `WithRemoteEvaluationCache` can be used to provide a `Cache` implementation
for this purpose. 
The `Cache` will be queried using a key based on the serialized context
before sending a request to the server.
This can speed up evaluation at the expense of using more memory.
A useful pattern can be to put a request-scoped cache in the `context.Context`
in a middleware upstream of where flags are evaluated, 
then provide this package with a cache which stores the 
flag variant bundle in the context. 
This means you'll only evaluate flags once per request.

#### Local Evaluation

Local evaluation is faster, but requires assigning any cohort information on the client side
as values in the context.
The provider will download all the flag rules from the server and evaluate them on demand.
If you have very large cohorts, this may use a noticable amount of memory.

## Usage
The Amplitude OpenFeature Provider uses the Amplitude GO SDK and integrates with the 
[OpenFeature Go SDK](https://openfeature.dev/docs/reference/sdks/server/go).

### Usage Example

```go
import (
    "context"
    "fmt"
    
    amplitude "github.com/open-feature/go-sdk-contrib/providers/amplitude"
    "github.com/open-feature/go-sdk/openfeature"
)

func main() {
    // Create a provider with local evaluation (default)
    provider, err := amplitude.New(context.Background(), "your-deployment-key")
    if err != nil {
        panic(err)
    }
    
    // Or use remote evaluation
    // provider, err := amplitude.New(context.Background(), "your-deployment-key", 
    //     amplitude.WithRemoteConfig(remote.Config{}))
    
    // Register the provider with OpenFeature (this calls Init internally)
    if err := openfeature.SetProviderAndWait(provider); err != nil {
        panic(err)
    }
    defer openfeature.Shutdown()
    
    // Create a client
    client := openfeature.NewClient("my-app")
    
    // Evaluate a flag
    evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
        "country": "US",
    })
    enabled, err := client.BooleanValue(context.Background(), "my-feature-flag", false, evalCtx)
    if err != nil {
        fmt.Printf("Error evaluating flag: %v\n", err)
    }
    
    if enabled {
        // Feature is enabled
    }
}
```

See [provider_test.go](./provider_test.go) for more examples.

### Event Tracking

This provider implements the OpenFeature [`Tracker` interface](https://openfeature.dev/docs/reference/sdks/server/go#tracking), 
allowing you to send tracking events to Amplitude Analytics. This is essential for robust 
experimentation powered by feature flags. Enable tracking by configuring the provider with 
an analytics config:

```go
import (
    "context"
    
    analytics "github.com/amplitude/analytics-go/amplitude"
    amplitude "github.com/open-feature/go-sdk-contrib/providers/amplitude"
    "github.com/open-feature/go-sdk/openfeature"
)

func main() {
    provider, err := amplitude.New(ctx, "deployment-key",
        amplitude.WithTrackingEnabled(analytics.Config{
            APIKey: "your-amplitude-api-key",
        }),
    )
    if err != nil {
        panic(err)
    }
    
    openfeature.SetProviderAndWait(provider)
    client := openfeature.NewClient("my-app")
    
    // Trigger a tracking event
    evalCtx := openfeature.NewEvaluationContext("user-123", nil)
    details := openfeature.NewTrackingEventDetails(99.77).Add("currencyCode", "USD")
    client.Track(ctx, "visited-promo-page", evalCtx, details)
}
```

When tracking is enabled:
- **Exposure events** are automatically sent when flags are evaluated
- **Custom tracking events** can be sent via the client's `Track` method
- **Assignment events** are tracked for local evaluation (if configured in the local config)

See the [Amplitude Event Tracking documentation](https://amplitude.com/docs/feature-experiment/under-the-hood/event-tracking) for details.

#### Revenue Tracking

The value passed to `openfeature.NewTrackingEventDetails(value)` is mapped to Amplitude's Revenue field:

```go
// Track a purchase event with revenue
evalCtx := openfeature.NewEvaluationContext("user-123", nil)
details := openfeature.NewTrackingEventDetails(99.99).
    Add("currency", "USD").
    Add("product_id", "SKU-12345")
client.Track(ctx, "purchase-completed", evalCtx, details)
```

The `99.99` value will be set as the Revenue on the Amplitude event. If the value is 0, 
the Revenue field is not set.

### Advanced Normalization

For advanced transformations beyond key mapping, the provider supports normalizer functions.

#### User Normalizer

Use `WithUserNormalizer` to modify the Amplitude User before evaluation:

```go
provider, err := amplitude.New(ctx, "deployment-key",
    amplitude.WithUserNormalizer(func(ctx context.Context, normCtx amplitude.UserNormalizationContext) error {
        // Add computed properties based on original context
        if normCtx.User.UserProperties == nil {
            normCtx.User.UserProperties = make(map[string]any)
        }
        normCtx.User.UserProperties["computed_tier"] = computeTier(normCtx.EvaluationContext)
        return nil
    }),
)
```

#### Event Normalizer

Use `WithEventNormalizer` to modify tracking events before they're sent:

```go
provider, err := amplitude.New(ctx, "deployment-key",
    amplitude.WithTrackingEnabled(analytics.Config{APIKey: "..."}),
    amplitude.WithEventNormalizer(func(ctx context.Context, normCtx amplitude.EventNormalizationContext) error {
        // Add user property mutations to the event
        normCtx.Event.UserProperties = map[analytics.IdentityOp]map[string]any{
            analytics.IdentityOpSet: {"last_action": normCtx.TrackingKey},
        }
        return nil
    }),
)
```

### Logging

This package performs very little logging, but where it does log it tries to delegate to the logger
configured on the Amplitude SDK configs (`local.Config` or `remote.Config`),
falling back to the standard library `log` package if those are not configured.

## Development

### Running Tests

Unit tests can be run without any external dependencies:

```shell
go test ./...
```

### Integration Tests

Integration tests use [go-vcr](https://github.com/dnaeon/go-vcr) to record and replay HTTP interactions. 
This allows tests to run without network access by replaying previously recorded cassettes.

#### Running in Replay Mode

By default, integration tests run in replay mode using existing cassettes:

```shell
go test ./...
```

#### Recording New Cassettes

To record new VCR cassettes (e.g., when updating the test flag configuration or adding new tests), 
you need to set the following environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `AMPLITUDE_DEPLOYMENT_KEY` | Your Amplitude Experiment deployment key (server-side). This is required for recording. Find this in the Amplitude console under Experiment > Deployments. |
| `AMPLITUDE_MANAGEMENT_API_KEY` | Your Amplitude Management API key. This is required to automatically create/update the test feature flag. See [Management API Authentication](https://amplitude.com/docs/apis/experiment/experiment-management-api#authentication). |
| `AMPLITUDE_PROJECT_ID` | Your Amplitude project ID. Required if creating a new flag. Find this in the Amplitude console URL. |
| `AMPLITUDE_DEPLOYMENT_ID` | Your Amplitude deployment ID. Required if creating a new flag to assign it to a deployment. Find this by `curl`ing an existing flag you've set up with a deployment. |
| `AMPLITUDE_PROJECT_KEY` | Your Amplitude project API key (for analytics events). This is required to test event tracking. Find this in the Amplitude console under Organization Settings > Projects > {project} > General > API Key. |

Example:

```shell
export AMPLITUDE_DEPLOYMENT_KEY="server-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export AMPLITUDE_MANAGEMENT_API_KEY="your-management-api-key"
export AMPLITUDE_PROJECT_ID="123456"
export AMPLITUDE_DEPLOYMENT_ID="17994"
export AMPLITUDE_PROJECT_KEY="your-project-api-key"

go test ./...
```

When these environment variables are set, the test suite will:

1. Use the Management API to ensure the test feature flag exists with the correct configuration
2. Record all HTTP interactions to VCR cassettes in the `testdata/` directory
3. The cassettes can then be committed to version control for replay mode

The test flag configuration is defined in `testdata/test-flag.json`. If you need to modify the flag's 
variants or targeting rules, update this file and re-record the cassettes.

### Test Flag Configuration

The test feature flag (`test-feature-flag`) is configured with the following variants:

| Variant | Payload | Target Segment |
|---------|---------|----------------|
| `enabled` | `true` (boolean) | Users with `user_id = "expect-enabled"` |
| `int` | `42` (number) | Users with `user_id = "expect-int"` |
| `payload` | `12.34` (number) | Users with `user_id = "expect-float"` |
| `string` | `"foo"` (string) | Users with `user_id = "expect-string"` |
| `object` | `{"a": "A", "b": "B"}` (object) | Users with `user_id = "expect-object"` |

See `testdata/test-flag.json` for the complete configuration.
