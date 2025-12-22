// Package image provides image processing and optimization functionality.
package image

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"

	"github.com/gen2brain/jpegli"
	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// kilobyte represents the number of bytes in a kilobyte for size calculations.
const kilobyte = 1024

// encodingResult holds the result of an encoding operation.
type encodingResult struct {
	data []byte
	err  error
}

// encodeToJPEGParallel encodes an image using both standard JPEG and jpegli in parallel,
// returning the smaller result.
func encodeToJPEGParallel(img image.Image, config Config) ([]byte, string, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	standardCh := make(chan encodingResult, 1)
	jpegliCh := make(chan encodingResult, 1)

	// Encode with standard JPEG in parallel
	go func() {
		defer wg.Done()
		data, err := encodeStandardJPEG(img, config.Quality)
		standardCh <- encodingResult{data: data, err: err}
	}()

	// Encode with jpegli in parallel
	go func() {
		defer wg.Done()
		data, err := encodeWithJpegli(img, config.Quality)
		jpegliCh <- encodingResult{data: data, err: err}
	}()

	wg.Wait()
	close(standardCh)
	close(jpegliCh)

	standardResult := <-standardCh
	jpegliResult := <-jpegliCh

	// Handle standard JPEG error
	if standardResult.err != nil {
		return nil, "", &types.ImageProcessingError{Reason: fmt.Sprintf("JPEG-codering mislukt: %v", standardResult.err)}
	}

	// Determine the best result
	if jpegliResult.err == nil && len(jpegliResult.data) > 0 && len(jpegliResult.data) < len(standardResult.data) {
		// Jpegli produced smaller file
		winnerInfo := fmt.Sprintf("jpegli (%d KB) versus standaard (%d KB)", len(jpegliResult.data)/kilobyte, len(standardResult.data)/kilobyte)
		return jpegliResult.data, winnerInfo, nil
	}

	// Standard JPEG is better or Jpegli failed
	if jpegliResult.err != nil {
		winnerInfo := fmt.Sprintf("standaard (%d KB) - jpegli mislukt", len(standardResult.data)/kilobyte)
		return standardResult.data, winnerInfo, nil
	}

	winnerInfo := fmt.Sprintf("standaard (%d KB) versus jpegli (%d KB)", len(standardResult.data)/kilobyte, len(jpegliResult.data)/kilobyte)
	return standardResult.data, winnerInfo, nil
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
		return nil, &types.ImageProcessingError{Reason: fmt.Sprintf("JPEG encoding mislukt: %v", err)}
	}
	return buf.Bytes(), nil
}
