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

// Ondersteunde formaten
var SupportedFormats = []string{"jpeg", "jpg", "png"}

type ImageOptimizer struct {
	Config ImageConfig
}

func NewImageOptimizer(config ImageConfig) *ImageOptimizer {
	return &ImageOptimizer{
		Config: config,
	}
}

func downloadImage(url string, maxSizeMB int) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kon afbeelding niet downloaden: HTTP %d", resp.StatusCode)
	}

	return readImageFromReader(resp.Body, maxSizeMB)
}

func readImageFile(path string, maxSizeMB int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readImageFromReader(file, maxSizeMB)
}

func readImageFromReader(r io.Reader, maxSizeMB int) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding data niet lezen: %w", err)
	}

	if err := validateImageSize(data, maxSizeMB); err != nil {
		return nil, err
	}

	if err := validateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}

func validateImageSize(data []byte, maxSizeMB int) error {
	maxSize := maxSizeMB * 1024 * 1024
	if len(data) > maxSize {
		return fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)", len(data), maxSize)
	}
	return nil
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
		return opt.optimizeJPEGPure(data)
	case "png":
		return opt.convertPNGToJPEG(data)
	default:
		return data, format, "origineel", nil
	}
}

func (opt *ImageOptimizer) optimizeJPEGPure(data []byte) ([]byte, string, string, error) {
	img, err := jpeg.Decode(createBytesReader(data))
	if err != nil {
		return nil, "", "", fmt.Errorf("kon JPEG niet decoderen: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) convertPNGToJPEG(data []byte) ([]byte, string, string, error) {
	img, err := png.Decode(createBytesReader(data))
	if err != nil {
		return nil, "", "", fmt.Errorf("kon PNG niet decoderen: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) processImage(img image.Image, originalData []byte, outputFormat string) ([]byte, string, string, error) {
	// Verklein als groter dan doelgrootte
	width, height := getImageDimensions(img)
	if needsResize(width, height, opt.Config.TargetWidth, opt.Config.TargetHeight) {
		img = opt.resizeImage(img, opt.Config.TargetWidth, opt.Config.TargetHeight)
	}

	// Encode JPEG
	optimizedData, usedEncoder, err := encodeToJPEG(img, opt.Config, len(originalData))
	if err != nil {
		return nil, "", "", err
	}

	// Geef geoptimaliseerde data terug als deze beter is, anders origineel
	if optimizedData != nil && len(optimizedData) > 0 {
		return optimizedData, outputFormat, usedEncoder, nil
	}

	return originalData, outputFormat, "origineel", nil
}

func (opt *ImageOptimizer) resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	width, height := getImageDimensions(img)

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

// Algemene afbeelding verwerkings functies
func createBytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}

func needsResize(width, height, targetWidth, targetHeight int) bool {
	return width > targetWidth || height > targetHeight
}

func getImageDimensions(img image.Image) (int, int) {
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy()
}

// Algemene JPEG encoding logica - retourneert data en encoder vergelijking
func encodeToJPEG(img image.Image, config ImageConfig, originalSize int) ([]byte, string, error) {
	var bestData []byte
	var bestSize int = originalSize
	var winnerInfo string

	// Probeer altijd standaard JPEG
	standardData, err := encodeWithStandardJPEG(img, config.Quality)
	if err != nil {
		return nil, "", fmt.Errorf("standaard JPEG encoding faalde: %w", err)
	}

	if len(standardData) > 0 {
		bestData = standardData
		bestSize = len(standardData)
		winnerInfo = fmt.Sprintf("standaard (%d KB)", len(standardData)/1024)
	}

	// Probeer altijd Jpegli (verplicht)
	if jpegliData, err := encodeWithJpegli(img, config.Quality); err == nil {
		if len(jpegliData) > 0 {
			if len(jpegliData) < bestSize {
				// Jpegli won
				bestData = jpegliData
				bestSize = len(jpegliData)
				winnerInfo = fmt.Sprintf("jpegli (%d KB) vs standaard (%d KB)", len(jpegliData)/1024, len(standardData)/1024)
			} else {
				// Standaard won
				winnerInfo = fmt.Sprintf("standaard (%d KB) vs jpegli (%d KB)", len(standardData)/1024, len(jpegliData)/1024)
			}
		}
	} else {
		// Jpegli faalde
		winnerInfo = fmt.Sprintf("standaard (%d KB) - jpegli faalde", len(standardData)/1024)
	}

	// Gebruik het beste resultaat (kleinste bestandsgrootte)
	if bestData != nil && len(bestData) > 0 {
		return bestData, winnerInfo, nil
	}

	return nil, "", fmt.Errorf("beide encoding methoden faalden")
}

func encodeWithJpegli(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	options := &jpegli.EncodingOptions{Quality: quality}
	if err := jpegli.Encode(&buf, img, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeWithStandardJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	options := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, options); err != nil {
		return nil, fmt.Errorf("kon JPEG niet encoderen: %w", err)
	}
	return buf.Bytes(), nil
}
