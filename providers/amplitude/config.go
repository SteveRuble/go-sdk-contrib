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
