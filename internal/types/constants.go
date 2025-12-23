package types

import "fmt"

// EntityType identifies the type of entity (artist or track).
type EntityType string

const (
	EntityTypeArtist EntityType = "artist"
	EntityTypeTrack  EntityType = "track"
)

// Table represents a database table name.
type Table string

const (
	TableArtist Table = "artist"
	TableTrack  Table = "track"
)

const (
	LabelArtist = "artiest"
	LabelTrack  = "track"
)

// VoicetrackUserID is the UUID used in Aeron to identify voice tracks.
const VoicetrackUserID = "021F097E-B504-49BB-9B89-16B64D2E8422"

// SupportedFormats lists the image formats that can be processed.
var SupportedFormats = []string{"jpeg", "jpg", "png"}

// TableForEntityType maps an entity type to its database table.
func TableForEntityType(entityType EntityType) Table {
	if entityType == EntityTypeTrack {
		return TableTrack
	}
	return TableArtist
}

// LabelForEntityType maps an entity type to its Dutch label.
func LabelForEntityType(entityType EntityType) string {
	if entityType == EntityTypeTrack {
		return "track"
	}
	return "artiest"
}

// LabelForTable maps a table to its Dutch label.
func LabelForTable(table Table) string {
	if table == TableTrack {
		return "track"
	}
	return "artiest"
}

// IDColumnForTable maps a table to its primary key column.
func IDColumnForTable(table Table) string {
	if table == TableTrack {
		return "titleid"
	}
	return "artistid"
}

// IsValidIdentifier validates SQL identifier characters.
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

// QualifiedTable returns a validated schema.table name.
func QualifiedTable(schema string, table Table) (string, error) {
	if !IsValidIdentifier(schema) {
		return "", fmt.Errorf("ongeldige schema naam: %s", schema)
	}
	if !IsValidIdentifier(string(table)) {
		return "", fmt.Errorf("ongeldige tabel naam: %s", table)
	}
	return fmt.Sprintf("%s.%s", schema, table), nil
}
