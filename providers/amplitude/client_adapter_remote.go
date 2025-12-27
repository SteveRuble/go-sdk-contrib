package amplitude

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/amplitude/experiment-go-server/pkg/experiment/remote"
)

// RemoteClient wraps the Amplitude remote evaluation client to implement ExperimentClient.
type clientAdapterRemote struct {
	client *remote.Client
	cache  Cache
}

// RemoteConfig contains configuration for remote evaluation.
type remoteConfig struct {
	remote.Config
	Cache Cache
}

// NewRemoteClient creates a new RemoteClient with the given deployment key, config, and logger.
func newClientAdapterRemote(deploymentKey string, config remoteConfig) *clientAdapterRemote {
	return &clientAdapterRemote{
		cache:  config.Cache,
		client: remote.Initialize(deploymentKey, &config.Config),
	}
}

// Start starts the remote evaluation client.
func (c *clientAdapterRemote) Start() error {
	return nil
}

// Stop stops the remote evaluation client.
func (c *clientAdapterRemote) Stop() error {
	return nil
}

// Evaluate evaluates the given flags for the given user using remote evaluation.
// Note: Remote evaluation fetches all variants for the user; flagKeys is ignored.
func (c *clientAdapterRemote) Evaluate(ctx context.Context, user *experiment.User, _ []string) (map[string]experiment.Variant, error) {
	// Check if the cache has the variants for the given context
	var cacheKey string
	if c.cache != nil {
		hasher := sha256.New()
		err := json.NewEncoder(hasher).Encode(user)
		if err != nil {
			return nil, fmt.Errorf("failed to encode user to create cache key: %w", err)
		} 
		cacheKey = string(hasher.Sum(nil))
		cacheValue, err := c.cache.Get(ctx, cacheKey)
		if err == nil && cacheValue != nil {
			return cacheValue.(map[string]experiment.Variant), nil
		}
	}
	variants, fetchErr := c.client.FetchV2(user)
	if fetchErr != nil {
		return nil, fetchErr
	}

	// Store the variants in the cache
	if c.cache != nil {
		err := c.cache.Set(ctx, cacheKey, variants)
		if err != nil {
			return nil, fmt.Errorf("failed to store variants in cache: %w", err)
		}
	}

	return variants, nil
}
