package amplitude

import (
	"strings"

	of "github.com/open-feature/go-sdk/openfeature"
)

// Key is the type for the keys in the Amplitude User type.
type Key string

const (
	// KeyUserID is the canonical key for the user ID in the Amplitude User type.
	// The primary key for the user.
	// Automatically mapped from the targeting key.
	KeyUserID Key = "user_id"
	// KeyDeviceID is the canonical key for the device ID in the Amplitude User type.
	KeyDeviceID Key = "device_id"
	// KeyCountry is the canonical key for the country in the Amplitude User type.
	KeyCountry Key = "country"
	// KeyRegion is the canonical key for the region in the Amplitude User type.
	KeyRegion Key = "region"
	// KeyDma is the canonical key for the DMA in the Amplitude User type.
	KeyDma Key = "dma"
	// KeyCity is the canonical key for the city in the Amplitude User type.
	KeyCity Key = "city"
	// KeyLanguage is the canonical key for the language in the Amplitude User type.
	KeyLanguage Key = "language"
	// KeyPlatform is the canonical key for the platform in the Amplitude User type.
	KeyPlatform Key = "platform"
	// KeyVersion is the canonical key for the version in the Amplitude User type.
	KeyVersion Key = "version"
	// KeyOs is the canonical key for the OS in the Amplitude User type.
	KeyOs Key = "os"
	// KeyDeviceManufacturer is the canonical key for the device manufacturer in the Amplitude User type.
	KeyDeviceManufacturer Key = "device_manufacturer"
	// KeyDeviceBrand is the canonical key for the device brand in the Amplitude User type.
	KeyDeviceBrand Key = "device_brand"
	// KeyDeviceModel is the canonical key for the device model in the Amplitude User type.
	KeyDeviceModel Key = "device_model"
	// KeyCarrier is the canonical key for the carrier in the Amplitude User type.
	KeyCarrier Key = "carrier"
	// KeyLibrary is the canonical key for the library in the Amplitude User type.
	KeyLibrary Key = "library"
	// KeyUserProperties is the canonical key for the user properties in the Amplitude User type.
	// The corresponding value is a map[string]interface{}.
	KeyUserProperties Key = "user_properties"
	// KeyGroupProperties is the canonical key for the group properties in the Amplitude User type.
	// The corresponding value is a map[string]map[string]interface{}.
	KeyGroupProperties Key = "group_properties"
	// KeyGroups is the canonical key for the groups in the Amplitude User type.
	// The corresponding value is a map[string][]string.
	KeyGroups Key = "groups"
	// KeyCohortIDs is the canonical key for the cohort IDs in the Amplitude User type.
	// The corresponding value is a map[string]struct{}.
	KeyCohortIDs Key = "cohort_ids"
	// KeyGroupCohortIDSet is the canonical key for the group cohort IDs in the Amplitude User type.
	// The corresponding value is a map[string]map[string]map[string]struct{}.
	KeyGroupCohortIDSet Key = "group_cohort_ids"
)

// DefaultKeyMap is a map of string keys that might be in the evaluation context
// to the canonical key used by Amplitude.
// You can add keys to this map to automatically map the keys in the evaluation context
// to the canonical keys used by Amplitude.
// Any keys that are not mapped will be added to the [User.UserProperties] map.
// For more advanced normalization, use a hook to pre-process the evaluation context.
func DefaultKeyMap() map[string]Key {
	var keyMap = map[string]Key{}
	for k, values := range map[Key][]string{
		KeyUserID:             {string(KeyUserID), of.TargetingKey, "userId", "user-id", "UserId", "UserID"},
		KeyDeviceID:           {string(KeyDeviceID), "deviceId", "device-id", "DeviceId", "DeviceID"},
		KeyCountry:            {string(KeyCountry), "Country"},
		KeyRegion:             {string(KeyRegion), "Region"},
		KeyDma:                {string(KeyDma),  "Dma", "DMA"},
		KeyCity:               {string(KeyCity), "City"},
		KeyLanguage:           {string(KeyLanguage), "Language"},
		KeyPlatform:           {string(KeyPlatform), "Platform"},
		KeyVersion:            {string(KeyVersion), "Version"},
		KeyOs:                 {string(KeyOs), "Os", "OS"},
		KeyDeviceManufacturer: {string(KeyDeviceManufacturer), "deviceManufacturer", "device-manufacturer", "DeviceManufacturer"},
		KeyDeviceBrand:        {string(KeyDeviceBrand), "deviceBrand", "device-brand", "DeviceBrand"},
		KeyDeviceModel:        {string(KeyDeviceModel), "deviceModel", "device-model", "DeviceModel"},
		KeyCarrier:            {string(KeyCarrier), "Carrier"},
		KeyLibrary:            {string(KeyLibrary), "Library"},
		KeyUserProperties:     {string(KeyUserProperties), "userProperties", "user-properties", "UserProperties"},
		KeyGroupProperties:    {string(KeyGroupProperties), "groupProperties", "group-properties", "GroupProperties"},
		KeyGroups:             {string(KeyGroups), "Groups"},
		KeyCohortIDs:          {string(KeyCohortIDs), "cohortIds", "cohort-ids", "CohortIds", "cohortIDs"},
		KeyGroupCohortIDSet:   {string(KeyGroupCohortIDSet), "groupCohortIds", "group-cohort-ids", "GroupCohortIds", "groupCohortIDs"},
	} {
		for _, value := range values {
			keyMap[value] = k
			keyMap[strings.ToLower(value)] = k
		}
	}
	return keyMap
}
