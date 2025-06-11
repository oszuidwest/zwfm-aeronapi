package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"slices"

	"github.com/gen2brain/jpegli"
	"golang.org/x/image/draw"
)

const kilobyte = 1024

// formatSizeKB formats bytes as KB for display
func formatSizeKB(bytes int) string {
	return fmt.Sprintf("%d KB", bytes/kilobyte)
}

// getImageDimensions returns the width and height of an image
func getImageDimensions(img image.Image) (width, height int) {
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy()
}

type ImageProcessingResult struct {
	Data      []byte
	Format    string
	Encoder   string
	Original  ImageInfo
	Optimized ImageInfo
	Savings   float64
}

type ImageInfo struct {
	Format string
	Width  int
	Height int
	Size   int
}

var SupportedFormats = []string{"jpeg", "jpg", "png"}

type ImageOptimizer struct {
	Config ImageConfig
}

func NewImageOptimizer(config ImageConfig) *ImageOptimizer {
	return &ImageOptimizer{
		Config: config,
	}
}

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download mislukt: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("leesfout: %w", err)
	}

	if err := validateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}

func validateImageData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("lege afbeelding")
	}

	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ongeldige afbeelding: %w", err)
	}

	return nil
}

func validateImageFormat(format string) error {
	if !slices.Contains(SupportedFormats, format) {
		return fmt.Errorf("formaat %s niet ondersteund (gebruik: %v)", format, SupportedFormats)
	}
	return nil
}

func getImageInfo(data []byte) (format string, width, height int, err error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, err
	}
	return format, config.Width, config.Height, nil
}

func (opt *ImageOptimizer) OptimizeImage(data []byte) ([]byte, string, string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", err
	}

	switch format {
	case "jpeg", "jpg":
		return opt.optimizeJPEG(data)
	case "png":
		return opt.convertPNGToJPEG(data)
	default:
		return data, format, "origineel", nil
	}
}

func (opt *ImageOptimizer) optimizeJPEG(data []byte) ([]byte, string, string, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", fmt.Errorf("JPEG decode fout: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) convertPNGToJPEG(data []byte) ([]byte, string, string, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", fmt.Errorf("PNG decode fout: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) processImage(img image.Image, originalData []byte, outputFormat string) ([]byte, string, string, error) {
	// Resize if image exceeds target dimensions to reduce memory usage
	width, height := getImageDimensions(img)
	if width > opt.Config.TargetWidth || height > opt.Config.TargetHeight {
		img = opt.resizeImage(img, opt.Config.TargetWidth, opt.Config.TargetHeight)
	}

	optimizedData, usedEncoder, err := encodeToJPEG(img, opt.Config)
	if err != nil {
		return nil, "", "", err
	}

	// Return optimized data only if it's smaller than original
	if len(optimizedData) > 0 && len(optimizedData) < len(originalData) {
		return optimizedData, outputFormat, usedEncoder, nil
	}

	return originalData, outputFormat, "origineel", nil
}

func (opt *ImageOptimizer) resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	width, height := getImageDimensions(img)

	// Calculate scale factor to fit within bounds while maintaining aspect ratio
	scaleX := float64(maxWidth) / float64(width)
	scaleY := float64(maxHeight) / float64(height)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	// If image is already smaller, don't upscale
	if scale >= 1 {
		return img
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

	// Create destination image
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Use CatmullRom for high-quality resizing (slower but best quality)
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	return dst
}

func encodeToJPEG(img image.Image, config ImageConfig) ([]byte, string, error) {
	// Try standard JPEG encoder first
	standardData, err := encodeStandardJPEG(img, config.Quality)
	if err != nil {
		return nil, "", fmt.Errorf("JPEG encoding mislukt: %w", err)
	}

	// Try Jpegli encoder for potentially better compression
	jpegliData, jpegliErr := encodeWithJpegli(img, config.Quality)

	// Determine the best result
	if jpegliErr == nil && len(jpegliData) > 0 && len(jpegliData) < len(standardData) {
		// Jpegli produced smaller file
		winnerInfo := fmt.Sprintf("jpegli (%s) vs standaard (%s)", formatSizeKB(len(jpegliData)), formatSizeKB(len(standardData)))
		return jpegliData, winnerInfo, nil
	}

	// Standard JPEG is better or Jpegli failed
	if jpegliErr != nil {
		winnerInfo := fmt.Sprintf("standaard (%s) - jpegli faalde", formatSizeKB(len(standardData)))
		return standardData, winnerInfo, nil
	}

	winnerInfo := fmt.Sprintf("standaard (%s) vs jpegli (%s)", formatSizeKB(len(standardData)), formatSizeKB(len(jpegliData)))
	return standardData, winnerInfo, nil
}

func encodeWithJpegli(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	options := &jpegli.EncodingOptions{Quality: quality}
	if err := jpegli.Encode(&buf, img, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeStandardJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	options := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, options); err != nil {
		return nil, fmt.Errorf("JPEG encoding mislukt: %w", err)
	}
	return buf.Bytes(), nil
}

// processImage is the main entry point for image processing
func processImage(imageData []byte, config ImageConfig) (*ImageProcessingResult, error) {
	originalInfo, err := extractImageInfo(imageData)
	if err != nil {
		return nil, err
	}

	// Validate image
	if err := validateImage(originalInfo, config); err != nil {
		return nil, err
	}

	// Skip optimization if already at target size
	if shouldSkipOptimization(originalInfo, config) {
		return createSkippedResult(imageData, originalInfo), nil
	}

	return optimizeImageData(imageData, originalInfo, config)
}

func validateImage(info *ImageInfo, config ImageConfig) error {
	if err := validateImageFormat(info.Format); err != nil {
		return err
	}
	return validateImageDimensions(info, config)
}

func extractImageInfo(imageData []byte) (*ImageInfo, error) {
	format, width, height, err := getImageInfo(imageData)
	if err != nil {
		return nil, fmt.Errorf("info ophalen mislukt: %w", err)
	}

	return &ImageInfo{
		Format: format,
		Width:  width,
		Height: height,
		Size:   len(imageData),
	}, nil
}

func validateImageDimensions(info *ImageInfo, config ImageConfig) error {
	if config.RejectSmaller && (info.Width < config.TargetWidth || info.Height < config.TargetHeight) {
		return fmt.Errorf("afbeelding te klein: %dx%d (minimaal %dx%d vereist)",
			info.Width, info.Height, config.TargetWidth, config.TargetHeight)
	}
	return nil
}

func shouldSkipOptimization(info *ImageInfo, config ImageConfig) bool {
	return info.Width == config.TargetWidth && info.Height == config.TargetHeight
}

func createSkippedResult(imageData []byte, originalInfo *ImageInfo) *ImageProcessingResult {
	return &ImageProcessingResult{
		Data:      imageData,
		Format:    originalInfo.Format,
		Encoder:   "origineel (geen optimalisatie)",
		Original:  *originalInfo,
		Optimized: *originalInfo,
		Savings:   0,
	}
}

func optimizeImageData(imageData []byte, originalInfo *ImageInfo, config ImageConfig) (*ImageProcessingResult, error) {
	optimizer := NewImageOptimizer(config)
	optimizedData, optFormat, optEncoder, err := optimizer.OptimizeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("optimalisatie mislukt: %w", err)
	}

	optimizedInfo, err := extractImageInfo(optimizedData)
	if err != nil {
		optimizedInfo = &ImageInfo{
			Format: optFormat,
			Width:  originalInfo.Width,
			Height: originalInfo.Height,
			Size:   len(optimizedData),
		}
	}

	// If the "optimized" version is larger, use the original
	if len(optimizedData) >= len(imageData) {
		return &ImageProcessingResult{
			Data:      imageData,
			Format:    originalInfo.Format,
			Encoder:   "origineel (kleiner dan geoptimaliseerd)",
			Original:  *originalInfo,
			Optimized: *originalInfo,
			Savings:   0,
		}, nil
	}

	savings := calculateSavings(originalInfo.Size, optimizedInfo.Size)

	return &ImageProcessingResult{
		Data:      optimizedData,
		Format:    optFormat,
		Encoder:   optEncoder,
		Original:  *originalInfo,
		Optimized: *optimizedInfo,
		Savings:   savings,
	}, nil
}

func calculateSavings(originalSize, optimizedSize int) float64 {
	savings := originalSize - optimizedSize
	return float64(savings) / float64(originalSize) * 100
}
