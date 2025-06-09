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
	ScopeArtist = "artist"
	ScopeTrack  = "track"
)

func main() {
	// Remove timestamp from log output
	log.SetFlags(0)

	var (
		scope       = flag.String("scope", "", "Verplicht: 'artist' of 'track'")
		name        = flag.String("name", "", "Naam van artiest of track titel")
		id          = flag.String("id", "", "UUID van artiest of track")
		imageURL    = flag.String("url", "", "URL van de afbeelding om te downloaden")
		imagePath   = flag.String("file", "", "Lokaal pad naar afbeelding")
		searchName  = flag.String("search", "", "Zoek met gedeeltelijke naam match")
		listMode    = flag.Bool("list", false, "Toon alle items zonder afbeeldingen")
		nukeMode    = flag.Bool("nuke", false, "Verwijder ALLE afbeeldingen uit de database (vereist bevestiging)")
		nukeAll     = flag.Bool("nukeall", false, "Verwijder ALLE afbeeldingen (artist + track)")
		dryRun      = flag.Bool("dry-run", false, "Toon wat gedaan zou worden zonder daadwerkelijk bij te werken")
		versionFlag = flag.Bool("version", false, "Toon versie-informatie")
		configFile  = flag.String("config", "", "Pad naar config bestand (standaard: config.yaml)")
	)
	flag.Parse()

	if *versionFlag {
		showVersion()
		return
	}

	// Check if no action specified
	if *name == "" && *id == "" && !*listMode && !*nukeMode && !*nukeAll && *searchName == "" {
		showUsage()
	}

	// Validate scope for operations that need it
	needsScope := *name != "" || *id != "" || *listMode || *searchName != "" || (*nukeMode && !*nukeAll)
	if needsScope && *scope == "" {
		log.Fatal("Moet -scope specificeren (artist of track)")
	}
	if *scope != "" && *scope != ScopeArtist && *scope != ScopeTrack {
		log.Fatal("Ongeldige scope: moet 'artist' of 'track' zijn")
	}

	// Configuratie laden
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	// Only show database info and connect for operations that need the database
	fmt.Printf("Database: %s:%s/%s\n", config.Database.Host, config.Database.Port, config.Database.Name)

	db, err := sql.Open("postgres", config.DatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Handle operations
	switch {
	case *listMode:
		if err := listItemsWithoutImages(db, config.Database.Schema, *scope); err != nil {
			log.Fatal(err)
		}

	case *searchName != "":
		if err := searchItems(db, config.Database.Schema, *scope, *searchName); err != nil {
			log.Fatal(err)
		}

	case *nukeMode:
		if err := nukeAllImages(db, config.Database.Schema, *scope, *dryRun); err != nil {
			log.Fatal(err)
		}

	case *name != "" || *id != "":
		// Validate input
		if *name != "" && *id != "" {
			log.Fatal("Kan niet zowel -name als -id specificeren")
		}
		if err := validateImageInput(*imageURL, *imagePath); err != nil {
			log.Fatal(err)
		}

		// Process image
		if err := processImage(db, config, *scope, *name, *id, *imageURL, *imagePath, *dryRun); err != nil {
			log.Fatal(err)
		}
	}
}

// Unified image processing function
func processImage(db *sql.DB, config *Config, scope, name, id, imageURL, imagePath string, dryRun bool) error {
	// Load image data
	imageData, err := loadImageFromSource(imageURL, imagePath, config.Image.MaxFileSizeMB)
	if err != nil {
		return err
	}

	// Process and optimize image
	processingResult, err := processAndOptimizeImage(imageData, config.Image)
	if err != nil {
		return err
	}

	// Handle based on scope
	if scope == ScopeArtist {
		artist, err := lookupArtist(db, config.Database.Schema, name, id)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Printf("%sDRY RUN:%s Would update image for %s\n", Yellow, Reset, artist.Name)
			return nil
		}

		if err := updateArtistImageInDB(db, config.Database.Schema, artist.ID, processingResult.Data); err != nil {
			return err
		}

		fmt.Printf("%s✓%s %s: %dKB → %dKB (%s)\n", Green, Reset, artist.Name, 
			processingResult.Original.Size/1024, processingResult.Optimized.Size/1024, processingResult.Encoder)
	} else {
		track, err := lookupTrack(db, config.Database.Schema, name, id)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Printf("%sDRY RUN:%s Would update image for track: %s - %s\n", Yellow, Reset, track.Artist, track.Title)
			return nil
		}

		if err := saveTrackImageToDatabase(db, config.Database.Schema, track.ID, processingResult.Data); err != nil {
			return err
		}

		fmt.Printf("%s✓%s %s - %s: %dKB → %dKB (%s)\n", Green, Reset, track.Artist, track.Title,
			processingResult.Original.Size/1024, processingResult.Optimized.Size/1024, processingResult.Encoder)
	}

	return nil
}

// Unified list function
func listItemsWithoutImages(db *sql.DB, schema, scope string) error {
	if scope == ScopeArtist {
		return listArtistsWithoutImages(db, schema)
	}
	return listTracksWithoutImages(db, schema)
}

// Unified search function
func searchItems(db *sql.DB, schema, scope, searchTerm string) error {
	if scope == ScopeArtist {
		return searchArtists(db, schema, searchTerm)
	}
	return searchTracks(db, schema, searchTerm)
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
	fmt.Println("  ./aeron-imgman -scope=artist -name=\"Name\" -url=\"image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=artist -id=\"UUID\" -file=\"/path/image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=track -name=\"Title\" -url=\"image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=track -id=\"UUID\" -file=\"/path/image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=artist -list")
	fmt.Println("  ./aeron-imgman -scope=track -list")
	fmt.Println("  ./aeron-imgman -scope=artist -search=\"Name\"")
	fmt.Println("  ./aeron-imgman -scope=track -search=\"Title\"")
	fmt.Println("\nOpties:")
	fmt.Println("  -scope string      Verplicht: 'artist' of 'track'")
	fmt.Println("  -name string       Naam van artiest of track titel")
	fmt.Println("  -id string         UUID van artiest of track")
	fmt.Println("  -url string        URL van afbeelding")
	fmt.Println("  -file string       Lokaal bestand")
	fmt.Println("  -list              Toon items zonder afbeelding")
	fmt.Println("  -search string     Zoek items")
	fmt.Println("  -nuke              Verwijder afbeeldingen van scope")
	fmt.Println("  -nukeall           Verwijder ALLE afbeeldingen (artist + track)")
	fmt.Println("  -dry-run           Simuleer actie")
	fmt.Println("  -version           Toon versie")
	fmt.Println("  -config string     Pad naar config bestand")
	fmt.Println("\nVereist: config.yaml")
	os.Exit(1)
}

func showVersion() {
	fmt.Printf("%sAeron Image Manager%s v%s (%s)\n", Bold, Reset, Version, Commit)
	fmt.Println("Copyright 2025 Streekomroep ZuidWest")
}