package main

import (
	"fmt"
	"regexp"
	"strings"
)

// UUID v4 format validation regex
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidateScope checks if the provided scope is valid
func ValidateScope(scope string) error {
	if scope != ScopeArtist && scope != ScopeTrack {
		return fmt.Errorf("ongeldig type: gebruik '%s' of '%s'", ScopeArtist, ScopeTrack)
	}
	return nil
}

// ValidateEntityID validates that an ID is a proper UUID v4
func ValidateEntityID(id string, entityType string) error {
	if id == "" {
		return fmt.Errorf("ongeldige %s-ID: mag niet leeg zijn", entityType)
	}

	// Validate UUID v4 format using regex
	if !uuidRegex.MatchString(strings.ToLower(id)) {
		return fmt.Errorf("ongeldige %s-ID: moet een UUID zijn", entityType)
	}

	return nil
}

// ValidateImageUploadParams validates image upload parameters
func ValidateImageUploadParams(params *ImageUploadParams) error {
	if err := ValidateScope(params.Scope); err != nil {
		return err
	}

	// Check that we have either URL or image data, but not both
	hasURL := params.URL != ""
	hasImageData := len(params.ImageData) > 0

	if !hasURL && !hasImageData {
		return fmt.Errorf("afbeelding is verplicht")
	}

	if hasURL && hasImageData {
		return fmt.Errorf("gebruik óf URL óf upload, niet beide")
	}

	return nil
}
