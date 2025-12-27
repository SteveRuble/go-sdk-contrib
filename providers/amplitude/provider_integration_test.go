package amplitude_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
	"github.com/amplitude/experiment-go-server/pkg/experiment/remote"
	pkg "github.com/open-feature/go-sdk-contrib/providers/amplitude"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

const (
	// Test feature flag name
	testFlagName = "test-feature-flag"
)

// reNonWordOnly is a regular expression that matches any non-word characters.
var reNonWordOnly = regexp.MustCompile(`\W+`)

// vcrHarness contains the VCR recorder and deployment key for tests.
type vcrHarness struct {
	Recorder      *recorder.Recorder
	DeploymentKey string
}

// setupVCR configures go-vcr for recording or replaying HTTP interactions.
// If the AMPLITUDE_SDK_KEY environment variable is set, it records new interactions.
// Otherwise, it replays from existing cassettes.
// The function sets http.DefaultTransport to the VCR transport.
// Recordings are stored under testdata/ with filenames matching the test name.
func setupVCR(t *testing.T) vcrHarness {
	t.Helper()

	deploymentKey := os.Getenv("AMPLITUDE_SDK_KEY")
	shouldRecord := deploymentKey != ""

	mode := recorder.ModeReplayOnly
	if shouldRecord {
		mode = recorder.ModeRecordOnly
		t.Log("VCR in record mode")
	} else {
		// Use a placeholder key for replay mode
		deploymentKey = "server-replay-placeholder-key"
		t.Log("VCR in replay mode")
	}

	// Sanitize test name for use as filename
	cassetteName := filepath.Join("testdata", reNonWordOnly.ReplaceAllString(t.Name(), "_"))

	// Ensure testdata directory exists
	t.Logf("Creating testdata directory: %s", filepath.Dir(cassetteName))
	require.NoError(t, os.MkdirAll(filepath.Dir(cassetteName), 0755))

	// Hook to remove Authorization header before saving to avoid storing secrets
	removeAuthHook := func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")
		return nil
	}

	// Custom matcher: match on method, path, and content length
	// Don't match on headers since Authorization will differ between record/replay
	customMatcher := func(req *http.Request, i cassette.Request) bool {
		expectedURL, _ := url.Parse(i.URL)
		if expectedURL == nil {
			return false
		}
		return i.Method == req.Method &&
			expectedURL.Path == req.URL.Path &&
			expectedURL.RawQuery == req.URL.RawQuery &&
			req.ContentLength == i.ContentLength
	}

	// Create the recorder with all options
	r, recorderErr := recorder.New(
		cassetteName,
		recorder.WithMode(mode),
		recorder.WithSkipRequestLatency(true),
		recorder.WithHook(removeAuthHook, recorder.BeforeSaveHook),
		recorder.WithMatcher(customMatcher),
	)
	require.NoError(t, recorderErr)

	// Save the original transport
	originalTransport := http.DefaultTransport

	// Set the VCR transport as the default
	http.DefaultTransport = r

	// Restore original transport and stop recorder after test
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		require.NoError(t, r.Stop())
	})

	return vcrHarness{
		Recorder:      r,
		DeploymentKey: deploymentKey,
	}
}

// TestLocal runs the test suite with a local evaluation client.
func TestLocal(t *testing.T) {
	harness := setupVCR(t)

	provider, providerErr := pkg.New(
		context.Background(),
		harness.DeploymentKey,
		pkg.WithLocalConfig(local.Config{}),
	)
	require.NoError(t, providerErr)
	defer provider.Shutdown()

	s := &ProviderTestSuite{provider: provider}
	suite.Run(t, s)
}

// TestRemote runs the test suite with a remote evaluation client.
func TestRemote(t *testing.T) {
	harness := setupVCR(t)

	provider, providerErr := pkg.New(
		context.Background(),
		harness.DeploymentKey,
		pkg.WithRemoteConfig(remote.Config{}),
	)
	require.NoError(t, providerErr)
	defer provider.Shutdown()

	s := &ProviderTestSuite{provider: provider}
	suite.Run(t, s)
}

// ProviderTestSuite is a test suite for the Amplitude provider.
// It tests both local and remote evaluation modes.
type ProviderTestSuite struct {
	suite.Suite
	provider *pkg.Provider
}

// SetupSuite is called once before all tests in the suite.
func (s *ProviderTestSuite) SetupSuite() {
	initErr := s.provider.Init(openfeature.EvaluationContext{})
	s.Require().NoError(initErr)
}

// TearDownSuite is called once after all tests in the suite.
func (s *ProviderTestSuite) TearDownSuite() {
	s.provider.Shutdown()
}

func (s *ProviderTestSuite) TestStringEvaluation() {
	tests := []struct {
		name          string
		userID        string
		expectedValue string
	}{
		{
			name:          "expect-string user should see foo",
			userID:        "expect-string",
			expectedValue: "foo",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			evalCtx := openfeature.FlattenedContext{
				openfeature.TargetingKey: tt.userID,
			}

			result := s.provider.StringEvaluation(context.Background(), testFlagName, "default", evalCtx)

			s.Empty(result.ResolutionError, "expected no resolution error")
			s.Equal(tt.expectedValue, result.Value)
		})
	}
}

func (s *ProviderTestSuite) TestBooleanEvaluation() {
	tests := []struct {
		name          string
		userID        string
		expectedValue bool
	}{
		{
			name:          "regular user should see enabled (true)",
			userID:        "test-user-123",
			expectedValue: true,
		},
		{
			name:          "expect-enabled user should see enabled (true)",
			userID:        "expect-enabled",
			expectedValue: true,
		},
		{
			name:          "expect-disabled user should see disabled (false)",
			userID:        "expect-disabled",
			expectedValue: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			evalCtx := openfeature.FlattenedContext{
				openfeature.TargetingKey: tt.userID,
			}

			result := s.provider.BooleanEvaluation(context.Background(), testFlagName, false, evalCtx)

			// Boolean evaluation: payload bool if available, else non-"off" = true, "off" = default
			s.Empty(result.ResolutionError, "expected no resolution error")
			s.Equal(tt.expectedValue, result.Value)
		})
	}
}

func (s *ProviderTestSuite) TestIntEvaluation() {
	tests := []struct {
		name          string
		userID        string
		expectedValue int64
	}{
		{
			name:          "expect-int user should see 42",
			userID:        "expect-int",
			expectedValue: 42,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			evalCtx := openfeature.FlattenedContext{
				openfeature.TargetingKey: tt.userID,
			}

			result := s.provider.IntEvaluation(context.Background(), testFlagName, 0, evalCtx)

			s.Empty(result.ResolutionError, "expected no resolution error")
			s.Equal(tt.expectedValue, result.Value)
		})
	}
}

func (s *ProviderTestSuite) TestFloatEvaluation() {
	tests := []struct {
		name          string
		userID        string
		expectedValue float64
	}{
		{
			name:          "expect-float user should see 12.34",
			userID:        "expect-float",
			expectedValue: 12.34,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			evalCtx := openfeature.FlattenedContext{
				openfeature.TargetingKey: tt.userID,
			}

			result := s.provider.FloatEvaluation(context.Background(), testFlagName, 0.0, evalCtx)

			s.Empty(result.ResolutionError, "expected no resolution error")
			s.Equal(tt.expectedValue, result.Value)
		})
	}
}

func (s *ProviderTestSuite) TestObjectEvaluation() {
	tests := []struct {
		name          string
		userID        string
		expectedValue map[string]any
	}{
		{
			name:   "expect-object user should see object with a and b",
			userID: "expect-object",
			expectedValue: map[string]any{
				"a": "A",
				"b": "B",
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			evalCtx := openfeature.FlattenedContext{
				openfeature.TargetingKey: tt.userID,
			}

			result := s.provider.ObjectEvaluation(context.Background(), testFlagName, nil, evalCtx)

			s.Empty(result.ResolutionError, "expected no resolution error")
			s.Equal(tt.expectedValue, result.Value)
		})
	}
}

// TestProviderNotReady tests that the provider returns errors when not initialized.
func TestProviderNotReady(t *testing.T) {
	harness := setupVCR(t)

	provider, providerErr := pkg.New(
		context.Background(),
		harness.DeploymentKey,
	)
	require.NoError(t, providerErr)
	defer provider.Shutdown()

	// Don't call Init - provider should not be ready

	evalCtx := openfeature.FlattenedContext{
		openfeature.TargetingKey: "test-user",
	}

	t.Run("BooleanEvaluation returns error when not ready", func(t *testing.T) {
		result := provider.BooleanEvaluation(context.Background(), testFlagName, false, evalCtx)
		require.NotEmpty(t, result.ResolutionError)
		require.Equal(t, false, result.Value) // Should return default
	})

	t.Run("StringEvaluation returns error when not ready", func(t *testing.T) {
		result := provider.StringEvaluation(context.Background(), testFlagName, "default", evalCtx)
		require.NotEmpty(t, result.ResolutionError)
		require.Equal(t, "default", result.Value) // Should return default
	})
}

// TestMissingTargetingKey tests that the provider returns errors when targeting key is missing.
func TestMissingTargetingKey(t *testing.T) {
	harness := setupVCR(t)

	provider, providerErr := pkg.New(
		context.Background(),
		harness.DeploymentKey,
	)
	require.NoError(t, providerErr)

	initErr := provider.Init(openfeature.EvaluationContext{})
	require.NoError(t, initErr)
	defer provider.Shutdown()

	// Empty eval context - no targeting key
	evalCtx := openfeature.FlattenedContext{}

	result := provider.StringEvaluation(context.Background(), testFlagName, "default", evalCtx)

	require.NotEmpty(t, result.ResolutionError)
	require.Equal(t, "default", result.Value)
}

// TestNewProvider_MissingDeploymentKey tests that NewProvider returns an error when deployment key is missing.
func TestNewProvider_MissingDeploymentKey(t *testing.T) {
	_, providerErr := pkg.New(context.Background(), "")
	require.Error(t, providerErr)
	require.True(t, strings.Contains(providerErr.Error(), "DeploymentKey is required"))
}

// Unused imports guard - ensures experiment package is available for type references
var _ experiment.Variant
