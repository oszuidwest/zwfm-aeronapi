package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/gen2brain/jpegli"
	"golang.org/x/image/draw"
)

// kilobyte represents the number of bytes in a kilobyte for size calculations.
const kilobyte = 1024

// ImageProcessingResult contains the complete results of image processing operations.
// It includes both the processed image data and detailed statistics about the optimization.
type ImageProcessingResult struct {
	Data      []byte    // The processed image data as bytes
	Format    string    // The output format (typically "jpeg")
	Encoder   string    // Description of which encoder was used
	Original  ImageInfo // Information about the original image
	Optimized ImageInfo // Information about the processed image
	Savings   float64   // Percentage of size reduction achieved
}

// ImageInfo contains metadata about an image including dimensions and file size.
// It is used to track image properties before and after processing.
type ImageInfo struct {
	Format string // Image format (e.g., "jpeg", "png")
	Width  int    // Image width in pixels
	Height int    // Image height in pixels
	Size   int    // File size in bytes
}

// ImageOptimizer handles image optimization operations using configurable settings.
// It provides methods to resize, compress, and optimize images for storage efficiency.
type ImageOptimizer struct {
	Config ImageConfig // Configuration settings for optimization
}

// NewImageOptimizer creates a new ImageOptimizer with the specified configuration.
// The optimizer will use the provided settings for all image processing operations.
func NewImageOptimizer(config ImageConfig) *ImageOptimizer {
	return &ImageOptimizer{
		Config: config,
	}
}

func downloadImage(urlString string) ([]byte, error) {
	return ValidateAndDownloadImage(urlString)
}

func getImageInfo(data []byte) (format string, width, height int, err error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, err
	}
	return format, config.Width, config.Height, nil
}

// OptimizeImage processes and optimizes image data according to the configured settings.
// It returns the optimized image data, format, encoder description, and any error encountered.
// The function automatically selects the best compression method between standard JPEG and jpegli.
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
		return nil, "", "", fmt.Errorf("fout bij decoderen van JPEG: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) convertPNGToJPEG(data []byte) ([]byte, string, string, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", fmt.Errorf("fout bij decoderen van PNG: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) processImage(img image.Image, originalData []byte, outputFormat string) ([]byte, string, string, error) {
	// Resize if image exceeds target dimensions to reduce memory usage
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

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

// resizeImage resizes an image to fit within the specified maximum dimensions while maintaining aspect ratio.
// It uses high-quality CatmullRom scaling and will not upscale images that are already smaller than the target size.
func (opt *ImageOptimizer) resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

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
		return nil, "", fmt.Errorf("JPEG-codering mislukt: %w", err)
	}

	// Try Jpegli encoder for potentially better compression
	jpegliData, jpegliErr := encodeWithJpegli(img, config.Quality)

	// Determine the best result
	if jpegliErr == nil && len(jpegliData) > 0 && len(jpegliData) < len(standardData) {
		// Jpegli produced smaller file
		winnerInfo := fmt.Sprintf("jpegli (%d KB) versus standaard (%d KB)", len(jpegliData)/kilobyte, len(standardData)/kilobyte)
		return jpegliData, winnerInfo, nil
	}

	// Standard JPEG is better or Jpegli failed
	if jpegliErr != nil {
		winnerInfo := fmt.Sprintf("standaard (%d KB) - jpegli mislukt", len(standardData)/kilobyte)
		return standardData, winnerInfo, nil
	}

	winnerInfo := fmt.Sprintf("standaard (%d KB) versus jpegli (%d KB)", len(standardData)/kilobyte, len(jpegliData)/kilobyte)
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
	if err := ValidateImageFormat(info.Format); err != nil {
		return err
	}
	return validateImageDimensions(info, config)
}

func extractImageInfo(imageData []byte) (*ImageInfo, error) {
	format, width, height, err := getImageInfo(imageData)
	if err != nil {
		return nil, fmt.Errorf("ophalen van afbeeldingsinformatie mislukt: %w", err)
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
		return fmt.Errorf("afbeelding is te klein: %dx%d (minimaal %dx%d vereist)",
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
		Encoder:   "origineel (geen optimalisatie nodig)",
		Original:  *originalInfo,
		Optimized: *originalInfo,
		Savings:   0,
	}
}

func optimizeImageData(imageData []byte, originalInfo *ImageInfo, config ImageConfig) (*ImageProcessingResult, error) {
	optimizer := NewImageOptimizer(config)
	optimizedData, optFormat, optEncoder, err := optimizer.OptimizeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("optimaliseren mislukt: %w", err)
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
			Encoder:   "origineel (kleiner dan geoptimaliseerde versie)",
			Original:  *originalInfo,
			Optimized: *originalInfo,
			Savings:   0,
		}, nil
	}

	savings := float64(originalInfo.Size-optimizedInfo.Size) / float64(originalInfo.Size) * 100

	return &ImageProcessingResult{
		Data:      optimizedData,
		Format:    optFormat,
		Encoder:   optEncoder,
		Original:  *originalInfo,
		Optimized: *optimizedInfo,
		Savings:   savings,
	}, nil
}
