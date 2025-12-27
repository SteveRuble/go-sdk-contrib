# Unofficial Amplitude OpenFeature GO Provider

[Amplitude](https://amplitude.com/) OpenFeature Provider can provide usage for Amplitude via OpenFeature GO SDK.

# Installation

To use the provider, you'll need to install the Amplitude provider. You can install the packages using the following command

```shell
go get github.com/open-feature/go-sdk-contrib/providers/amplitude
```

## Concepts

### Payloads and Typing

In Amplitude, a variant may be given a JSON payload. This package interprets the payload by unmarshalling it to the requested type. 
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
- `map[string]any`
  ```json
  {
    "foo": "bar"
  }
  ```

If the payload cannot be unmarshalled to the requested type, the provider returns an error, except in the special cases below.

#### Special Cases

* The default variant (the variant you always get when rollout is at 0%) is interpreted as the default value of the requested type (`false`, `0`, `0.0`, `""`, or `nil`)
* If a variant has no payload and is not the default variant:
  * If a `bool` is requested it is interpreted as `true`.
  * Otherwise the provider returns an error.

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
The `WithRemoteEvaluation` option supports providing a `Cacher`
for this purpose. 
The `Cacher` will be queried using a key based on the serialized context
before sending a request to the server.

#### Local Evaluation

Local evaluation is faster, but requires assigning any cohort information on the client side
as values in the context.
The provider will download all the flag rules from the server and evaluate them on demand.
If you have very large cohorts, this may use a noticable amount of memory.

## Usage
The Amplitude OpenFeature Provider uses the Amplitude GO SDK.

### Usage Example

TODO: fill in example.

```go

```
See [provider_test.go](./pkg/provider_test.go) for more information.

