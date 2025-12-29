package amplitude

import (
	"regexp"
	"strings"

	of "github.com/open-feature/go-sdk/openfeature"
)

// Key is the type for the keys in Amplitude User and Event types.
type Key string

// ==========================================================================
// User and Event shared fields
// These fields are present on both the experiment.User and analytics.Event types.
// ==========================================================================
const (
	// KeyUserID is the canonical key for the user ID.
	// The primary identifier for the user.
	// Automatically mapped from the OpenFeature targeting key.
	KeyUserID Key = "user_id"
	// KeyDeviceID is the canonical key for the device ID.
	// A device-specific identifier, useful when user ID is not available.
	KeyDeviceID Key = "device_id"
	// KeyCountry is the canonical key for the country (e.g., "United States").
	KeyCountry Key = "country"
	// KeyRegion is the canonical key for the region/state (e.g., "California").
	KeyRegion Key = "region"
	// KeyDMA is the canonical key for the Designated Market Area.
	// DMA is a geographic region used by Nielsen for TV ratings in the US.
	KeyDMA Key = "dma"
	// KeyCity is the canonical key for the city name.
	KeyCity Key = "city"
	// KeyLanguage is the canonical key for the user's language (e.g., "en-US").
	KeyLanguage Key = "language"
	// KeyPlatform is the canonical key for the platform (e.g., "iOS", "Android", "Web").
	KeyPlatform Key = "platform"
	// KeyVersion is the canonical key for the application version.
	// Note: This maps to User.Version; for events, see also KeyAppVersion and KeyVersionName.
	KeyVersion Key = "version"
	// KeyOS is the canonical key for the operating system on the User type.
	// Note: For events, use KeyOSName and KeyOSVersion instead.
	KeyOS Key = "os"
	// KeyDeviceManufacturer is the canonical key for the device manufacturer (e.g., "Apple", "Samsung").
	KeyDeviceManufacturer Key = "device_manufacturer"
	// KeyDeviceBrand is the canonical key for the device brand (e.g., "iPhone", "Galaxy").
	KeyDeviceBrand Key = "device_brand"
	// KeyDeviceModel is the canonical key for the device model (e.g., "iPhone 14 Pro").
	KeyDeviceModel Key = "device_model"
	// KeyCarrier is the canonical key for the mobile carrier (e.g., "Verizon", "AT&T").
	KeyCarrier Key = "carrier"
	// KeyLibrary is the canonical key for the SDK/library name and version used to send the event.
	KeyLibrary Key = "library"
)

// ==========================================================================
// User-only fields
// These fields are only present on the experiment.User type.
// ==========================================================================

const (
	// KeyUserProperties is the canonical key for custom user properties.
	// The corresponding value is a map[string]interface{}.
	KeyUserProperties Key = "user_properties"
	// KeyGroupProperties is the canonical key for group properties.
	// The corresponding value is a map[string]map[string]interface{}.
	KeyGroupProperties Key = "group_properties"
	// KeyGroups is the canonical key for the groups the user belongs to.
	// The corresponding value is a map[string][]string.
	KeyGroups Key = "groups"
	// KeyCohortIDs is the canonical key for the cohort IDs the user belongs to.
	// The corresponding value is a map[string]struct{}.
	KeyCohortIDs Key = "cohort_ids"
	// KeyGroupCohortIDSet is the canonical key for group cohort IDs.
	// The corresponding value is a map[string]map[string]map[string]struct{}.
	KeyGroupCohortIDSet Key = "group_cohort_ids"

	// ==========================================================================
	// Event-only fields
	// These fields are only present on the analytics.Event type (EventOptions).
	// ==========================================================================

	// KeyTime is the canonical key for the event timestamp (Unix epoch milliseconds).
	// Event-only field.
	KeyTime Key = "time"
	// KeyInsertID is the canonical key for the insert ID used for event deduplication.
	// Amplitude uses this to prevent duplicate events from being counted.
	// Event-only field.
	KeyInsertID Key = "insert_id"
	// KeyLocationLat is the canonical key for the latitude coordinate.
	// Event-only field.
	KeyLocationLat Key = "location_lat"
	// KeyLocationLng is the canonical key for the longitude coordinate.
	// Event-only field.
	KeyLocationLng Key = "location_lng"
	// KeyAppVersion is the canonical key for the application version string.
	// Event-only field.
	KeyAppVersion Key = "app_version"
	// KeyVersionName is the canonical key for the version name.
	// Event-only field.
	KeyVersionName Key = "version_name"
	// KeyOSName is the canonical key for the operating system name (e.g., "iOS", "Android").
	// Event-only field. Note: User type uses KeyOS instead.
	KeyOSName Key = "os_name"
	// KeyOSVersion is the canonical key for the operating system version (e.g., "17.0").
	// Event-only field.
	KeyOSVersion Key = "os_version"
	// KeyIDFA is the canonical key for the iOS Identifier for Advertisers.
	// A unique identifier assigned by Apple for advertising/tracking purposes.
	// Requires user opt-in via App Tracking Transparency on iOS 14.5+.
	// Event-only field.
	KeyIDFA Key = "idfa"
	// KeyIDFV is the canonical key for the iOS Identifier for Vendor.
	// A unique identifier per device+vendor combination; resets if all vendor apps are uninstalled.
	// Event-only field.
	KeyIDFV Key = "idfv"
	// KeyADID is the canonical key for the Android Advertising ID (also known as GAID).
	// Google's equivalent to Apple's IDFA for Android devices.
	// Event-only field.
	KeyADID Key = "adid"
	// KeyAndroidID is the canonical key for the Android device ID.
	// A unique identifier for Android devices.
	// Event-only field.
	KeyAndroidID Key = "android_id"
	// KeyIP is the canonical key for the IP address.
	// Amplitude can use this to derive location information.
	// Event-only field.
	KeyIP Key = "ip"
	// KeyPrice is the canonical key for the price of a product (for revenue tracking).
	// Event-only field.
	KeyPrice Key = "price"
	// KeyQuantity is the canonical key for the quantity of products purchased.
	// Event-only field.
	KeyQuantity Key = "quantity"
	// KeyRevenue is the canonical key for the revenue amount.
	// Event-only field.
	KeyRevenue Key = "revenue"
	// KeyCurrency is the canonical key for the currency code (e.g., "USD", "EUR").
	// Event-only field.
	KeyCurrency Key = "currency"
	// KeyProductID is the canonical key for the product identifier.
	// Event-only field.
	KeyProductID Key = "productId"
	// KeyRevenueType is the canonical key for the type of revenue (e.g., "purchase", "subscription").
	// Event-only field.
	// Note: this name does not follow the snake_case convention used by other keys;
	// this is correct based on the Amplitude SDK implementation.
	KeyRevenueType Key = "revenueType"
	// KeyEventID is the canonical key for the event ID.
	// Event-only field.
	KeyEventID Key = "event_id"
	// KeySessionID is the canonical key for the session ID.
	// Event-only field.
	KeySessionID Key = "session_id"
	// KeyPartnerID is the canonical key for the partner ID (for attribution).
	// Event-only field.
	KeyPartnerID Key = "partner_id"
	// KeyPlan is the canonical key for the tracking plan metadata.
	// Event-only field.
	KeyPlan Key = "plan"
	// KeyIngestionMetadata is the canonical key for ingestion metadata.
	// Event-only field.
	KeyIngestionMetadata Key = "ingestion_metadata"
	// KeyEventProperties is the canonical key for custom event properties.
	// The corresponding value is a map[string]interface{}.
	// Event-only field.
	KeyEventProperties Key = "event_properties"
	// KeyEventType is the canonical key for the event type/name.
	// Event-only field.
	KeyEventType Key = "event_type"
)

// eventKeys contains fields that are ONLY present on analytics.Event (EventOptions),
// not on experiment.User.
var eventKeys = []Key{
	KeyTime,
	KeyInsertID,
	KeyLocationLat,
	KeyLocationLng,
	KeyAppVersion,
	KeyVersionName,
	KeyOSName,
	KeyOSVersion,
	KeyIDFA,
	KeyIDFV,
	KeyADID,
	KeyAndroidID,
	KeyIP,
	KeyPrice,
	KeyQuantity,
	KeyRevenue,
	KeyCurrency,
	KeyProductID,
	KeyRevenueType,
	KeyEventID,
	KeySessionID,
	KeyPartnerID,
	KeyPlan,
	KeyIngestionMetadata,
	KeyEventProperties,
	KeyEventType,
}

// userKeys contains ALL fields present on experiment.User (including shared fields).
var userKeys = []Key{
	// Shared fields (also on EventOptions)
	KeyUserID,
	KeyDeviceID,
	KeyCountry,
	KeyRegion,
	KeyDMA,
	KeyCity,
	KeyLanguage,
	KeyPlatform,
	KeyDeviceManufacturer,
	KeyDeviceBrand,
	KeyDeviceModel,
	KeyCarrier,
	KeyLibrary,
	KeyUserProperties,
	KeyGroupProperties,
	KeyGroups,
	// User-only fields
	KeyVersion,
	KeyOS,
	KeyCohortIDs,
	KeyGroupCohortIDSet,
}

// sharedKeys contains fields that are present on BOTH experiment.User and analytics.Event.
var sharedKeys = []Key{
	KeyUserID,
	KeyDeviceID,
	KeyCountry,
	KeyRegion,
	KeyDMA,
	KeyCity,
	KeyLanguage,
	KeyPlatform,
	KeyDeviceManufacturer,
	KeyDeviceBrand,
	KeyDeviceModel,
	KeyCarrier,
	KeyLibrary,
	KeyUserProperties,
	KeyGroupProperties,
	KeyGroups,
}

var allKeys = append(append(userKeys, eventKeys...), sharedKeys...)

// DefaultKeyMap is a map of string keys that might be in the evaluation context
// to the canonical key used by Amplitude.
// You can add keys to this map to automatically map the keys in the evaluation context
// to the canonical keys used by Amplitude.
// Any keys that are not mapped will be added to the [User.UserProperties] map.
// For more advanced normalization, use a hook to pre-process the evaluation context.
func DefaultKeyMap() map[string]Key {
	var keyMap = map[string]Key{}

	// All canonical keys - permutations will be generated automatically
	// Generate permutations for each canonical key
	for _, key := range allKeys {
		for _, perm := range makePermutations(string(key)) {
			keyMap[perm] = key
		}
	}

	// Special case: OpenFeature targeting key maps to user ID
	keyMap[of.TargetingKey] = KeyUserID

	return keyMap
}

func makePermutations(value string) []string {
	// Special case for unconventional keys,
	// which we will convert to snake_case so that all the permutations will be created correctly.
	switch value {
	case string(KeyRevenueType):
		value = "revenue_type"
	}
	
	result := make([]string, 0, 11)
	result = append(result, value)
	result = append(result, strings.ToLower(value))
	result = append(result, strings.ToUpper(value))
	if strings.Contains(value, "_") {
		result = append(result, reWordBreak.ReplaceAllStringFunc(value, func(match string) string {
			return strings.ToUpper(match[1:])
		}))
		result = append(result, reWordBreak.ReplaceAllStringFunc(value, func(match string) string {
			return strings.ToLower("-" + match[1:])
		}))
	}
	withoutSuffix, hasIDSuffix := strings.CutSuffix(value, "_ids")
	if hasIDSuffix {
		result = append(result, withoutSuffix+"IDs")
		result = append(result, strings.ToLower(withoutSuffix+"-ids"))
		result = append(result, strings.ToLower(withoutSuffix+"ids"))
	}
	withoutSuffix, hasIDSuffix = strings.CutSuffix(value, "_id")
	if hasIDSuffix {
		result = append(result, withoutSuffix+"ID")
		result = append(result, strings.ToLower(withoutSuffix+"-id"))
		result = append(result, strings.ToLower(withoutSuffix+"id"))
	}

	return result
}

var reWordBreak = regexp.MustCompile(`[_^].`)
