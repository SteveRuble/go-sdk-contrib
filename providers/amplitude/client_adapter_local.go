package amplitude

import (
	"context"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
)

// LocalClient wraps the Amplitude local evaluation client to implement ExperimentClient.
type clientAdapterLocal struct {
	client *local.Client
}

// localConfig contains configuration for local evaluation.
type localConfig struct {
	local.Config
}

// newClientAdapterLocal creates a new LocalClient with the given deployment key, config, and logger.
// The client must be started by calling Start() before use.
func newClientAdapterLocal(deploymentKey string, config localConfig) *clientAdapterLocal {
	return &clientAdapterLocal{
		client: local.Initialize(deploymentKey, &config.Config),
	}
}

// Start starts the local evaluation client, fetching flag configurations.
func (c *clientAdapterLocal) Start() error {
	return c.client.Start()
}

// Stop stops the local evaluation client.
func (c *clientAdapterLocal) Stop() error {
	return nil
}

// Evaluate evaluates the given flags for the given user using local evaluation.
func (c *clientAdapterLocal) Evaluate(_ context.Context, user *experiment.User, flagKeys []string) (map[string]experiment.Variant, error) {
	return c.client.EvaluateV2(user, flagKeys)
}
