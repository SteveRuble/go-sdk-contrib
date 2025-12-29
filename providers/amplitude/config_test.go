package amplitude

import (
	"context"
	"testing"

	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
	"github.com/amplitude/experiment-go-server/pkg/experiment/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithLocalConfig(t *testing.T) {
	localCfg := local.Config{
		Debug: true,
	}

	option := WithLocalConfig(localCfg)

	cfg := &Config{}
	option(cfg)

	require.NotNil(t, cfg.LocalConfig)
	assert.True(t, cfg.LocalConfig.Debug)
}

func TestWithRemoteConfig(t *testing.T) {
	remoteCfg := remote.Config{
		Debug: true,
	}

	option := WithRemoteConfig(remoteCfg)

	cfg := &Config{}
	option(cfg)

	require.NotNil(t, cfg.RemoteConfig)
	assert.True(t, cfg.RemoteConfig.Debug)
}

func TestWithRemoteEvaluationCache(t *testing.T) {
	cache := &mockCache{}

	option := WithRemoteEvaluationCache(cache)

	cfg := &Config{}
	option(cfg)

	assert.Equal(t, cache, cfg.RemoteEvaluationCache)
}

// mockCache is a simple mock implementation of the Cache interface
type mockCache struct {
	data map[string]any
}

func (m *mockCache) Set(_ context.Context, key string, value any) error {
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Get(_ context.Context, key string) (any, error) {
	if m.data == nil {
		return nil, nil
	}
	return m.data[key], nil
}

func TestConfig_getLocalConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		expectDebug bool
	}{
		{
			name:        "nil LocalConfig returns empty config",
			cfg:         Config{},
			expectDebug: false,
		},
		{
			name: "returns configured LocalConfig",
			cfg: Config{
				LocalConfig: &local.Config{
					Debug: true,
				},
			},
			expectDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.getLocalConfig()
			assert.Equal(t, tt.expectDebug, result.Debug)
		})
	}
}

func TestConfig_getRemoteConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		expectDebug bool
	}{
		{
			name:        "nil RemoteConfig returns empty config",
			cfg:         Config{},
			expectDebug: false,
		},
		{
			name: "returns configured RemoteConfig",
			cfg: Config{
				RemoteConfig: &remote.Config{
					Debug: true,
				},
			},
			expectDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.getRemoteConfig()
			assert.Equal(t, tt.expectDebug, result.Debug)
		})
	}
}

func TestConfig_getRemoteConfig_IncludesCache(t *testing.T) {
	cache := &mockCache{}
	cfg := Config{
		RemoteConfig:          &remote.Config{Debug: true},
		RemoteEvaluationCache: cache,
	}

	result := cfg.getRemoteConfig()

	assert.Equal(t, cache, result.Cache, "getRemoteConfig should include RemoteEvaluationCache")
	assert.True(t, result.Debug)
}

func TestNew_AppliesOptions(t *testing.T) {
	mock := &mockClientAdapter{}

	localCfg := local.Config{Debug: true}

	provider, err := New(
		context.Background(),
		"test-key",
		WithLocalConfig(localCfg),
		withMockClient(mock),
	)

	require.NoError(t, err)
	require.NotNil(t, provider)
	require.NotNil(t, provider.config.LocalConfig)
	assert.True(t, provider.config.LocalConfig.Debug)
}

func TestNew_MultipleOptions(t *testing.T) {
	mock := &mockClientAdapter{}

	cache := &mockCache{}

	provider, err := New(
		context.Background(),
		"test-key",
		WithRemoteConfig(remote.Config{Debug: true}),
		WithRemoteEvaluationCache(cache),
		withMockClient(mock),
	)

	require.NoError(t, err)
	require.NotNil(t, provider)
	require.NotNil(t, provider.config.RemoteConfig)
	assert.True(t, provider.config.RemoteConfig.Debug)
	assert.Equal(t, cache, provider.config.RemoteEvaluationCache)
}

func TestNewFromConfig_BothConfigsErrors(t *testing.T) {
	cfg := Config{
		DeploymentKey: "test-key",
		LocalConfig:   &local.Config{},
		RemoteConfig:  &remote.Config{},
	}

	_, err := NewFromConfig(context.Background(), cfg)
	require.Error(t, err)
	assert.Equal(t, "you cannot configure the provider to use both local and remote evaluation at the same time", err.Error())

}

func TestNewFromConfig_UsesLocalByDefault(t *testing.T) {
	cfg := Config{
		DeploymentKey: "test-key",
		// Neither LocalConfig nor RemoteConfig set
	}

	provider, err := NewFromConfig(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)
}


