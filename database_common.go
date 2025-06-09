package main

type EntityType int

const (
	EntityArtist EntityType = iota
	EntityTrack
)

type QueryConfig struct {
	Schema       string
	Table        string
	IDColumn     string
	NameColumn   string
	ImageColumn  string
	ExtraColumns []string
}

func (e EntityType) QueryConfig(schema string) QueryConfig {
	switch e {
	case EntityArtist:
		return QueryConfig{
			Schema:      schema,
			Table:       "artist",
			IDColumn:    "artistid",
			NameColumn:  "artist",
			ImageColumn: "picture",
		}
	case EntityTrack:
		return QueryConfig{
			Schema:       schema,
			Table:        "track",
			IDColumn:     "titleid",
			NameColumn:   "tracktitle",
			ImageColumn:  "picture",
			ExtraColumns: []string{"artist"},
		}
	default:
		panic("unknown entity type")
	}
}
