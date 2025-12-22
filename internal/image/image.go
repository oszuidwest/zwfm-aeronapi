// Package image provides image processing and optimization functionality.
package image

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
	"github.com/oszuidwest/zwfm-aeronapi/internal/util"
	"golang.org/x/image/draw"
)

// Config contains settings for image processing operations.
type Config struct {
	TargetWidth   int
	TargetHeight  int
	Quality       int
	RejectSmaller bool
}

// ProcessingResult contains the complete results of image processing operations.
// It includes both the processed image data and detailed statistics about the optimization.
type ProcessingResult struct {
	Data      []byte  // The processed image data as bytes
	Format    string  // The output format (typically "jpeg")
	Encoder   string  // Description of which encoder was used
	Original  Info    // Information about the original image
	Optimized Info    // Information about the processed image
	Savings   float64 // Percentage of size reduction achieved
}

// Info contains metadata about an image including dimensions and file size.
// It is used to track image properties before and after processing.
type Info struct {
	Format string // Image format (e.g., "jpeg", "png")
	Width  int    // Image width in pixels
	Height int    // Image height in pixels
	Size   int    // File size in bytes
}

// Optimizer handles image optimization operations using configurable settings.
// It provides methods to resize, compress, and optimize images for storage efficiency.
type Optimizer struct {
	Config Config // Configuration settings for optimization
}

// NewOptimizer creates a new Optimizer with the specified configuration.
// The optimizer will use the provided settings for all image processing operations.
func NewOptimizer(config Config) *Optimizer {
	return &Optimizer{
		Config: config,
	}
}

// DownloadImage downloads an image from a URL with SSRF protection.
func DownloadImage(urlString string, maxSize int64) ([]byte, error) {
	return util.ValidateAndDownloadImage(urlString, maxSize)
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
func (o *Optimizer) OptimizeImage(data []byte) ([]byte, string, string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", err
	}

	switch format {
	case "jpeg", "jpg":
		return o.optimizeJPEG(data)
	case "png":
		return o.convertPNGToJPEG(data)
	default:
		return data, format, "origineel", nil
	}
}

func (o *Optimizer) optimizeJPEG(data []byte) ([]byte, string, string, error) {
	sourceImage, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", &types.ImageProcessingError{Message: fmt.Sprintf("decoderen van JPEG mislukt: %v", err)}
	}

	return o.processImage(sourceImage, data, "jpeg")
}

func (o *Optimizer) convertPNGToJPEG(data []byte) ([]byte, string, string, error) {
	sourceImage, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", &types.ImageProcessingError{Message: fmt.Sprintf("decoderen van PNG mislukt: %v", err)}
	}

	return o.processImage(sourceImage, data, "jpeg")
}

func (o *Optimizer) processImage(sourceImage image.Image, originalData []byte, outputFormat string) ([]byte, string, string, error) {
	// Resize if image exceeds target dimensions to reduce memory usage
	bounds := sourceImage.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	if width > o.Config.TargetWidth || height > o.Config.TargetHeight {
		sourceImage = o.resizeImage(sourceImage, o.Config.TargetWidth, o.Config.TargetHeight)
	}

	// Encode to JPEG
	var jpegBuffer bytes.Buffer
	if err := jpeg.Encode(&jpegBuffer, sourceImage, &jpeg.Options{Quality: o.Config.Quality}); err != nil {
		return nil, "", "", &types.ImageProcessingError{Message: fmt.Sprintf("JPEG encoding mislukt: %v", err)}
	}
	optimizedData := jpegBuffer.Bytes()

	// Return optimized data only if it's smaller than original
	if len(optimizedData) < len(originalData) {
		return optimizedData, outputFormat, "geoptimaliseerd", nil
	}

	return originalData, outputFormat, "origineel", nil
}

// resizeImage resizes an image to fit within the specified maximum dimensions while maintaining aspect ratio.
// It uses high-quality CatmullRom scaling and will not upscale images that are already smaller than the target size.
func (o *Optimizer) resizeImage(sourceImage image.Image, maxWidth, maxHeight int) image.Image {
	bounds := sourceImage.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Calculate scale factor to fit within bounds while maintaining aspect ratio
	scaleFactorX := float64(maxWidth) / float64(width)
	scaleFactorY := float64(maxHeight) / float64(height)
	scale := scaleFactorX
	if scaleFactorY < scaleFactorX {
		scale = scaleFactorY
	}

	// If image is already smaller, don't upscale
	if scale >= 1 {
		return sourceImage
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

	// Create destination image
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Use CatmullRom for high-quality resizing (slower but best quality)
	draw.CatmullRom.Scale(dst, dst.Bounds(), sourceImage, sourceImage.Bounds(), draw.Over, nil)

	return dst
}

// Process is the main entry point for image processing.
func Process(imageData []byte, config Config) (*ProcessingResult, error) {
	originalInfo, err := extractImageInfo(imageData)
	if err != nil {
		return nil, err
	}

	// Validate image
	if err := validateImage(originalInfo, config); err != nil {
		return nil, err
	}

	// Skip optimization if already at target size
	if isAlreadyTargetSize(originalInfo, config) {
		return createSkippedResult(imageData, originalInfo), nil
	}

	return optimizeImageData(imageData, originalInfo, config)
}

func validateImage(info *Info, config Config) error {
	if err := util.ValidateImageFormat(info.Format); err != nil {
		return err
	}
	return validateImageDimensions(info, config)
}

func extractImageInfo(imageData []byte) (*Info, error) {
	format, width, height, err := getImageInfo(imageData)
	if err != nil {
		return nil, &types.ImageProcessingError{Message: fmt.Sprintf("ophalen van afbeeldingsinformatie mislukt: %v", err)}
	}

	return &Info{
		Format: format,
		Width:  width,
		Height: height,
		Size:   len(imageData),
	}, nil
}

func validateImageDimensions(info *Info, config Config) error {
	if config.RejectSmaller && (info.Width < config.TargetWidth || info.Height < config.TargetHeight) {
		return &types.ValidationError{
			Field: "dimensions",
			Message: fmt.Sprintf("afbeelding is te klein: %dx%d (minimaal %dx%d vereist)",
				info.Width, info.Height, config.TargetWidth, config.TargetHeight),
		}
	}
	return nil
}

func isAlreadyTargetSize(info *Info, config Config) bool {
	return info.Width == config.TargetWidth && info.Height == config.TargetHeight
}

func createSkippedResult(imageData []byte, originalInfo *Info) *ProcessingResult {
	return &ProcessingResult{
		Data:      imageData,
		Format:    originalInfo.Format,
		Encoder:   "origineel (geen optimalisatie nodig)",
		Original:  *originalInfo,
		Optimized: *originalInfo,
		Savings:   0,
	}
}

func optimizeImageData(imageData []byte, originalInfo *Info, config Config) (*ProcessingResult, error) {
	optimizer := NewOptimizer(config)
	optimizedData, optFormat, optEncoder, err := optimizer.OptimizeImage(imageData)
	if err != nil {
		return nil, &types.ImageProcessingError{Message: fmt.Sprintf("optimaliseren mislukt: %v", err)}
	}

	optimizedInfo, err := extractImageInfo(optimizedData)
	if err != nil {
		optimizedInfo = &Info{
			Format: optFormat,
			Width:  originalInfo.Width,
			Height: originalInfo.Height,
			Size:   len(optimizedData),
		}
	}

	// If the "optimized" version is larger, use the original
	if len(optimizedData) >= len(imageData) {
		return &ProcessingResult{
			Data:      imageData,
			Format:    originalInfo.Format,
			Encoder:   "origineel (kleiner dan geoptimaliseerde versie)",
			Original:  *originalInfo,
			Optimized: *originalInfo,
			Savings:   0,
		}, nil
	}

	savings := float64(originalInfo.Size-optimizedInfo.Size) / float64(originalInfo.Size) * 100

	return &ProcessingResult{
		Data:      optimizedData,
		Format:    optFormat,
		Encoder:   optEncoder,
		Original:  *originalInfo,
		Optimized: *optimizedInfo,
		Savings:   savings,
	}, nil
}
