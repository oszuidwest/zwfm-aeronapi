package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"slices"

	"github.com/gen2brain/jpegli"
)

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
		return nil, fmt.Errorf("kon afbeelding niet downloaden: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding data niet lezen: %w", err)
	}

	if err := validateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}

func readImageFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := validateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}

func validateImageData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("lege afbeelding data")
	}

	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ongeldige afbeelding data: %w", err)
	}

	return nil
}

func validateImageFormat(format string) error {
	if !slices.Contains(SupportedFormats, format) {
		return fmt.Errorf("niet ondersteund afbeelding formaat: %s (ondersteund: %v)", format, SupportedFormats)
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
		return nil, "", "", fmt.Errorf("kon JPEG niet decoderen: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) convertPNGToJPEG(data []byte) ([]byte, string, string, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", fmt.Errorf("kon PNG niet decoderen: %w", err)
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

	// Return optimized data if it's better, otherwise return original
	if len(optimizedData) > 0 {
		return optimizedData, outputFormat, usedEncoder, nil
	}

	return originalData, outputFormat, "origineel", nil
}

func (opt *ImageOptimizer) resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	scaleX := float64(maxWidth) / float64(width)
	scaleY := float64(maxHeight) / float64(height)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	if scale >= 1 {
		return img
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}

	return dst
}

func encodeToJPEG(img image.Image, config ImageConfig) ([]byte, string, error) {
	// Try standard JPEG encoder first
	standardData, err := encodeStandardJPEG(img, config.Quality)
	if err != nil {
		return nil, "", fmt.Errorf("standaard JPEG encoding faalde: %w", err)
	}

	// Try Jpegli encoder for potentially better compression
	jpegliData, jpegliErr := encodeWithJpegli(img, config.Quality)

	// Determine the best result
	if jpegliErr == nil && len(jpegliData) > 0 && len(jpegliData) < len(standardData) {
		// Jpegli produced smaller file
		winnerInfo := fmt.Sprintf("jpegli (%d KB) vs standaard (%d KB)", len(jpegliData)/Kilobyte, len(standardData)/Kilobyte)
		return jpegliData, winnerInfo, nil
	}

	// Standard JPEG is better or Jpegli failed
	if jpegliErr != nil {
		winnerInfo := fmt.Sprintf("standaard (%d KB) - jpegli faalde", len(standardData)/Kilobyte)
		return standardData, winnerInfo, nil
	}

	winnerInfo := fmt.Sprintf("standaard (%d KB) vs jpegli (%d KB)", len(standardData)/Kilobyte, len(jpegliData)/Kilobyte)
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
		return nil, fmt.Errorf("kon JPEG niet encoderen: %w", err)
	}
	return buf.Bytes(), nil
}
