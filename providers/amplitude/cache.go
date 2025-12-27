package amplitude

import "context"

// Cache is an interface for a cache.
// You may want to provide an implementation using a library like github.com/hashicorp/golang-lru/v2,
// or an implementation which expects a mutable value to be added to the context
// early in the request pipeline and then uses it to cache values for the duration of the request.
// This will mean that flags are evaluated once per request, rather than once per flag evaluation.
type Cache interface {
	// Set sets the value for the given key.
	Set(ctx context.Context, key string, value any) error
	// Get gets the value for the given key.
	Get(ctx context.Context, key string) (any, error)
}
