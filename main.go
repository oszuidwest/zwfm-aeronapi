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
		artistName  = flag.String("artist", "", "Artiest naam om bij te werken")
		artistID    = flag.String("artistid", "", "Artiest ID om bij te werken")
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

	if *artistName == "" && *artistID == "" && !*listMode && !*nukeMode && *searchName == "" {
		showUsage()
	}

	// Configuratie laden
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	// Only show database info and connect for operations that need the database
	if *listMode || *searchName != "" || *nukeMode || *artistName != "" || *artistID != "" {
		fmt.Printf("Database: %s:%s/%s\n", config.Database.Host, config.Database.Port, config.Database.Name)

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

		// Validate that either artist name or ID is provided, but not both
		if *artistName != "" && *artistID != "" {
			log.Fatal("Kan niet zowel -artist als -artistid specificeren")
		}
		if *artistName == "" && *artistID == "" {
			log.Fatal("Moet óf -artist óf -artistid specificeren")
		}

		if err := processArtistImage(db, config, *artistName, *artistID, *imageURL, *imagePath, *dryRun); err != nil {
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

func processArtistImage(db *sql.DB, config *Config, artistName, artistID, imageURL, imagePath string, dryRun bool) error {
	// Lookup artist by name or ID
	artist, err := lookupArtist(db, config.Database.Schema, artistName, artistID)
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
		fmt.Printf("%sDRY RUN:%s Would update image for %s\n", Yellow, Reset, artist.Name)
		return nil
	}

	if err := saveImageToDatabase(db, config.Database.Schema, artist.ID, processingResult.Data); err != nil {
		return err
	}

	fmt.Printf("%s✓%s %s: %dKB → %dKB (%s)\n", Green, Reset, artist.Name, processingResult.Original.Size/1024, processingResult.Optimized.Size/1024, processingResult.Encoder)
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
	fmt.Printf("%sAeron Image Manager%s\n\n", Bold, Reset)
	fmt.Println("Gebruik:")
	fmt.Println("  ./aeron-imgman -artist=\"Name\" -url=\"image.jpg\"")
	fmt.Println("  ./aeron-imgman -artistid=\"UUID\" -file=\"/path/image.jpg\"")
	fmt.Println("  ./aeron-imgman -list")
	fmt.Println("  ./aeron-imgman -search=\"Name\"")
	fmt.Println("\nOpties:")
	fmt.Println("  -artist string     Artiest naam")
	fmt.Println("  -artistid string   Artiest ID (UUID)")
	fmt.Println("  -url string        URL van afbeelding")
	fmt.Println("  -file string       Lokaal bestand")
	fmt.Println("  -list              Toon artiesten zonder afbeelding")
	fmt.Println("  -search string     Zoek artiesten")
	fmt.Println("  -nuke              Verwijder ALLE afbeeldingen")
	fmt.Println("  -dry-run           Simuleer actie")
	fmt.Println("  -version           Toon versie")
	fmt.Println("\nVereist: config.yaml")
	os.Exit(1)
}

func showVersion() {
	fmt.Printf("%sAeron Image Manager%s v%s (%s)\n", Bold, Reset, Version, Commit)
	fmt.Println("Copyright 2025 Streekomroep ZuidWest")
}
