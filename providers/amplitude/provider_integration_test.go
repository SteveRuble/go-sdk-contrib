package amplitude_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	experiment "github.com/amplitude/experiment-go-server/pkg/experiment"
	"github.com/amplitude/experiment-go-server/pkg/experiment/local"
	"github.com/amplitude/experiment-go-server/pkg/experiment/remote"
	pkg "github.com/open-feature/go-sdk-contrib/providers/amplitude"
	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/suite"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

const (
	// testFlagName is the name of the feature flag used in tests.
	testFlagName = "test-feature-flag"

	// testFlagConfigPath is the path to the test flag configuration JSON.
	testFlagConfigPath = "testdata/test-flag.json"

	// amplitudeManagementAPIBase is the base URL for the Amplitude Management API.
	amplitudeManagementAPIBase = "https://experiment.amplitude.com/api/1"
)

// reNonWordOnly is a regular expression that matches any non-word characters.
var reNonWordOnly = regexp.MustCompile(`\W+`)

// IntegrationTestSuite is the main test suite for integration tests.
// It handles VCR setup, flag management, and provider initialization.
type IntegrationTestSuite struct {
	suite.Suite
	deploymentKey     string
	managementAPIKey  string
	projectID         string
	deploymentID      string
	recorder          *recorder.Recorder
	originalTransport http.RoundTripper
}

// SetupSuite is called once before all tests in the suite.
// It configures VCR and ensures the test flag exists in Amplitude.
func (s *IntegrationTestSuite) SetupSuite() {
	s.deploymentKey = os.Getenv("AMPLITUDE_SDK_KEY")
	s.managementAPIKey = os.Getenv("AMPLITUDE_MANAGEMENT_API_KEY")
	s.projectID = os.Getenv("AMPLITUDE_PROJECT_ID")
	s.deploymentID = os.Getenv("AMPLITUDE_DEPLOYMENT_ID")

	shouldRecord := s.deploymentKey != "" && s.managementAPIKey != ""

	if shouldRecord {
		s.T().Log("Recording mode: AMPLITUDE_SDK_KEY and AMPLITUDE_MANAGEMENT_API_KEY are set")
		s.ensureTestFlagExists()
	} else {
		s.T().Log("Replay mode: using VCR cassettes")
		s.deploymentKey = "server-replay-placeholder-key"
	}
}

// TearDownSuite is called once after all tests in the suite.
func (s *IntegrationTestSuite) TearDownSuite() {
	// Nothing to clean up at suite level
}

// setupVCR configures go-vcr for recording or replaying HTTP interactions.
func (s *IntegrationTestSuite) setupVCR(testName string) {
	shouldRecord := os.Getenv("AMPLITUDE_SDK_KEY") != ""

	mode := recorder.ModeReplayOnly
	if shouldRecord {
		mode = recorder.ModeRecordOnly
		s.T().Log("VCR in record mode")
	} else {
		s.T().Log("VCR in replay mode")
	}

	cassetteName := filepath.Join("testdata", reNonWordOnly.ReplaceAllString(testName, "_"))
	s.Require().NoError(os.MkdirAll(filepath.Dir(cassetteName), 0755))

	removeAuthHook := func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")
		return nil
	}

	customMatcher := func(req *http.Request, i cassette.Request) bool {
		expectedURL, _ := url.Parse(i.URL)
		if expectedURL == nil {
			return false
		}
		// Basic matching: method, path, query
		if i.Method != req.Method ||
			expectedURL.Path != req.URL.Path ||
			expectedURL.RawQuery != req.URL.RawQuery {
			return false
		}
		// Match X-Amp-Exp-User header if present (used by remote evaluation)
		// This header contains the base64-encoded user context
		recordedUserHeader := ""
		if vals, ok := i.Headers["X-Amp-Exp-User"]; ok && len(vals) > 0 {
			recordedUserHeader = vals[0]
		}
		requestUserHeader := req.Header.Get("X-Amp-Exp-User")
		return recordedUserHeader == requestUserHeader
	}

	r, recorderErr := recorder.New(
		cassetteName,
		recorder.WithMode(mode),
		recorder.WithSkipRequestLatency(true),
		recorder.WithHook(removeAuthHook, recorder.BeforeSaveHook),
		recorder.WithMatcher(customMatcher),
	)
	s.Require().NoError(recorderErr)

	s.originalTransport = http.DefaultTransport
	http.DefaultTransport = r
	s.recorder = r
}

// tearDownVCR restores the original transport and stops the recorder.
func (s *IntegrationTestSuite) tearDownVCR() {
	if s.originalTransport != nil {
		http.DefaultTransport = s.originalTransport
	}
	if s.recorder != nil {
		s.Require().NoError(s.recorder.Stop())
	}
}

// ensureTestFlagExists uses the Management API to create or update the test flag.
func (s *IntegrationTestSuite) ensureTestFlagExists() {
	s.T().Log("Ensuring test flag exists via Management API...")

	flagConfig, loadErr := s.loadFlagConfig()
	s.Require().NoError(loadErr, "failed to load flag config from %s", testFlagConfigPath)

	// Try to find existing flag
	existingFlag, findErr := s.findFlagByKey(testFlagName)
	if findErr != nil {
		s.T().Logf("Error finding flag: %v", findErr)
	}

	if existingFlag != nil {
		s.T().Logf("Flag %q exists with ID %v, updating...", testFlagName, existingFlag["id"])
		updateErr := s.updateFlag(existingFlag["id"], flagConfig)
		s.Require().NoError(updateErr, "failed to update flag")
		s.T().Log("Flag updated successfully")
	} else {
		s.T().Logf("Flag %q not found, creating...", testFlagName)
		flagID, createErr := s.createFlag(flagConfig)
		s.Require().NoError(createErr, "failed to create flag")
		s.T().Log("Flag created successfully")

		// Enable the flag after creation (new flags are disabled by default)
		s.T().Log("Enabling newly created flag...")
		enableErr := s.enableFlag(flagID)
		s.Require().NoError(enableErr, "failed to enable flag")
		s.T().Log("Flag enabled successfully")
	}
}

// loadFlagConfig loads the flag configuration from the JSON file.
// It replaces placeholder values with actual values from environment variables.
func (s *IntegrationTestSuite) loadFlagConfig() (map[string]any, error) {
	data, readErr := os.ReadFile(testFlagConfigPath)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read flag config: %w", readErr)
	}

	var config map[string]any
	if unmarshalErr := json.Unmarshal(data, &config); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse flag config: %w", unmarshalErr)
	}

	// Replace projectId placeholder with environment variable
	if s.projectID != "" {
		config["projectId"] = s.projectID
	}

	// Replace deployments placeholder with environment variable
	if s.deploymentID != "" {
		config["deployments"] = []string{s.deploymentID}
	}

	return config, nil
}

// findFlagByKey searches for a flag by its key using the Management API.
func (s *IntegrationTestSuite) findFlagByKey(key string) (map[string]any, error) {
	reqURL := fmt.Sprintf("%s/flags?key=%s", amplitudeManagementAPIBase, url.QueryEscape(key))

	req, reqErr := http.NewRequest(http.MethodGet, reqURL, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request: %w", reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+s.managementAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		return nil, fmt.Errorf("failed to execute request: %w", respErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Flags []map[string]any `json:"flags"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	// Find the flag with matching key
	for _, flag := range result.Flags {
		if flag["key"] == key {
			return flag, nil
		}
	}

	return nil, nil
}

// createFlag creates a new flag using the Management API.
// It returns the ID of the created flag.
func (s *IntegrationTestSuite) createFlag(config map[string]any) (string, error) {
	reqURL := fmt.Sprintf("%s/flags", amplitudeManagementAPIBase)

	body, marshalErr := json.Marshal(config)
	if marshalErr != nil {
		return "", fmt.Errorf("failed to marshal config: %w", marshalErr)
	}

	req, reqErr := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(body))
	if reqErr != nil {
		return "", fmt.Errorf("failed to create request: %w", reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+s.managementAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		return "", fmt.Errorf("failed to execute request: %w", respErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response to get the flag ID
	var result struct {
		ID string `json:"id"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return "", fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return result.ID, nil
}

// enableFlag enables a flag using the Management API.
func (s *IntegrationTestSuite) enableFlag(flagID string) error {
	reqURL := fmt.Sprintf("%s/flags/%s", amplitudeManagementAPIBase, flagID)

	body, marshalErr := json.Marshal(map[string]any{"enabled": true})
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal config: %w", marshalErr)
	}

	req, reqErr := http.NewRequest(http.MethodPatch, reqURL, bytes.NewReader(body))
	if reqErr != nil {
		return fmt.Errorf("failed to create request: %w", reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+s.managementAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		return fmt.Errorf("failed to execute request: %w", respErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// updateFlag updates an existing flag using the Management API.
func (s *IntegrationTestSuite) updateFlag(flagID any, config map[string]any) error {
	reqURL := fmt.Sprintf("%s/flags/%v", amplitudeManagementAPIBase, flagID)

	// Remove fields that shouldn't be in PATCH request
	updateConfig := make(map[string]any)
	for k, v := range config {
		// Skip read-only, immutable, and unsupported fields
		if k == "id" || k == "key" || k == "projectId" || k == "deployments" || k == "createdBy" ||
			k == "lastModifiedBy" || k == "createdAt" || k == "lastModifiedAt" || k == "deleted" ||
			k == "rolloutWeights" || k == "variants" {
			continue
		}
		updateConfig[k] = v
	}

	body, marshalErr := json.Marshal(updateConfig)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal config: %w", marshalErr)
	}

	req, reqErr := http.NewRequest(http.MethodPatch, reqURL, bytes.NewReader(body))
	if reqErr != nil {
		return fmt.Errorf("failed to create request: %w", reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+s.managementAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		return fmt.Errorf("failed to execute request: %w", respErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// TestIntegration runs the integration test suite.
func TestIntegration(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

// TestLocalEvaluation tests the provider with local evaluation.
func (s *IntegrationTestSuite) TestLocalEvaluation() {
	s.setupVCR(s.T().Name())
	defer s.tearDownVCR()

	provider, providerErr := pkg.New(
		context.Background(),
		s.deploymentKey,
		pkg.WithLocalConfig(local.Config{}),
	)
	s.Require().NoError(providerErr)
	defer provider.Shutdown()

	initErr := provider.Init(of.EvaluationContext{})
	s.Require().NoError(initErr)

	s.runEvaluationTests(provider)
}

// TestRemoteEvaluation tests the provider with remote evaluation.
func (s *IntegrationTestSuite) TestRemoteEvaluation() {
	s.setupVCR(s.T().Name())
	defer s.tearDownVCR()

	provider, providerErr := pkg.New(
		context.Background(),
		s.deploymentKey,
		pkg.WithRemoteConfig(remote.Config{}),
	)
	s.Require().NoError(providerErr)
	defer provider.Shutdown()

	initErr := provider.Init(of.EvaluationContext{})
	s.Require().NoError(initErr)

	s.runEvaluationTests(provider)
}

// runEvaluationTests runs all evaluation tests against the given provider.
func (s *IntegrationTestSuite) runEvaluationTests(provider *pkg.Provider) {
	s.Run("BooleanEvaluation", func() {
		tests := []struct {
			name          string
			userID        string
			expectedValue bool
		}{
			{
				name:          "expect-enabled user should see enabled (true)",
				userID:        "expect-enabled",
				expectedValue: true,
			},
		}

		for _, tt := range tests {
			s.Run(tt.name, func() {
				evalCtx := of.FlattenedContext{
					of.TargetingKey: tt.userID,
				}

				result := provider.BooleanEvaluation(context.Background(), testFlagName, false, evalCtx)

				s.Equal(of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
				s.Equal(tt.expectedValue, result.Value)
			})
		}
	})

	s.Run("StringEvaluation", func() {
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
				evalCtx := of.FlattenedContext{
					of.TargetingKey: tt.userID,
				}

				result := provider.StringEvaluation(context.Background(), testFlagName, "default", evalCtx)

				s.Equal(of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
				s.Equal(tt.expectedValue, result.Value)
			})
		}
	})

	s.Run("IntEvaluation", func() {
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
				evalCtx := of.FlattenedContext{
					of.TargetingKey: tt.userID,
				}

				result := provider.IntEvaluation(context.Background(), testFlagName, 0, evalCtx)

				s.Equal(of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
				s.Equal(tt.expectedValue, result.Value)
			})
		}
	})

	s.Run("FloatEvaluation", func() {
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
				evalCtx := of.FlattenedContext{
					of.TargetingKey: tt.userID,
				}

				result := provider.FloatEvaluation(context.Background(), testFlagName, 0.0, evalCtx)

				s.Equal(of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
				s.Equal(tt.expectedValue, result.Value)
			})
		}
	})

	s.Run("ObjectEvaluation", func() {
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
				evalCtx := of.FlattenedContext{
					of.TargetingKey: tt.userID,
				}

				result := provider.ObjectEvaluation(context.Background(), testFlagName, nil, evalCtx)

				s.Equal(of.ResolutionError{}, result.ResolutionError, "expected no resolution error")
				s.Equal(tt.expectedValue, result.Value)
			})
		}
	})
}

// TestProviderNotReady tests that the provider returns errors when not initialized.
func (s *IntegrationTestSuite) TestProviderNotReady() {
	s.setupVCR(s.T().Name())
	defer s.tearDownVCR()

	provider, providerErr := pkg.New(
		context.Background(),
		s.deploymentKey,
	)
	s.Require().NoError(providerErr)
	defer provider.Shutdown()

	// Don't call Init - provider should not be ready

	evalCtx := of.FlattenedContext{
		of.TargetingKey: "test-user",
	}

	s.Run("BooleanEvaluation returns error when not ready", func() {
		result := provider.BooleanEvaluation(context.Background(), testFlagName, false, evalCtx)
		s.NotEmpty(result.ResolutionError)
		s.Equal(false, result.Value) // Should return default
	})

	s.Run("StringEvaluation returns error when not ready", func() {
		result := provider.StringEvaluation(context.Background(), testFlagName, "default", evalCtx)
		s.NotEmpty(result.ResolutionError)
		s.Equal("default", result.Value) // Should return default
	})
}

// TestMissingTargetingKey tests that the provider returns errors when targeting key is missing.
func (s *IntegrationTestSuite) TestMissingTargetingKey() {
	s.setupVCR(s.T().Name())
	defer s.tearDownVCR()

	provider, providerErr := pkg.New(
		context.Background(),
		s.deploymentKey,
	)
	s.Require().NoError(providerErr)

	initErr := provider.Init(of.EvaluationContext{})
	s.Require().NoError(initErr)
	defer provider.Shutdown()

	// Empty eval context - no targeting key
	evalCtx := of.FlattenedContext{}

	result := provider.StringEvaluation(context.Background(), testFlagName, "default", evalCtx)

	s.NotEmpty(result.ResolutionError)
	s.Equal("default", result.Value)
}

// TestNewProvider_MissingDeploymentKey tests that NewProvider returns an error when deployment key is missing.
func (s *IntegrationTestSuite) TestNewProvider_MissingDeploymentKey() {
	_, providerErr := pkg.New(context.Background(), "")
	s.Require().Error(providerErr)
	s.Contains(providerErr.Error(), "you must provide a deployment key")
}

// Unused imports guard - ensures experiment package is available for type references
var _ experiment.Variant
