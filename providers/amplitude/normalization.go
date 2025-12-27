package amplitude

import of "github.com/open-feature/go-sdk/openfeature"

// Key is the type for the keys in the Amplitude User type.
type Key string

const (
	// KeyUserID is the canonical key for the user ID in the Amplitude User type.
	// The primary key for the user.
	// Automatically mapped from the targeting key.
	KeyUserID             = "user_id"
	// KeyDeviceID is the canonical key for the device ID in the Amplitude User type.
	KeyDeviceID           = "device_id"
	// KeyCountry is the canonical key for the country in the Amplitude User type.
	KeyCountry            = "country"
	// KeyRegion is the canonical key for the region in the Amplitude User type.
	KeyRegion             = "region"
	// KeyDma is the canonical key for the DMA in the Amplitude User type.
	KeyDma                = "dma"
	// KeyCity is the canonical key for the city in the Amplitude User type.
	KeyCity               = "city"
	// KeyLanguage is the canonical key for the language in the Amplitude User type.
	KeyLanguage           = "language"
	// KeyPlatform is the canonical key for the platform in the Amplitude User type.
	KeyPlatform           = "platform"
	// KeyVersion is the canonical key for the version in the Amplitude User type.
	KeyVersion            = "version"
	// KeyOs is the canonical key for the OS in the Amplitude User type.
	KeyOs                 = "os"
	// KeyDeviceManufacturer is the canonical key for the device manufacturer in the Amplitude User type.
	KeyDeviceManufacturer = "device_manufacturer"
	// KeyDeviceBrand is the canonical key for the device brand in the Amplitude User type.
	KeyDeviceBrand        = "device_brand"
	// KeyDeviceModel is the canonical key for the device model in the Amplitude User type.
	KeyDeviceModel        = "device_model"
	// KeyCarrier is the canonical key for the carrier in the Amplitude User type.
	KeyCarrier            = "carrier"
	// KeyLibrary is the canonical key for the library in the Amplitude User type.
	KeyLibrary            = "library"
	// KeyUserProperties is the canonical key for the user properties in the Amplitude User type.
	// The corresponding value is a map[string]interface{}.
	KeyUserProperties     = "user_properties"
	// KeyGroupProperties is the canonical key for the group properties in the Amplitude User type.
	// The corresponding value is a map[string]map[string]interface{}.
	KeyGroupProperties    = "group_properties"
	// KeyGroups is the canonical key for the groups in the Amplitude User type.
	// The corresponding value is a map[string][]string.
	KeyGroups             = "groups"
	// KeyCohortIDs is the canonical key for the cohort IDs in the Amplitude User type.
	// The corresponding value is a map[string]struct{}.
	KeyCohortIDs          = "cohort_ids"
	// KeyGroupCohortIDSet is the canonical key for the group cohort IDs in the Amplitude User type.
	// The corresponding value is a map[string]map[string]map[string]struct{}.
	KeyGroupCohortIDSet     = "group_cohort_ids"
)


// DefaultKeyMap is a map of string keys that might be in the evaluation context
// to the canonical key used by Amplitude.
// You can add keys to this map to automatically map the keys in the evaluation context
// to the canonical keys used by Amplitude.
// Any keys that are not mapped will be added to the [User.UserProperties] map.
// For more advanced normalization, use a hook to pre-process the evaluation context.
func DefaultKeyMap() map[string]Key {
	var keyMap = map[string]Key{}
	for k, values := range map[Key][]string {
		KeyUserID: {KeyUserID, of.TargetingKey, "userId", "user-id", "UserId", "UserID"},
		KeyDeviceID: {KeyDeviceID, "deviceId", "device-id", "DeviceId", "DeviceID"},
		KeyCountry: {KeyCountry, "country", "Country"},
		KeyRegion: {KeyRegion, "region", "Region"},
		KeyDma: {KeyDma, "dma", "Dma", "DMA"},
		KeyCity: {KeyCity, "city", "City"},
		KeyLanguage: {KeyLanguage, "language", "Language"},
		KeyPlatform: {KeyPlatform, "platform", "Platform"},
		KeyVersion: {KeyVersion, "version", "Version"},
		KeyOs: {KeyOs, "os", "Os", "OS"},
		KeyDeviceManufacturer: {KeyDeviceManufacturer, "deviceManufacturer", "device-manufacturer", "DeviceManufacturer"},
		KeyDeviceBrand: {KeyDeviceBrand, "deviceBrand", "device-brand", "DeviceBrand"},
		KeyDeviceModel: {KeyDeviceModel, "deviceModel", "device-model", "DeviceModel"},
		KeyCarrier: {KeyCarrier, "carrier", "Carrier"},
		KeyLibrary: {KeyLibrary, "library", "Library"},
		KeyUserProperties: {KeyUserProperties, "userProperties", "user-properties", "UserProperties"},
		KeyGroupProperties: {KeyGroupProperties, "groupProperties", "group-properties", "GroupProperties"},
		KeyGroups: {KeyGroups, "groups", "Groups"},
		KeyCohortIDs: {KeyCohortIDs, "cohortIds", "cohort-ids", "CohortIds", "cohortIDs"},
		KeyGroupCohortIDSet: {KeyGroupCohortIDSet, "groupCohortIds", "group-cohort-ids", "GroupCohortIds", "groupCohortIDs"},
	}{
		for _, value := range values {
			keyMap[value] = k
		}
	}
	return keyMap
}
