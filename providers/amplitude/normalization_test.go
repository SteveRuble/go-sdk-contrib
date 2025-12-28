package amplitude

import (
	"testing"

	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToAmplitudeUser(t *testing.T) {
	tests := []struct {
		name           string
		evalCtx        of.FlattenedContext
		expectedUserID string
		expectedDevice string
		expectedProps  map[string]any
		expectError    bool
		errorContains  string
	}{
		{
			name: "targeting key maps to user ID",
			evalCtx: of.FlattenedContext{
				of.TargetingKey: "user-123",
			},
			expectedUserID: "user-123",
			expectError:    false,
		},
		{
			name: "user_id maps to user ID",
			evalCtx: of.FlattenedContext{
				"user_id": "user-456",
			},
			expectedUserID: "user-456",
			expectError:    false,
		},
		{
			name: "device_id maps to device ID",
			evalCtx: of.FlattenedContext{
				"device_id": "device-789",
			},
			expectedDevice: "device-789",
			expectError:    false,
		},
		{
			name: "both user ID and device ID",
			evalCtx: of.FlattenedContext{
				of.TargetingKey: "user-123",
				"device_id":     "device-456",
			},
			expectedUserID: "user-123",
			expectedDevice: "device-456",
			expectError:    false,
		},
		{
			name: "unknown keys go to user properties",
			evalCtx: of.FlattenedContext{
				of.TargetingKey: "user-123",
				"custom_prop":   "custom_value",
				"another_prop":  123,
			},
			expectedUserID: "user-123",
			expectedProps: map[string]any{
				"custom_prop":  "custom_value",
				"another_prop": float64(123), // JSON marshaling converts to float64
			},
			expectError: false,
		},
		{
			name:          "empty context fails - no user ID or device ID",
			evalCtx:       of.FlattenedContext{},
			expectError:   true,
			errorContains: "must contain",
		},
		{
			name: "only custom properties fails - no user ID or device ID",
			evalCtx: of.FlattenedContext{
				"custom_prop": "value",
			},
			expectError:   true,
			errorContains: "must contain",
		},
		{
			name: "country maps correctly",
			evalCtx: of.FlattenedContext{
				of.TargetingKey: "user-123",
				"country":       "US",
			},
			expectedUserID: "user-123",
			expectError:    false,
		},
		{
			name: "platform maps correctly",
			evalCtx: of.FlattenedContext{
				of.TargetingKey: "user-123",
				"platform":      "iOS",
			},
			expectedUserID: "user-123",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a provider to call toAmplitudeUser (it's a method on Provider)
			provider := &Provider{}
			user, err := provider.toAmplitudeUser(tt.evalCtx)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, user)

			if tt.expectedUserID != "" {
				assert.Equal(t, tt.expectedUserID, user.UserId)
			}
			if tt.expectedDevice != "" {
				assert.Equal(t, tt.expectedDevice, user.DeviceId)
			}
			if tt.expectedProps != nil {
				for key, expectedVal := range tt.expectedProps {
					assert.Equal(t, expectedVal, user.UserProperties[key])
				}
			}
		})
	}
}

func TestToAmplitudeUser_StandardAmplitudeFields(t *testing.T) {
	// Test that standard Amplitude User fields are mapped correctly
	provider := &Provider{}

	evalCtx := of.FlattenedContext{
		of.TargetingKey: "user-123",
		string(KeyCountry):       "US",
		string(KeyRegion):        "CA",
		string(KeyCity):          "San Francisco",
		string(KeyLanguage):      "en",
		string(KeyPlatform):      "iOS",
		string(KeyVersion):       "1.0.0",
		string(KeyOs):            "iOS 16",
		string(KeyCarrier):       "Verizon",
		string(KeyLibrary):       "go-sdk",
		string(KeyUserProperties): map[string]any{
			"custom_prop": "custom_value",
		},
		string(KeyGroupProperties): map[string]map[string]any{
			"group-1": {
				"custom_prop": "custom_value",
			},
		},
		string(KeyGroups): map[string][]string{
			"group-1": {"group-1", "group-2"},
			"group-2": {"group-3", "group-4"},
		},
		string(KeyCohortIDs): map[string]struct{}{
			"cohort-1": {},
			"cohort-2": {},
		},
		string(KeyGroupCohortIDSet): map[string]map[string]map[string]struct{}{
			"group-1": {
				"cohort-1": {},
				"cohort-2": {},	
			},
		},
	}

	user, err := provider.toAmplitudeUser(evalCtx)
	require.NoError(t, err)

	assert.Equal(t, "user-123", user.UserId)
	assert.Equal(t, "US", user.Country)
	assert.Equal(t, "CA", user.Region)
	assert.Equal(t, "San Francisco", user.City)
	assert.Equal(t, "en", user.Language)
	assert.Equal(t, "iOS", user.Platform)
	assert.Equal(t, "1.0.0", user.Version)
	assert.Equal(t, "iOS 16", user.Os)
	assert.Equal(t, "Verizon", user.Carrier)
	assert.Equal(t, "go-sdk", user.Library)
	assert.Equal(t, map[string]any{"custom_prop": "custom_value"}, user.UserProperties)
	assert.Equal(t, map[string]map[string]any{"group-1": {"custom_prop": "custom_value"}}, user.GroupProperties)
	assert.Equal(t, map[string][]string{"group-1": {"group-1", "group-2"}, "group-2": {"group-3", "group-4"}}, user.Groups)
	assert.Equal(t, map[string]struct{}{"cohort-1": {}, "cohort-2": {}}, user.CohortIds)
	assert.Equal(t, map[string]map[string]map[string]struct{}{"group-1": {"cohort-1": {}, "cohort-2": {}}}, user.GroupCohortIds)
}

func TestToAmplitudeUser_DeviceFields(t *testing.T) {
	provider := &Provider{}

	evalCtx := of.FlattenedContext{
		"device_id":           "device-123",
		"device_manufacturer": "Apple",
		"device_brand":        "Apple",
		"device_model":        "iPhone 14",
	}

	user, err := provider.toAmplitudeUser(evalCtx)
	require.NoError(t, err)

	assert.Equal(t, "device-123", user.DeviceId)
	assert.Equal(t, "Apple", user.DeviceManufacturer)
	assert.Equal(t, "Apple", user.DeviceBrand)
	assert.Equal(t, "iPhone 14", user.DeviceModel)
}

func TestToAmplitudeUser_AlternateKeyFormats(t *testing.T) {
	// Test that various key formats (camelCase, kebab-case, PascalCase) work
	tests := []struct {
		name        string
		evalCtx     of.FlattenedContext
		checkField  func(t *testing.T, user interface{})
	}{
		{
			name: "userId camelCase",
			evalCtx: of.FlattenedContext{
				"userId": "user-123",
			},
			checkField: func(t *testing.T, u interface{}) {
				user := u.(testUserCheck)
				assert.Equal(t, "user-123", user.UserID)
			},
		},
		{
			name: "user-id kebab-case",
			evalCtx: of.FlattenedContext{
				"user-id": "user-123",
			},
			checkField: func(t *testing.T, u interface{}) {
				user := u.(testUserCheck)
				assert.Equal(t, "user-123", user.UserID)
			},
		},
		{
			name: "deviceId camelCase",
			evalCtx: of.FlattenedContext{
				"deviceId": "device-123",
			},
			checkField: func(t *testing.T, u interface{}) {
				user := u.(testUserCheck)
				assert.Equal(t, "device-123", user.DeviceID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{}
			user, err := provider.toAmplitudeUser(tt.evalCtx)
			require.NoError(t, err)
			tt.checkField(t, testUserCheck{user.UserId, user.DeviceId})
		})
	}
}

// testUserCheck is a helper struct for checking user fields in table tests
type testUserCheck struct {
	UserID   string
	DeviceID string
}

func TestToAmplitudeUser_MarshalError(t *testing.T) {
	// Test with a value that can't be JSON marshaled (channels, functions)
	provider := &Provider{}

	// Channels cannot be marshaled to JSON
	evalCtx := of.FlattenedContext{
		of.TargetingKey: "user-123",
		"invalid_field": make(chan int),
	}

	_, err := provider.toAmplitudeUser(evalCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal user map")
}
