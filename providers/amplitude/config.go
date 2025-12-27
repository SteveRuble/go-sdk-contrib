package amplitude

import (
	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
	"github.com/amplitude/experiment-go-server/pkg/experiment/remote"
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

// WithKeyMap sets the key map for the Amplitude provider.
// If unset, [DefaultKeyMap] will be used.
func WithKeyMap(keyMap map[string]Key) Option {
	return func(c *Config) {
		c.KeyMap = keyMap
	}
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
	return remoteConfig{Config: *c.RemoteConfig}
}