package amplitude

import (
	"context"

	analytics "github.com/amplitude/analytics-go/amplitude"
	"github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
	"github.com/amplitude/experiment-go-server/pkg/experiment/remote"
	of "github.com/open-feature/go-sdk/openfeature"
)

// Config contains the configuration for the Amplitude provider.
// Either LocalConfig or RemoteConfig should be set, but not both.
// If neither is set, local evaluation with default settings is used.
type Config struct {
	// DeploymentKey is the server deployment key from the Amplitude console.
	DeploymentKey string
	// LocalConfig is optional configuration for local evaluation.
	// Local evaluation is the default behavior.
	LocalConfig *local.Config
	// RemoteConfig is optional configuration for remote evaluation.
	// If set, remote evaluation will be used.
	RemoteConfig *remote.Config
	// cache is an optional cache for remote evaluation.
	// If set, the cache will be used to store the results of the evaluations.
	RemoteEvaluationCache Cache
	// KeyMap is a map of string keys that might be in the evaluation context
	// to the canonical key used by Amplitude.
	// You can add keys to this map to automatically map the keys in the evaluation context
	// to the canonical keys used by Amplitude.
	// If multiple keys found in the evaluation context 
	// map to the same canonical key, no error will be raised,
	// one will simply override the other.
	// Any keys that are not mapped will be added to the User.UserProperties map.
	// For more advanced normalization, use a hook to pre-process the evaluation context.
	// If unset, [DefaultKeyMap] will be used.
	KeyMap map[string]Key

	// UserNormalizer is an optional function that normalizes the evaluation context into an Amplitude User.
	// If set, it will be used to normalize the evaluation context into an Amplitude User,
	// after key mapping has been applied. 
	// In other words, you only need this if you're doing something
	// beyond mapping keys from the evaluation context to canonical keys
	// on the [experiment.User] type.
	UserNormalizer func(ctx context.Context, context UserNormalizationContext) error

	// EventNormalizer is an optional function that normalizes the evaluation context into an Amplitude Event.
	// If set, it will be used to normalize the evaluation context into an Amplitude Event,
	// after key mapping has been applied. 
	// In other words, you only need this if you're doing something
	// beyond mapping keys from the evaluation context to canonical keys
	// on the [analytics.Event] type.
	// You may want to do this if you want to have the event update
	// user or group properties.
	EventNormalizer func(ctx context.Context, normContext EventNormalizationContext) error

	// AnalyticsConfig is an optional Amplitude analytics config.
	// If set, it will be used to track events when the provider is used as a tracker.
	// It will also automatically record exposure events for flags.
	AnalyticsConfig *analytics.Config

	// testClientAdapter is an optional clientAdapter for testing.
	// When set, NewFromConfig will use this instead of creating a real client.
	// This field is not part of the public API.
	testClientAdapter clientAdapter
}

// Option is a function that configures the Config.
type Option func(*Config)

// WithLocalConfig sets the local configuration.
func WithLocalConfig(localConfig local.Config) Option {
	return func(c *Config) {
		c.LocalConfig = &localConfig
	}
}

// WithRemoteConfig sets the remote configuration.
func WithRemoteConfig(remoteConfig remote.Config) Option {
	return func(c *Config) {
		c.RemoteConfig = &remoteConfig
	}
}

// WithRemoteEvaluationCache sets the cache for remote evaluation.
// This will be used to cache the variants available for a given context,
// so subsequent evaluations for the same context don't need to 
// re-fetch the variants from the server.
func WithRemoteEvaluationCache(cache Cache) Option {
	return func(c *Config) {
		c.RemoteEvaluationCache = cache
	}
}

// WithTrackingEnabled configures the Amplitude provider to track assignment and exposure events.
// See documentation at https://amplitude.com/docs/feature-experiment/under-the-hood/event-tracking.
// This option is automatically enabled if you're using local evaluation
// and you populated [local.Config.AssignmentConfig] (and vice versa).
// Note: assignment is automatically tracked for remote evaluation.
func WithTrackingEnabled(config analytics.Config) Option {
	return func(c *Config) {
		c.AnalyticsConfig = &config
	}
}

// WithKeyMap sets the key map for the Amplitude provider.
// If unset, [DefaultKeyMap] will be used.
func WithKeyMap(keyMap map[string]Key) Option {
	return func(c *Config) {
		c.KeyMap = keyMap
	}
}

// WithUserNormalizer sets the user normalizer for the Amplitude provider.
// If set, it will be used to normalize the evaluation context into an Amplitude User,
// after key mapping has been applied. 
// In other words, you only need this if you're doing something
// beyond mapping keys from the evaluation context to canonical keys
// on the [experiment.User] type.
// You may want to do this if you want to have the user update
// user or group properties.
func WithUserNormalizer(userNormalizer func(ctx context.Context, context UserNormalizationContext) error) Option {
	return func(c *Config) {
		c.UserNormalizer = userNormalizer
	}
}

// UserNormalizationContext is the context for the user normalizer.
type UserNormalizationContext struct {
	// EvaluationContext is the evaluation context for the user normalizer.
	EvaluationContext of.FlattenedContext
	// User is the user for the user normalizer.
	// It will already have been populated with any 
	// keys from the evaluation context that have been mapped to canonical keys
	// on the [experiment.User] type.
	User *experiment.User
}

// WithEventNormalizer sets the event normalizer for the Amplitude provider.
// If set, it will be used to normalize the evaluation context into an Amplitude Event,
// after key mapping has been applied. 
// In other words, you only need this if you're doing something
// beyond mapping keys from the evaluation context to canonical keys
// on the [analytics.Event] type.
// You may want to do this if you want to have the event update
// user or group properties.
func WithEventNormalizer(eventNormalizer func(ctx context.Context, normContext EventNormalizationContext) error) Option {
	return func(c *Config) {
		c.EventNormalizer = eventNormalizer
	}
}

// EventNormalizationContext is the context for the event normalizer.
type EventNormalizationContext struct {
	// EvaluationContext is the evaluation context for the event normalizer.
	EvaluationContext of.EvaluationContext
	// TrackingKey is the tracking key (probably the event name).
	TrackingKey string
	// TrackingEventDetails is the tracking event details for the event normalizer.
	TrackingEventDetails of.TrackingEventDetails
	// Event is the event for the event normalizer.
	// It will already have been populated with any 
	// keys from the evaluation context and tracking event details 
	// that have been mapped to canonical keys
	// on the [analytics.Event] type.
	Event *analytics.Event
}

// getKeyMap returns the key map for the Amplitude provider.
// If unset, [DefaultKeyMap] will be used.
func (c *Config) getKeyMap() map[string]Key {
	if c.KeyMap == nil {
		c.KeyMap = DefaultKeyMap()
	}
	return c.KeyMap
}

// getLocalConfig returns the local configuration for the Amplitude provider.
func (c *Config) getLocalConfig() localConfig {
	if c.LocalConfig == nil {
		c.LocalConfig = &local.Config{}
	}
	return localConfig{Config: *c.LocalConfig}
}

// getRemoteConfig returns the remote configuration for the Amplitude provider.
func (c *Config) getRemoteConfig() remoteConfig {
	if c.RemoteConfig == nil {
		c.RemoteConfig = &remote.Config{}
	}
	return remoteConfig{
		Config: *c.RemoteConfig,
		Cache:  c.RemoteEvaluationCache,
	}
}