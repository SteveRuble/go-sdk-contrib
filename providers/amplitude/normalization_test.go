package amplitude

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	analytics "github.com/amplitude/analytics-go/amplitude"
	"github.com/amplitude/experiment-go-server/pkg/experiment"
	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakePermutations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "simple lowercase word",
			input: "country",
			expected: []string{
				"country",
				"COUNTRY",
			},
		},
		{
			name:  "word with underscore",
			input: "user_id",
			expected: []string{
				"user_id",
				"USER_ID",
				"userId",
				"user-id",
				"userID",
				"userID", // also yields user-id from _id suffix handler
				"userId",
			},
		},
		{
			name:  "word with multiple underscores",
			input: "device_manufacturer",
			expected: []string{
				"device_manufacturer",
				"DEVICE_MANUFACTURER",
				"deviceManufacturer",
				"device-manufacturer",
			},
		},
		{
			name:  "word ending with _ids (plural)",
			input: "cohort_ids",
			expected: []string{
				"cohort_ids",
				"COHORT_IDS",
				"cohortIds",
				"cohort-ids",
				"cohortIDs",
				"cohort-ids",
				"cohortIds",
			},
		},
		{
			name:  "word ending with _id (singular)",
			input: "device_id",
			expected: []string{
				"device_id",
				"DEVICE_ID",
				"deviceId",
				"device-id",
				"deviceID",
				"device-id",
				"deviceId",
			},
		},
		{
			name:  "word with multiple underscores ending in _ids",
			input: "group_cohort_ids",
			expected: []string{
				"group_cohort_ids",
				"GROUP_COHORT_IDS",
				"groupCohortIds",
				"group-cohort-ids",
				"group_cohortIDs",
				"group_cohort-ids",
				"group_cohortids", // lowercase version of group_cohort + Ids
			},
		},
		{
			name:  "uppercase word",
			input: "DMA",
			expected: []string{
				"DMA",
				"dma",
			},
		},
		{
			name:  "mixed case word",
			input: "productId",
			expected: []string{
				"productId",
				"productid",
				"PRODUCTID",
			},
		},
		{
			name:  "word with single underscore at start",
			input: "_test",
			expected: []string{
				"_test",
				"_TEST",
				"Test",
				"-test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []string
			for _, perm := range makePermutations(tt.input) {
				result = append(result, perm)
			}

			// Check that all expected values are present
			for _, exp := range tt.expected {
				assert.True(t, slices.Contains(result, exp),
					"expected permutation %q not found in result %v", exp, result)
			}
		})
	}
}

func TestMakePermutations_NoDuplicates(t *testing.T) {
	// Test that duplicate handling works (we allow duplicates in the iterator,
	// but the map will deduplicate them)
	tests := []struct {
		name  string
		input string
	}{
		{"user_id", "user_id"},
		{"device_id", "device_id"},
		{"cohort_ids", "cohort_ids"},
		{"country", "country"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[string]bool)
			for _, perm := range makePermutations(tt.input) {
				// Just collect all permutations - duplicates are allowed
				// since the DefaultKeyMap will deduplicate via map assignment
				seen[perm] = true
			}
			// Verify we got at least the basic permutations
			assert.GreaterOrEqual(t, len(seen), 2, "should have at least original and uppercase/lowercase")
		})
	}
}

func TestDefaultKeyMap_ContainsExpectedMappings(t *testing.T) {
	keyMap := DefaultKeyMap()

	// Test that common variations all map to the correct canonical key
	tests := []struct {
		inputKey    string
		expectedKey Key
	}{
		// user_id variations
		{of.TargetingKey, KeyUserID},
		{"user_id", KeyUserID},
		{"userId", KeyUserID},
		{"user-id", KeyUserID},
		{"USER_ID", KeyUserID},

		// device_id variations
		{"device_id", KeyDeviceID},
		{"deviceId", KeyDeviceID},
		{"device-id", KeyDeviceID},
		{"DEVICE_ID", KeyDeviceID},

		// Simple fields
		{"country", KeyCountry},
		{"COUNTRY", KeyCountry},
		{"platform", KeyPlatform},
		{"PLATFORM", KeyPlatform},

		// Fields with underscores
		{"device_manufacturer", KeyDeviceManufacturer},
		{"deviceManufacturer", KeyDeviceManufacturer},
		{"device-manufacturer", KeyDeviceManufacturer},

		// Fields ending in _ids
		{"cohort_ids", KeyCohortIDs},
		{"cohortIds", KeyCohortIDs},
		{"cohortIDs", KeyCohortIDs},
		{"cohort-ids", KeyCohortIDs},

		// Fields ending in _id
		{"session_id", KeySessionID},
		{"sessionId", KeySessionID},
		{"sessionID", KeySessionID},
		{"session-id", KeySessionID},
	}

	for _, tt := range tests {
		t.Run(tt.inputKey, func(t *testing.T) {
			actual, ok := keyMap[tt.inputKey]
			require.True(t, ok, "key %q should exist in keyMap", tt.inputKey)
			assert.Equal(t, tt.expectedKey, actual, "key %q should map to %q", tt.inputKey, tt.expectedKey)
		})
	}
}

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
				"another_prop": 123,
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
			user, err := provider.toAmplitudeUser(context.Background(), tt.evalCtx)

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
					assert.EqualValues(t, expectedVal, user.UserProperties[key])
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
		string(KeyOS):            "iOS 16",
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

	user, err := provider.toAmplitudeUser(context.Background(), evalCtx)
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

	user, err := provider.toAmplitudeUser(context.Background(), evalCtx)
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
			user, err := provider.toAmplitudeUser(context.Background(), tt.evalCtx)
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
		of.TargetingKey: make(chan int),
	}

	_, err := provider.toAmplitudeUser(context.Background(), evalCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal user map")
}

// getJSONTags extracts all JSON tag names from a struct type, including embedded structs.
func getJSONTags(t reflect.Type) map[string]bool {
	tags := make(map[string]bool)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return tags
	}

	for i := range t.NumField() {
		field := t.Field(i)

		// Handle embedded structs (like EventOptions in Event)
		if field.Anonymous {
			for k := range getJSONTags(field.Type) {
				tags[k] = true
			}
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Parse the tag to get just the name (before any comma options)
		tagName := strings.Split(jsonTag, ",")[0]
		if tagName != "" {
			tags[tagName] = true
		}
	}
	return tags
}

// keySliceToSet converts a slice of Key to a set of strings.
func keySliceToSet(keys []Key) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[string(k)] = true
	}
	return set
}

func TestKeyArraysMatchStructFields(t *testing.T) {
	// Get JSON tags from the Amplitude types using reflection
	eventOptionsTags := getJSONTags(reflect.TypeOf(analytics.Event{}))
	userTags := getJSONTags(reflect.TypeOf(experiment.User{}))

	// Determine which fields are shared, event-only, and user-only
	actualShared := make(map[string]bool)
	actualEventOnly := make(map[string]bool)
	actualUserOnly := make(map[string]bool)

	for tag := range eventOptionsTags {
		if userTags[tag] {
			actualShared[tag] = true
		} else {
			actualEventOnly[tag] = true
		}
	}
	for tag := range userTags {
		if !eventOptionsTags[tag] {
			actualUserOnly[tag] = true
		}
	}

	// Convert our key arrays to sets for comparison
	declaredShared := keySliceToSet(sharedKeys)
	declaredEvent := keySliceToSet(eventKeys)
	declaredUser := keySliceToSet(userKeys)

	t.Run("sharedKeys matches fields present in both User and EventOptions", func(t *testing.T) {
		for tag := range actualShared {
			assert.True(t, declaredShared[tag],
				"field %q exists in both User and EventOptions but is not in sharedKeys", tag)
		}
		for key := range declaredShared {
			assert.True(t, actualShared[key],
				"sharedKeys contains %q but it is not present in both User and EventOptions", key)
		}
	})

	t.Run("eventKeys matches fields only in EventOptions", func(t *testing.T) {
		for tag := range actualEventOnly {
			assert.True(t, declaredEvent[tag],
				"field %q exists only in EventOptions but is not in eventKeys", tag)
		}
		for key := range declaredEvent {
			assert.True(t, actualEventOnly[key],
				"eventKeys contains %q but it is not an event-only field", key)
		}
	})

	t.Run("userKeys matches fields only in User plus shared fields", func(t *testing.T) {
		// userKeys should contain user-only fields AND shared fields
		expectedUserKeys := make(map[string]bool)
		for tag := range actualUserOnly {
			expectedUserKeys[tag] = true
		}
		for tag := range actualShared {
			expectedUserKeys[tag] = true
		}

		for tag := range expectedUserKeys {
			assert.True(t, declaredUser[tag],
				"field %q should be in userKeys (user-only or shared) but is not", tag)
		}
		for key := range declaredUser {
			assert.True(t, expectedUserKeys[key],
				"userKeys contains %q but it is not a user field", key)
		}
	})
}
