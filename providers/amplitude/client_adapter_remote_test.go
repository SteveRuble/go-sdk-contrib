package amplitude

import (
	"context"
	"errors"
	"testing"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCacheWithError is a mock cache that can return errors.
type mockCacheWithError struct {
	data     map[string]any
	getErr   error
	setErr   error
	getCalls []string
	setCalls []setCacheCall
}

type setCacheCall struct {
	key   string
	value any
}

func (m *mockCacheWithError) Get(_ context.Context, key string) (any, error) {
	m.getCalls = append(m.getCalls, key)
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.data == nil {
		return nil, nil
	}
	return m.data[key], nil
}

func (m *mockCacheWithError) Set(_ context.Context, key string, value any) error {
	m.setCalls = append(m.setCalls, setCacheCall{key: key, value: value})
	if m.setErr != nil {
		return m.setErr
	}
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
	return nil
}

// mockRemoteEvaluator is a mock implementation of remoteEvaluator for testing.
type mockRemoteEvaluator struct {
	fetchFunc  func(user *experiment.User) (map[string]experiment.Variant, error)
	fetchCalls []*experiment.User
}

func (m *mockRemoteEvaluator) FetchV2(user *experiment.User) (map[string]experiment.Variant, error) {
	m.fetchCalls = append(m.fetchCalls, user)
	if m.fetchFunc != nil {
		return m.fetchFunc(user)
	}
	return map[string]experiment.Variant{}, nil
}

func TestRemoteConfig_CacheField(t *testing.T) {
	cache := &mockCacheWithError{}
	cfg := remoteConfig{
		Cache: cache,
	}

	assert.Equal(t, cache, cfg.Cache)
}

func TestClientAdapterRemote_Start(t *testing.T) {
	// The Start method is a no-op for remote client
	client := &clientAdapterRemote{}
	err := client.Start()
	assert.NoError(t, err)
}

func TestClientAdapterRemote_Stop(t *testing.T) {
	// The Stop method is a no-op for remote client
	client := &clientAdapterRemote{}
	err := client.Stop()
	assert.NoError(t, err)
}

func TestClientAdapterLocal_Stop(t *testing.T) {
	// The Stop method is a no-op for local client
	client := &clientAdapterLocal{}
	err := client.Stop()
	assert.NoError(t, err)
}

// Test cache interface implementation
func TestMockCache_ImplementsCache(t *testing.T) {
	var _ Cache = (*mockCacheWithError)(nil)
}

func TestCacheInterface(t *testing.T) {
	cache := &mockCacheWithError{}

	// Test Set
	setErr := cache.Set(context.Background(), "key1", "value1")
	require.NoError(t, setErr)

	// Test Get
	val, getErr := cache.Get(context.Background(), "key1")
	require.NoError(t, getErr)
	assert.Equal(t, "value1", val)

	// Test Get non-existent key
	val2, getErr2 := cache.Get(context.Background(), "nonexistent")
	require.NoError(t, getErr2)
	assert.Nil(t, val2)
}

func TestCacheInterface_Errors(t *testing.T) {
	expectedErr := errors.New("cache error")

	t.Run("Get returns error", func(t *testing.T) {
		cache := &mockCacheWithError{getErr: expectedErr}
		_, err := cache.Get(context.Background(), "key")
		assert.Equal(t, expectedErr, err)
	})

	t.Run("Set returns error", func(t *testing.T) {
		cache := &mockCacheWithError{setErr: expectedErr}
		err := cache.Set(context.Background(), "key", "value")
		assert.Equal(t, expectedErr, err)
	})
}

func TestClientAdapterRemote_Evaluate_NoCache(t *testing.T) {
	expectedVariants := map[string]experiment.Variant{
		"flag-1": {Key: "on", Value: "enabled"},
	}
	evaluator := &mockRemoteEvaluator{
		fetchFunc: func(user *experiment.User) (map[string]experiment.Variant, error) {
			return expectedVariants, nil
		},
	}

	client := &clientAdapterRemote{
		evaluator: evaluator,
		cache:     nil,
	}

	user := &experiment.User{UserId: "user-1"}
	result, err := client.Evaluate(context.Background(), user, []string{"flag-1"})

	require.NoError(t, err)
	assert.Equal(t, expectedVariants, result)
	assert.Len(t, evaluator.fetchCalls, 1)
	assert.Equal(t, user, evaluator.fetchCalls[0])
}

func TestClientAdapterRemote_Evaluate_WithCache_CacheHit(t *testing.T) {
	expectedVariants := map[string]experiment.Variant{
		"flag-1": {Key: "on", Value: "enabled"},
	}
	evaluator := &mockRemoteEvaluator{}
	cache := &mockCacheWithError{}

	client := &clientAdapterRemote{
		evaluator: evaluator,
		cache:     cache,
	}

	user := &experiment.User{UserId: "user-1"}

	// First call - should fetch and cache
	evaluator.fetchFunc = func(user *experiment.User) (map[string]experiment.Variant, error) {
		return expectedVariants, nil
	}
	result1, err1 := client.Evaluate(context.Background(), user, nil)
	require.NoError(t, err1)
	assert.Equal(t, expectedVariants, result1)
	assert.Len(t, evaluator.fetchCalls, 1)
	assert.Len(t, cache.setCalls, 1)

	// Second call - should hit cache
	result2, err2 := client.Evaluate(context.Background(), user, nil)
	require.NoError(t, err2)
	assert.Equal(t, expectedVariants, result2)
	// Should not have made another fetch call
	assert.Len(t, evaluator.fetchCalls, 1)
}

func TestClientAdapterRemote_Evaluate_FetchError(t *testing.T) {
	expectedErr := errors.New("fetch error")
	evaluator := &mockRemoteEvaluator{
		fetchFunc: func(user *experiment.User) (map[string]experiment.Variant, error) {
			return nil, expectedErr
		},
	}

	client := &clientAdapterRemote{
		evaluator: evaluator,
		cache:     nil,
	}

	user := &experiment.User{UserId: "user-1"}
	result, err := client.Evaluate(context.Background(), user, nil)

	assert.Nil(t, result)
	assert.Equal(t, expectedErr, err)
}

func TestClientAdapterRemote_Evaluate_CacheSetError_LogsButSucceeds(t *testing.T) {
	expectedVariants := map[string]experiment.Variant{
		"flag-1": {Key: "on", Value: "enabled"},
	}
	evaluator := &mockRemoteEvaluator{
		fetchFunc: func(user *experiment.User) (map[string]experiment.Variant, error) {
			return expectedVariants, nil
		},
	}
	// Cache set returns error, but evaluation should still succeed
	cache := &mockCacheWithError{setErr: errors.New("cache set error")}

	client := &clientAdapterRemote{
		evaluator: evaluator,
		cache:     cache,
	}

	user := &experiment.User{UserId: "user-1"}
	result, err := client.Evaluate(context.Background(), user, nil)

	// Cache set errors should be logged but not fail the evaluation
	require.NoError(t, err)
	assert.Equal(t, expectedVariants, result)
}

func TestClientAdapterRemote_Evaluate_CacheGetError_StillFetches(t *testing.T) {
	expectedVariants := map[string]experiment.Variant{
		"flag-1": {Key: "on", Value: "enabled"},
	}
	evaluator := &mockRemoteEvaluator{
		fetchFunc: func(user *experiment.User) (map[string]experiment.Variant, error) {
			return expectedVariants, nil
		},
	}
	// Cache get returns error, but should still proceed to fetch
	cache := &mockCacheWithError{getErr: errors.New("cache get error")}

	client := &clientAdapterRemote{
		evaluator: evaluator,
		cache:     cache,
	}

	user := &experiment.User{UserId: "user-1"}
	result, err := client.Evaluate(context.Background(), user, nil)

	// Should still succeed by fetching
	require.NoError(t, err)
	assert.Equal(t, expectedVariants, result)
	assert.Len(t, evaluator.fetchCalls, 1)
}

