package main

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
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

	// Validate URL with SafeURL to prevent SSRF attacks
	if hasURL {
		if err := ValidateURL(params.URL); err != nil {
			return err
		}
	}

	return nil
}

// createHTTPClient creates a standard HTTP client with timeout
func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// ValidateURL validates a URL by parsing and checking scheme
func ValidateURL(urlString string) error {
	if urlString == "" {
		return fmt.Errorf("lege URL")
	}

	// Parse URL
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("ongeldige URL: %w", err)
	}

	// Only allow HTTP and HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("alleen HTTP en HTTPS URLs toegestaan")
	}

	// Check hostname is present
	if parsedURL.Host == "" {
		return fmt.Errorf("geen hostname opgegeven")
	}

	return nil
}

// ValidateContentType validates that a content type is an image type
func ValidateContentType(contentType string) error {
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("geen afbeelding content-type: %s", contentType)
	}
	return nil
}

// ValidateImageData validates that data represents a valid image
func ValidateImageData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("afbeelding is leeg")
	}

	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ongeldige afbeelding: %w", err)
	}

	return nil
}

// ValidateImageFormat validates that a format is supported
func ValidateImageFormat(format string) error {
	if !slices.Contains(SupportedFormats, format) {
		return fmt.Errorf("bestandsformaat %s wordt niet ondersteund (gebruik: %v)", format, SupportedFormats)
	}
	return nil
}

// ValidateAndDownloadImage validates URL and downloads image with all necessary checks
func ValidateAndDownloadImage(urlString string) ([]byte, error) {
	// 1. Validate URL first
	if err := ValidateURL(urlString); err != nil {
		return nil, err
	}

	// 2. Download image using standard HTTP client
	resp, err := createHTTPClient().Get(urlString)
	if err != nil {
		return nil, fmt.Errorf("downloaden mislukt: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 3. Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloaden mislukt: HTTP %d", resp.StatusCode)
	}

	// 4. Validate Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if err := ValidateContentType(contentType); err != nil {
		return nil, err
	}

	// 5. Read response with size limit (50MB)
	const maxSize = 50 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("fout bij lezen: %w", err)
	}

	// 6. Validate image data
	if err := ValidateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}
