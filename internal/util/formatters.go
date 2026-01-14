package util

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// FormatBytes converts bytes to a human-readable string with binary prefixes.
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// DecodeBase64 decodes a base64 string, stripping any data URL prefix if present.
func DecodeBase64(data string) ([]byte, error) {
	if _, after, found := strings.Cut(data, ","); found {
		data = after
	}
	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}
