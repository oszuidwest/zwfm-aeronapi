package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// ANSI colors for clean output
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	Cyan   = "\033[36m"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Constanten
const (
	MaxFileSize = 20 * 1024 * 1024 // 20MB (for download validation)
)

func main() {
	// Remove timestamp from log output
	log.SetFlags(0)

	var (
		artistName  = flag.String("artist", "", "Artiest naam om bij te werken (vereist)")
		imageURL    = flag.String("url", "", "URL van de afbeelding om te downloaden")
		imagePath   = flag.String("file", "", "Lokaal pad naar afbeelding")
		searchName  = flag.String("search", "", "Zoek artiesten met gedeeltelijke naam match")
		listMode    = flag.Bool("list", false, "Toon a;;e artiesten zonder afbeeldingen")
		nukeMode    = flag.Bool("nuke", false, "Verwijder ALLE afbeeldingen uit de database (vereist bevestiging)")
		dryRun      = flag.Bool("dry-run", false, "Toon wat gedaan zou worden zonder daadwerkelijk bij te werken")
		versionFlag = flag.Bool("version", false, "Toon versie-informatie")
		configFile  = flag.String("config", "", "Pad naar config bestand (standaard: config.yaml)")
	)
	flag.Parse()

	if *versionFlag {
		showVersion()
		return
	}

	if *artistName == "" && !*listMode && !*nukeMode && *searchName == "" {
		showUsage()
	}

	// Configuratie laden
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	// Only show database info and connect for operations that need the database
	if *listMode || *searchName != "" || *nukeMode || *artistName != "" {
		fmt.Printf("%sDatabase:%s %s:%s/%s (schema: %s)\n", Cyan, Reset, config.Database.Host, config.Database.Port, config.Database.Name, config.Database.Schema)

		db, err := sql.Open("postgres", config.DatabaseURL())
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Fatal(err)
		}

		if *listMode {
			if err := listArtistsWithoutImages(db, config.Database.Schema); err != nil {
				log.Fatal(err)
			}
			return
		}

		if *searchName != "" {
			if err := searchArtists(db, config.Database.Schema, *searchName); err != nil {
				log.Fatal(err)
			}
			return
		}

		if *nukeMode {
			if err := nukeAllImages(db, config.Database.Schema, *dryRun); err != nil {
				log.Fatal(err)
			}
			return
		}

		// Continue to image processing for artist operations
		if err := validateImageInput(*imageURL, *imagePath); err != nil {
			log.Fatal(err)
		}

		if err := processArtistImage(db, config, *artistName, *imageURL, *imagePath, *dryRun); err != nil {
			log.Fatal(err)
		}
	}
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

func findAndValidateArtist(db *sql.DB, schema, artistName string) (*Artist, error) {
	artistID, hasExistingImage, err := findArtistByExactName(db, schema, artistName)
	if err != nil {
		return nil, err
	}

	return &Artist{
		ID:       artistID,
		Name:     artistName,
		HasImage: hasExistingImage,
	}, nil
}

func loadImageFromSource(imageURL, imagePath string, maxFileSizeMB int) ([]byte, error) {
	var imageData []byte
	var err error

	if imageURL != "" {
		imageData, err = downloadImage(imageURL, maxFileSizeMB)
		if err != nil {
			return nil, err
		}
	} else {
		imageData, err = readImageFile(imagePath, maxFileSizeMB)
		if err != nil {
			return nil, err
		}
	}

	return imageData, nil
}

func processAndOptimizeImage(imageData []byte, config ImageConfig) (*ImageProcessingResult, error) {
	originalInfo, err := extractImageInfo(imageData)
	if err != nil {
		return nil, err
	}

	// Original image info processed internally

	if err := validateImageFormat(originalInfo.Format); err != nil {
		return nil, err
	}

	if err := validateImageDimensions(originalInfo, config); err != nil {
		return nil, err
	}

	if shouldSkipOptimization(originalInfo, config) {
		return createSkippedResult(imageData, originalInfo), nil
	}

	return optimizeImageData(imageData, originalInfo, config)
}

func extractImageInfo(imageData []byte) (*ImageInfo, error) {
	format, width, height, err := getImageInfo(imageData)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding informatie niet verkrijgen: %w", err)
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
		return fmt.Errorf("afbeelding te klein: %dx%d (vereist: minimaal %dx%d)",
			info.Width, info.Height, config.TargetWidth, config.TargetHeight)
	}
	return nil
}

func shouldSkipOptimization(info *ImageInfo, config ImageConfig) bool {
	return info.Width == config.TargetWidth && info.Height == config.TargetHeight
}

func createSkippedResult(imageData []byte, originalInfo *ImageInfo) *ImageProcessingResult {
	// Image already perfect size - no processing needed
	result := &ImageProcessingResult{
		Data:      imageData,
		Format:    originalInfo.Format,
		Encoder:   "origineel (geen optimalisatie)",
		Original:  *originalInfo,
		Optimized: *originalInfo,
		Savings:   0,
	}
	// Skipped optimization - already perfect size
	return result
}

func optimizeImageData(imageData []byte, originalInfo *ImageInfo, config ImageConfig) (*ImageProcessingResult, error) {
	// Processing image for optimization

	optimizer := NewImageOptimizer(config)
	optimizedData, optFormat, optEncoder, err := optimizer.OptimizeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding niet optimaliseren: %w", err)
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

	savings := calculateSavings(originalInfo.Size, optimizedInfo.Size)

	result := &ImageProcessingResult{
		Data:      optimizedData,
		Format:    optFormat,
		Encoder:   optEncoder,
		Original:  *originalInfo,
		Optimized: *optimizedInfo,
		Savings:   savings,
	}

	// Image optimized successfully
	return result, nil
}

func calculateSavings(originalSize, optimizedSize int) float64 {
	savings := originalSize - optimizedSize
	return float64(savings) / float64(originalSize) * 100
}

func saveImageToDatabase(db *sql.DB, schema, artistID string, imageData []byte) error {
	// Updating database with new image
	if err := updateArtistImageInDB(db, schema, artistID, imageData); err != nil {
		return fmt.Errorf("kon database niet bijwerken: %w", err)
	}
	return nil
}

func processArtistImage(db *sql.DB, config *Config, artistName, imageURL, imagePath string, dryRun bool) error {
	artist, err := findAndValidateArtist(db, config.Database.Schema, artistName)
	if err != nil {
		return err
	}

	// Artist found in database

	imageData, err := loadImageFromSource(imageURL, imagePath, config.Image.MaxFileSizeMB)
	if err != nil {
		return err
	}

	processingResult, err := processAndOptimizeImage(imageData, config.Image)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("%sDRY RUN:%s Would update image for %s\n", Yellow, Reset, artistName)
		return nil
	}

	if err := saveImageToDatabase(db, config.Database.Schema, artist.ID, processingResult.Data); err != nil {
		return err
	}

	fmt.Printf("%s%s:%s %dKB → %dKB (%s) %s✓%s\n", Bold, artistName, Reset, processingResult.Original.Size/1024, processingResult.Optimized.Size/1024, processingResult.Encoder, Green, Reset)
	return nil
}

// Moved from image_utils.go
func validateImageInput(imageURL, imagePath string) error {
	if imageURL == "" && imagePath == "" {
		return fmt.Errorf("zowel -url of -file moet gespecificeerd worden")
	}
	if imageURL != "" && imagePath != "" {
		return fmt.Errorf("kan niet zowel -url als -file specificeren")
	}
	return nil
}

// Moved from output.go
func showUsage() {
	fmt.Printf("%sAeron Image Manager%s - Afbeeldingenbeheer voor Aeron databases\n\n", Bold, Reset)
	fmt.Println("Gebruik:")
	fmt.Printf("  %s./aeron-imgman -artist=\"Artist\" -url=\"image.jpg\"%s\n", Green, Reset)
	fmt.Printf("  %s./aeron-imgman -artist=\"Artist\" -file=\"/path/image.jpg\"%s\n", Green, Reset)
	fmt.Printf("  %s./aeron-imgman -list%s\n", Yellow, Reset)
	fmt.Printf("  %s./aeron-imgman -search=\"Name\"%s\n", Yellow, Reset)
	fmt.Println("\nOpties:")
	fmt.Printf("  %s-artist%s string    Artiest naam (vereist)\n", Bold, Reset)
	fmt.Printf("  %s-url%s string       URL van afbeelding\n", Bold, Reset)
	fmt.Printf("  %s-file%s string      Lokaal bestand\n", Bold, Reset)
	fmt.Printf("  %s-list%s             Toon artiesten zonder afbeelding\n", Bold, Reset)
	fmt.Printf("  %s-search%s string    Zoek artiesten\n", Bold, Reset)
	fmt.Printf("  %s-nuke%s             Verwijder ALLE afbeeldingen\n", Bold, Reset)
	fmt.Printf("  %s-dry-run%s          Simuleer actie\n", Bold, Reset)
	fmt.Printf("  %s-version%s          Toon versie\n", Bold, Reset)
	fmt.Println("\nVereist: config.yaml bestand met database en image instellingen")
	os.Exit(1)
}

func showVersion() {
	fmt.Printf("%sAeron Image Manager%s v%s (%s)\n", Bold, Reset, Version, Commit)
	fmt.Println("Copyright 2025 Streekomroep ZuidWest")
}
