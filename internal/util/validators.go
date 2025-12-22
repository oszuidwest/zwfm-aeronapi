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

var uuidRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidateEntityID validates that an ID is a proper UUID v4 format.
func ValidateEntityID(id string, entityLabel string) error {
	if id == "" {
		return types.NewValidationError("id", fmt.Sprintf("ongeldige %s-ID: mag niet leeg zijn", entityLabel))
	}

	if !uuidRegex.MatchString(id) {
		return types.NewValidationError("id", fmt.Sprintf("ongeldige %s-ID: moet een UUID zijn", entityLabel))
	}

	return nil
}

func newSafeHTTPClient() *safeurl.WrappedClient {
	config := safeurl.GetConfigBuilder().Build()
	return safeurl.Client(config)
}

// ValidateURL validates a URL for allowed schemes and hostname presence.
func ValidateURL(urlString string) error {
	if urlString == "" {
		return types.NewValidationError("url", "lege URL")
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return types.NewValidationError("url", fmt.Sprintf("ongeldige URL: %v", err))
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return types.NewValidationError("url", "alleen HTTP en HTTPS URLs toegestaan")
	}

	if parsedURL.Host == "" {
		return types.NewValidationError("url", "geen hostname opgegeven")
	}

	return nil
}

// ValidateContentType validates that a Content-Type header indicates an image.
func ValidateContentType(contentType string) error {
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return &types.ImageProcessingError{Message: fmt.Sprintf("geen afbeelding content-type: %s", contentType)}
	}
	return nil
}

// ValidateImageData validates that byte data represents a valid image.
func ValidateImageData(data []byte) error {
	if len(data) == 0 {
		return &types.ImageProcessingError{Message: "afbeelding is leeg"}
	}

	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return &types.ImageProcessingError{Message: fmt.Sprintf("ongeldige afbeelding: %v", err)}
	}

	return nil
}

// ValidateImageFormat validates that an image format is supported.
func ValidateImageFormat(format string) error {
	if !slices.Contains(types.SupportedFormats, format) {
		return &types.ImageProcessingError{Message: fmt.Sprintf("bestandsformaat %s wordt niet ondersteund (gebruik: %v)", format, types.SupportedFormats)}
	}
	return nil
}

// ValidateAndDownloadImage validates and securely downloads an image from a URL.
func ValidateAndDownloadImage(urlString string, maxSize int64) ([]byte, error) {
	if err := ValidateURL(urlString); err != nil {
		return nil, err
	}

	client := newSafeHTTPClient()

	resp, err := client.Get(urlString)
	if err != nil {
		return nil, &types.ImageProcessingError{Message: fmt.Sprintf("downloaden mislukt: %v", err)}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Debug("Sluiten response body mislukt", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &types.ImageProcessingError{Message: fmt.Sprintf("downloaden mislukt: HTTP %d", resp.StatusCode)}
	}

	contentType := resp.Header.Get("Content-Type")
	if err := ValidateContentType(contentType); err != nil {
		return nil, err
	}

	limitedReader := io.LimitReader(resp.Body, maxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &types.ImageProcessingError{Message: fmt.Sprintf("fout bij lezen: %v", err)}
	}

	if err := ValidateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}
