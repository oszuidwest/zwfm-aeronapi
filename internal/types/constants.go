package types

import "fmt"

// EntityType is a typed string that identifies the type of entity (artist or track).
type EntityType string

// EntityType constants define the valid entity types for image operations.
// These values are used throughout the application to distinguish between
// artist and track entities.
const (
	EntityTypeArtist EntityType = "artist" // Represents artist entities
	EntityTypeTrack  EntityType = "track"  // Represents track entities
)

// Table constants for database operations.
type Table string

const (
	TableArtist Table = "artist"
	TableTrack  Table = "track"
)

// Label constants provide Dutch labels for entities in user-facing messages.
const (
	LabelArtist = "artiest" // Dutch word for "artist"
	LabelTrack  = "track"   // English word "track" (commonly used in Dutch radio)
)

// VoicetrackUserID is the UUID used in Aeron to identify voice tracks.
// This is a system-level constant in the Aeron database.
const VoicetrackUserID = "021F097E-B504-49BB-9B89-16B64D2E8422"

// SupportedFormats lists the image formats that can be processed by the application.
// All formats are converted to JPEG during optimization for consistency and size.
var SupportedFormats = []string{"jpeg", "jpg", "png"}

// TableForEntityType returns the database table name for a given entity type.
func TableForEntityType(entityType EntityType) Table {
	if entityType == EntityTypeTrack {
		return TableTrack
	}
	return TableArtist
}

// LabelForEntityType returns the Dutch label for a given entity type.
// It converts internal entity type constants to user-friendly Dutch labels.
func LabelForEntityType(entityType EntityType) string {
	if entityType == EntityTypeTrack {
		return "track"
	}
	return "artiest"
}

// LabelForTable returns the Dutch label for a given table.
func LabelForTable(table Table) string {
	if table == TableTrack {
		return "track"
	}
	return "artiest"
}

// IDColumnForTable returns the primary key column name for a given table.
func IDColumnForTable(table Table) string {
	if table == TableTrack {
		return "titleid"
	}
	return "artistid"
}

// IsValidIdentifier validates that a name contains only safe characters for SQL identifiers.
// It prevents SQL injection by allowing only alphanumeric characters and underscores.
// This is used for both schema names and table names.
func IsValidIdentifier(name string) bool {
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
	if !IsValidIdentifier(schema) {
		return "", fmt.Errorf("ongeldige schema naam: %s", schema)
	}
	if !IsValidIdentifier(string(table)) {
		return "", fmt.Errorf("ongeldige tabel naam: %s", table)
	}
	return fmt.Sprintf("%s.%s", schema, table), nil
}

// ErrorMessages provides centralized Dutch error messages for consistent user communication.
// Keys are internal error codes, values are user-friendly Dutch error messages.
//
// Format strings use standard fmt.Sprintf verbs:
//   %s - string interpolation (e.g., entity type, entity ID)
//   %d - integer interpolation (e.g., count)
//   %w - error wrapping (for errors.Is/As compatibility)
//
// Example usage:
//   msg := fmt.Sprintf(ErrorMessages["invalid_uuid"], "artiest")
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
