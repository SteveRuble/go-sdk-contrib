package amplitude

import (
	"context"

	"github.com/amplitude/experiment-go-server/pkg/experiment"
)

// clientWrapper is an interface for evaluating feature flags using the
// Amplitude Experiment SDK. It abstracts over local and remote evaluation modes.
type clientAdapter interface {
	// Evaluate evaluates the given flags for the given user and returns a map
	// of flag keys to variants. If flagKeys is nil or empty, all flags are evaluated.
	Evaluate(ctx context.Context, user *experiment.User, flagKeys []string) (map[string]experiment.Variant, error)
	// Start starts the experiment client.
	Start() error
	// Stop stops the experiment client.
	Stop() error
}
