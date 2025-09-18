package main

// Scope constants define the valid entity types for image operations.
// These values are used throughout the application to distinguish between
// artist and track entities.
const (
	ScopeArtist = "artist" // Represents artist entities
	ScopeTrack  = "track"  // Represents track entities
)

// Error message suffixes provide consistent Dutch error messaging.
// These are appended to entity-specific error messages throughout the application.
const (
	ErrSuffixFailed    = "mislukt"      // "failed" - used for operation failures
	ErrSuffixNotExists = "bestaat niet" // "does not exist" - used for missing entities
)

// Item type constants provide Dutch labels for entities.
// These are used in user-facing messages and API responses.
const (
	ItemTypeArtist = "artiest" // Dutch word for "artist"
	ItemTypeTrack  = "track"   // English word "track" (commonly used in Dutch radio)
)

// Database table name constants ensure consistent table references.
// These match the actual table names in the Aeron PostgreSQL schema.
const (
	tableArtist = "artist" // Artist table name
	tableTrack  = "track"  // Track table name
)

// SupportedFormats lists the image formats that can be processed by the application.
// All formats are converted to JPEG during optimization for consistency and size.
var SupportedFormats = []string{"jpeg", "jpg", "png"}

// GetEntityType returns the Dutch entity type string for a given scope.
// It converts internal scope constants to user-friendly Dutch labels.
func GetEntityType(scope string) string {
	if scope == ScopeTrack {
		return "Track"
	}
	return "Artiest"
}

// ErrorMessages provides centralized Dutch error messages for consistent user communication.
// Keys are internal error codes, values are user-friendly Dutch error messages.
// Use fmt.Sprintf with these messages when formatting is needed.
var ErrorMessages = map[string]string{
	// Authentication errors
	"auth_failed":     "Niet geautoriseerd: ongeldige of ontbrekende API-sleutel",
	"missing_confirm": "Ontbrekende bevestigingsheader: %s",

	// Validation errors
	"invalid_request": "Ongeldige aanvraaginhoud",
	"invalid_base64":  "Ongeldige base64-afbeelding",
	"invalid_uuid":    "Ongeldige %s-ID: moet een UUID zijn",
	"image_required":  "afbeelding is verplicht",
	"either_or":       "gebruik óf URL óf upload, niet beide",

	// Entity errors
	"entity_not_found": "%s-ID '%s' bestaat niet",
	"no_image":         "%s heeft geen afbeelding",

	// Database errors
	"db_error":      "databasefout: %w",
	"update_failed": "bijwerken van %s mislukt: %w",
	"delete_failed": "verwijderen van %s mislukt: %w",
	"count_failed":  "tellen van %s mislukt: %w",

	// Success messages
	"image_deleted": "%s-afbeelding succesvol verwijderd",
	"bulk_deleted":  "%d %s-afbeeldingen verwijderd",
}
