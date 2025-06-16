package main

// Scope constants - used across multiple files
const (
	ScopeArtist = "artist"
	ScopeTrack  = "track"
)

// Error message suffixes - used across multiple files
const (
	ErrSuffixFailed    = "mislukt"
	ErrSuffixNotExists = "bestaat niet"
)

// Common strings - used across multiple files
const (
	ItemTypeArtist = "artiest"
	ItemTypeTrack  = "track"
)

// Database table names - used across multiple files
const (
	tableArtist = "artist"
	tableTrack  = "track"
)

// Supported image formats - used across multiple files
var SupportedFormats = []string{"jpeg", "jpg", "png"}

// GetEntityType returns the Dutch entity type string for a given scope
func GetEntityType(scope string) string {
	if scope == ScopeTrack {
		return "Track"
	}
	return "Artiest"
}

// Error messages - centralized for consistency
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
