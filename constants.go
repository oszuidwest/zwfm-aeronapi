package main

import "fmt"

// Scope is a typed string for entity scope validation.
type Scope string

// Scope constants define the valid entity types for image operations.
// These values are used throughout the application to distinguish between
// artist and track entities.
const (
	ScopeArtist Scope = "artist" // Represents artist entities
	ScopeTrack  Scope = "track"  // Represents track entities
)

// Table constants for database operations.
type Table string

const (
	TableArtist Table = "artist"
	TableTrack  Table = "track"
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

// VoicetrackUserID is the UUID used in Aeron to identify voice tracks.
// This is a system-level constant in the Aeron database.
const VoicetrackUserID = "021F097E-B504-49BB-9B89-16B64D2E8422"

// SupportedFormats lists the image formats that can be processed by the application.
// All formats are converted to JPEG during optimization for consistency and size.
var SupportedFormats = []string{"jpeg", "jpg", "png"}

// TableForScope returns the database table name for a given scope.
func TableForScope(scope Scope) Table {
	if scope == ScopeTrack {
		return TableTrack
	}
	return TableArtist
}

// EntityTypeForScope returns the Dutch entity type string for a given scope.
// It converts internal scope constants to user-friendly Dutch labels.
func EntityTypeForScope(scope Scope) string {
	if scope == ScopeTrack {
		return "track"
	}
	return "artiest"
}

// EntityTypeForTable returns the Dutch entity type string for a given table.
func EntityTypeForTable(table Table) string {
	if table == TableTrack {
		return "track"
	}
	return "artiest"
}

// GetEntityType returns the Dutch entity type string for a given scope string.
// Kept for backwards compatibility with existing code.
func GetEntityType(scope string) string {
	if scope == string(ScopeTrack) {
		return "Track"
	}
	return "Artiest"
}

// IDColumnForTable returns the primary key column name for a given table.
func IDColumnForTable(table Table) string {
	if table == TableTrack {
		return "titleid"
	}
	return "artistid"
}

// isValidIdentifier validates that a name contains only safe characters for SQL identifiers.
// It prevents SQL injection by allowing only alphanumeric characters and underscores.
// This is used for both schema names and table names.
func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// QualifiedTable returns a fully qualified table name (schema.table) after validating
// both the schema and table names to prevent SQL injection.
// Returns an error if either name contains invalid characters.
func QualifiedTable(schema string, table Table) (string, error) {
	if !isValidIdentifier(schema) {
		return "", fmt.Errorf("ongeldige schema naam: %s", schema)
	}
	if !isValidIdentifier(string(table)) {
		return "", fmt.Errorf("ongeldige tabel naam: %s", table)
	}
	return fmt.Sprintf("%s.%s", schema, table), nil
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
