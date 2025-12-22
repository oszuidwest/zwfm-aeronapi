// Package util provides utility functions for input validation and HTTP operations.
package util

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/doyensec/safeurl"
	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// uuidRegex validates UUID v4 format using a compiled regular expression.
// It ensures that UUIDs follow the proper v4 format with correct version and variant bits.
var uuidRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidateScope checks if the provided scope is valid for entity operations.
// Valid scopes are ScopeArtist and ScopeTrack. Returns an error with available options if invalid.
func ValidateScope(scope types.Scope) error {
	if scope != types.ScopeArtist && scope != types.ScopeTrack {
		return types.NewValidationError("scope", fmt.Sprintf("ongeldig type: gebruik '%s' of '%s'", types.ScopeArtist, types.ScopeTrack))
	}
	return nil
}

// ValidateEntityID validates that an ID is a proper UUID v4 format.
// It checks for empty strings and validates the UUID format using regex matching.
func ValidateEntityID(id string, entityType string) error {
	if id == "" {
		return types.NewValidationError("id", fmt.Sprintf("ongeldige %s-ID: mag niet leeg zijn", entityType))
	}

	// Validate UUID v4 format using regex
	if !uuidRegex.MatchString(id) {
		return types.NewValidationError("id", fmt.Sprintf("ongeldige %s-ID: moet een UUID zijn", entityType))
	}

	return nil
}

// ImageUploadParams contains the parameters for image upload validation.
type ImageUploadParams struct {
	Scope     types.Scope
	URL       string
	ImageData []byte
}

// ValidateImageUploadParams validates all parameters required for image upload operations.
// It ensures proper scope, mutually exclusive URL/image data, and URL safety when applicable.
func ValidateImageUploadParams(params *ImageUploadParams) error {
	if err := ValidateScope(params.Scope); err != nil {
		return err
	}

	// Check that we have either URL or image data, but not both
	hasURL := params.URL != ""
	hasImageData := len(params.ImageData) > 0

	if !hasURL && !hasImageData {
		return types.NewValidationError("image", "afbeelding is verplicht")
	}

	if hasURL && hasImageData {
		return types.NewValidationError("image", "gebruik óf URL óf upload, niet beide")
	}

	// Validate URL with SafeURL to prevent SSRF attacks
	if hasURL {
		if err := ValidateURL(params.URL); err != nil {
			return err
		}
	}

	return nil
}

// createSafeHTTPClient creates a safeurl HTTP client configured with SSRF protection.
// It uses default security settings that block private IPs, loopback addresses, and other dangerous targets.
func createSafeHTTPClient() *safeurl.WrappedClient {
	// Use default config which blocks private IPs, loopback, etc.
	config := safeurl.GetConfigBuilder().Build()

	return safeurl.Client(config)
}

// ValidateURL validates a URL by parsing it and checking for allowed schemes and hostname presence.
// Only HTTP and HTTPS schemes are permitted to prevent access to local files or other protocols.
func ValidateURL(urlString string) error {
	if urlString == "" {
		return types.NewValidationError("url", "lege URL")
	}

	// Parse URL
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return types.NewValidationError("url", fmt.Sprintf("ongeldige URL: %v", err))
	}

	// Only allow HTTP and HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return types.NewValidationError("url", "alleen HTTP en HTTPS URLs toegestaan")
	}

	// Check hostname is present
	if parsedURL.Host == "" {
		return types.NewValidationError("url", "geen hostname opgegeven")
	}

	return nil
}

// ValidateContentType validates that a Content-Type header indicates an image.
// It checks for the "image/" prefix and allows empty content types for flexibility.
func ValidateContentType(contentType string) error {
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return &types.ImageProcessingError{Reason: fmt.Sprintf("geen afbeelding content-type: %s", contentType)}
	}
	return nil
}

// ValidateImageData validates that the provided byte data represents a valid image.
// It uses Go's image.DecodeConfig to verify the data can be decoded as an image.
func ValidateImageData(data []byte) error {
	if len(data) == 0 {
		return &types.ImageProcessingError{Reason: "afbeelding is leeg"}
	}

	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return &types.ImageProcessingError{Reason: fmt.Sprintf("ongeldige afbeelding: %v", err)}
	}

	return nil
}

// ValidateImageFormat validates that an image format is supported by the application.
// Supported formats are defined in the SupportedFormats slice and include JPEG and PNG.
func ValidateImageFormat(format string) error {
	if !slices.Contains(types.SupportedFormats, format) {
		return &types.ImageProcessingError{Reason: fmt.Sprintf("bestandsformaat %s wordt niet ondersteund (gebruik: %v)", format, types.SupportedFormats)}
	}
	return nil
}

// ValidateAndDownloadImage performs comprehensive validation and secure download of an image from a URL.
// It validates the URL, downloads using SSRF protection, validates content type, and verifies image data.
// Returns the downloaded image bytes or an error if any validation step fails.
func ValidateAndDownloadImage(urlString string, maxSize int64) ([]byte, error) {
	// 1. Validate URL first
	if err := ValidateURL(urlString); err != nil {
		return nil, err
	}

	// 2. Create safe HTTP client with SSRF protection
	client := createSafeHTTPClient()

	// 3. Download image using safe client
	resp, err := client.Get(urlString)
	if err != nil {
		// safeurl returns specific errors for blocked requests
		return nil, &types.ImageProcessingError{Reason: fmt.Sprintf("downloaden mislukt: %v", err)}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Debug("Sluiten response body mislukt", "error", err)
		}
	}()

	// 4. Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, &types.ImageProcessingError{Reason: fmt.Sprintf("downloaden mislukt: HTTP %d", resp.StatusCode)}
	}

	// 5. Validate Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if err := ValidateContentType(contentType); err != nil {
		return nil, err
	}

	// 6. Read response with size limit
	limitedReader := io.LimitReader(resp.Body, maxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &types.ImageProcessingError{Reason: fmt.Sprintf("fout bij lezen: %v", err)}
	}

	// 7. Validate image data
	if err := ValidateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}
