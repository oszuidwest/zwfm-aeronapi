package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// ANSI color codes
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

// Constants
const (
	ScopeArtist = "artist"
	ScopeTrack  = "track"

	// Default limits
	DefaultListLimit   = 10
	DefaultNukePreview = 20

	// Size constants
	Kilobyte = 1024
)

func main() {
	log.SetFlags(0)

	var (
		scope       = flag.String("scope", "", "Verplicht: 'artist' of 'track'")
		name        = flag.String("name", "", "Naam van artiest of track titel")
		id          = flag.String("id", "", "UUID van artiest of track")
		imageURL    = flag.String("url", "", "URL van de afbeelding om te downloaden")
		imagePath   = flag.String("file", "", "Lokaal pad naar afbeelding")
		statsMode   = flag.Bool("stats", false, "Toon statistieken")
		nukeMode    = flag.Bool("nuke", false, "Verwijder ALLE afbeeldingen uit de database (vereist bevestiging)")
		dryRun      = flag.Bool("dry-run", false, "Toon wat gedaan zou worden zonder daadwerkelijk bij te werken")
		versionFlag = flag.Bool("version", false, "Toon versie-informatie")
		configFile  = flag.String("config", "", "Pad naar config bestand (standaard: config.yaml)")
		serverMode  = flag.Bool("server", false, "Start REST API server")
		serverPort  = flag.String("port", "8080", "Server poort (standaard: 8080)")
	)
	flag.Parse()

	if *versionFlag {
		showVersion()
		return
	}

	if *name == "" && *id == "" && !*statsMode && !*nukeMode && !*serverMode {
		showUsage()
	}

	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	if *serverMode {
		db, err := sql.Open("postgres", config.DatabaseURL())
		if err != nil {
			log.Fatal(err)
		}
		defer func() { _ = db.Close() }()

		if err := db.Ping(); err != nil {
			log.Fatal(err)
		}

		service := NewImageService(db, config)
		apiServer := NewAPIServer(service, config)
		log.Fatal(apiServer.Start(*serverPort))
	}

	if *scope == "" {
		log.Fatal("Moet -scope specificeren (artist of track)")
	}
	if *scope != ScopeArtist && *scope != ScopeTrack {
		log.Fatal("Ongeldige scope: moet 'artist' of 'track' zijn")
	}

	db, err := sql.Open("postgres", config.DatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	service := NewImageService(db, config)

	switch {
	case *statsMode:
		if err := handleStats(service, *scope); err != nil {
			log.Fatal(err)
		}

	case *nukeMode:
		if err := handleNuke(service, *scope, *dryRun); err != nil {
			log.Fatal(err)
		}

	case *name != "" || *id != "":
		params := &ImageUploadParams{
			Scope: *scope,
			Name:  *name,
			ID:    *id,
			URL:   *imageURL,
		}

		if *imagePath != "" {
			imageData, err := readImageFile(*imagePath)
			if err != nil {
				log.Fatal(err)
			}
			params.ImageData = imageData
		}

		if err := handleUpload(service, params, *dryRun); err != nil {
			log.Fatal(err)
		}
	}
}

func handleStats(service *ImageService, scope string) error {
	// Get statistics
	stats, err := service.GetStatistics(scope)
	if err != nil {
		return err
	}

	// Display statistics
	displayStatistics(scope, stats)

	return nil
}

func displayStatistics(scope string, stats *Statistics) {
	itemType := "Artiesten"
	if scope == ScopeTrack {
		itemType = "Tracks"
	}

	fmt.Printf("%s%s Statistieken%s\n", Bold, itemType, Reset)
	fmt.Printf("Totaal: %d\n", stats.Total)
	fmt.Printf("  %s✓%s Met afbeelding: %d\n", Green, Reset, stats.WithImages)
	fmt.Printf("  %s✗%s Zonder afbeelding: %d\n", Red, Reset, stats.WithoutImages)

	if scope == ScopeArtist {
		fmt.Printf("  %s⚠%s  Orphaned (zonder tracks): %d\n", Yellow, Reset, stats.Orphaned)
	} else {
		fmt.Printf("  %s⚠%s  Orphaned (zonder artiest): %d\n", Yellow, Reset, stats.Orphaned)
	}
}

func handleNuke(service *ImageService, scope string, dryRun bool) error {
	result, err := service.CountForNuke(scope)
	if err != nil {
		return err
	}

	if result.Count == 0 {
		fmt.Printf("Geen %s met afbeeldingen gevonden.\n", scopeDesc(scope))
		return nil
	}

	fmt.Printf("%s%sWAARSCHUWING:%s %d %s afbeeldingen verwijderen\n\n", Bold, Red, Reset, result.Count, scopeDesc(scope))

	previewItems, err := service.GetPreviewItems(scope, DefaultNukePreview)
	if err != nil {
		return err
	}

	for i, item := range previewItems {
		if i < DefaultNukePreview {
			fmt.Printf("  • %s\n", item)
		} else if i == DefaultNukePreview {
			fmt.Printf("  ... en %d meer\n", result.Count-DefaultNukePreview)
			break
		}
	}

	if dryRun {
		fmt.Printf("\n%sDRY RUN:%s Zou verwijderen maar doet dit niet\n", Yellow, Reset)
		return nil
	}

	fmt.Printf("\nBevestig met '%sVERWIJDER ALLES%s': ", Red, Reset)
	var confirmation string
	_, _ = fmt.Scanln(&confirmation)

	if confirmation != "VERWIJDER ALLES" {
		fmt.Println("Operatie geannuleerd.")
		return nil
	}

	nukeResult, err := service.NukeImages(scope)
	if err != nil {
		return err
	}

	fmt.Printf("%s✓%s %d %s afbeeldingen verwijderd\n", Green, Reset, nukeResult.Deleted, scopeDesc(scope))
	return nil
}

func handleUpload(service *ImageService, params *ImageUploadParams, dryRun bool) error {
	if dryRun {
		if params.Scope == ScopeArtist {
			_, err := lookupArtist(service.db, service.config.Database.Schema, params.Name, params.ID)
			if err != nil {
				return err
			}
			fmt.Printf("%sDRY RUN:%s Would update image for artist\n", Yellow, Reset)
		} else {
			_, err := lookupTrack(service.db, service.config.Database.Schema, params.Name, params.ID)
			if err != nil {
				return err
			}
			fmt.Printf("%sDRY RUN:%s Would update image for track\n", Yellow, Reset)
		}
		return nil
	}

	result, err := service.UploadImage(params)
	if err != nil {
		return err
	}

	if params.Scope == ScopeArtist {
		fmt.Printf("%s✓%s %s: %dKB → %dKB (%s)\n", Green, Reset, result.ItemName,
			result.OriginalSize/Kilobyte, result.OptimizedSize/Kilobyte, result.Encoder)
	} else {
		fmt.Printf("%s✓%s %s - %s: %dKB → %dKB (%s)\n", Green, Reset, result.ItemName, result.ItemTitle,
			result.OriginalSize/Kilobyte, result.OptimizedSize/Kilobyte, result.Encoder)
	}

	return nil
}

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

func showUsage() {
	fmt.Printf("%sAeron Image Manager%s\n\n", Bold, Reset)
	fmt.Println("Gebruik:")
	fmt.Println("  ./aeron-imgman -scope=artist -name=\"Name\" -url=\"image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=artist -id=\"UUID\" -file=\"/path/image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=track -name=\"Title\" -url=\"image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=track -id=\"UUID\" -file=\"/path/image.jpg\"")
	fmt.Println("  ./aeron-imgman -scope=artist -stats")
	fmt.Println("  ./aeron-imgman -scope=track -stats")
	fmt.Println("  ./aeron-imgman -server [-port=8080]")
	fmt.Println("\nOpties:")
	fmt.Println("  -scope string      Verplicht: 'artist' of 'track'")
	fmt.Println("  -name string       Naam van artiest of track titel")
	fmt.Println("  -id string         UUID van artiest of track")
	fmt.Println("  -url string        URL van afbeelding")
	fmt.Println("  -file string       Lokaal bestand")
	fmt.Println("  -stats             Toon statistieken")
	fmt.Println("  -nuke              Verwijder alle afbeeldingen van scope")
	fmt.Println("  -dry-run           Simuleer actie")
	fmt.Println("  -server            Start REST API server")
	fmt.Println("  -port string       Server poort (standaard: 8080)")
	fmt.Println("  -version           Toon versie")
	fmt.Println("  -config string     Pad naar config bestand")
	fmt.Println("\nVereist: config.yaml")
	os.Exit(1)
}

func showVersion() {
	fmt.Printf("%sAeron Image Manager%s %s (%s)\n", Bold, Reset, Version, Commit)
	fmt.Println("Copyright 2025 Streekomroep ZuidWest")
}
