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
	MaxFileSize = 20 * 1024 * 1024 // 20MB (for download validation)
	ScopeArtist = "artist"
	ScopeTrack  = "track"
)

func main() {
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

	if *name == "" && *id == "" && !*listMode && !*nukeMode && *searchName == "" && !*serverMode {
		showUsage()
	}

	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	if *serverMode {
		fmt.Printf("Database: %s:%s/%s\n", config.Database.Host, config.Database.Port, config.Database.Name)

		db, err := sql.Open("postgres", config.DatabaseURL())
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

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

	fmt.Printf("Database: %s:%s/%s\n", config.Database.Host, config.Database.Port, config.Database.Name)

	db, err := sql.Open("postgres", config.DatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	service := NewImageService(db, config)

	switch {
	case *listMode:
		if err := handleListCommand(service, *scope); err != nil {
			log.Fatal(err)
		}

	case *searchName != "":
		if err := handleSearchCommand(service, *scope, *searchName); err != nil {
			log.Fatal(err)
		}

	case *nukeMode:
		if err := handleNukeCommand(service, *scope, *dryRun); err != nil {
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
			imageData, err := readImageFile(*imagePath, config.Image.MaxFileSizeMB)
			if err != nil {
				log.Fatal(err)
			}
			params.ImageData = imageData
		}

		if err := handleUploadCommand(service, params, *dryRun); err != nil {
			log.Fatal(err)
		}
	}
}

func handleListCommand(service *ImageService, scope string) error {
	items, err := service.ListWithoutImages(scope, 50)
	if err != nil {
		return err
	}

	if scope == ScopeArtist {
		artists := items.([]Artist)
		displayArtistList("Artiesten zonder afbeeldingen", artists, false, "max 50 getoond")
	} else {
		tracks := items.([]Track)
		displayTrackList("Tracks zonder afbeeldingen", tracks, false, "max 50 getoond")
	}
	return nil
}

func handleSearchCommand(service *ImageService, scope, searchTerm string) error {
	items, err := service.Search(scope, searchTerm)
	if err != nil {
		return err
	}

	if scope == ScopeArtist {
		artists := items.([]Artist)
		if len(artists) == 0 {
			fmt.Printf("%sGeen artiesten gevonden met '%s' in hun naam%s\n", Yellow, searchTerm, Reset)
			return nil
		}
		displayArtistList(fmt.Sprintf("Artiesten met '%s' in hun naam", searchTerm), artists, true, "")
	} else {
		tracks := items.([]Track)
		if len(tracks) == 0 {
			fmt.Printf("%sGeen tracks gevonden met '%s' in titel of artiest%s\n", Yellow, searchTerm, Reset)
			return nil
		}
		displayTrackList(fmt.Sprintf("Tracks met '%s' in titel of artiest", searchTerm), tracks, true, "")
	}
	return nil
}

func handleNukeCommand(service *ImageService, scope string, dryRun bool) error {
	result, err := service.CountImagesForNuke(scope)
	if err != nil {
		return err
	}

	if result.Count == 0 {
		fmt.Printf("Geen %s met afbeeldingen gevonden.\n", getScopeDescription(scope))
		return nil
	}

	fmt.Printf("%s%sWAARSCHUWING:%s %d %s afbeeldingen verwijderen\n\n", Bold, Red, Reset, result.Count, getScopeDescription(scope))

	previewItems, err := service.GetPreviewItems(scope, 20)
	if err != nil {
		return err
	}

	for i, item := range previewItems {
		if i < 20 {
			fmt.Printf("  • %s\n", item)
		} else if i == 20 {
			fmt.Printf("  ... en %d meer\n", result.Count-20)
			break
		}
	}

	if dryRun {
		fmt.Printf("\n%sDRY RUN:%s Zou verwijderen maar doet dit niet\n", Yellow, Reset)
		return nil
	}

	fmt.Printf("\nBevestig met '%sVERWIJDER ALLES%s': ", Red, Reset)
	var confirmation string
	fmt.Scanln(&confirmation)

	if confirmation != "VERWIJDER ALLES" {
		fmt.Println("Operatie geannuleerd.")
		return nil
	}

	nukeResult, err := service.NukeImages(scope)
	if err != nil {
		return err
	}

	fmt.Printf("%s✓%s %d %s afbeeldingen verwijderd\n", Green, Reset, nukeResult.Deleted, getScopeDescription(scope))
	return nil
}

func handleUploadCommand(service *ImageService, params *ImageUploadParams, dryRun bool) error {
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
			result.OriginalSize/1024, result.OptimizedSize/1024, result.Encoder)
	} else {
		fmt.Printf("%s✓%s %s - %s: %dKB → %dKB (%s)\n", Green, Reset, result.ItemName, result.ItemTitle,
			result.OriginalSize/1024, result.OptimizedSize/1024, result.Encoder)
	}

	return nil
}

func processAndOptimizeImage(imageData []byte, config ImageConfig) (*ImageProcessingResult, error) {
	originalInfo, err := extractImageInfo(imageData)
	if err != nil {
		return nil, err
	}

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
	fmt.Println("  ./aeron-imgman -scope=artist -list")
	fmt.Println("  ./aeron-imgman -scope=track -list")
	fmt.Println("  ./aeron-imgman -scope=artist -search=\"Name\"")
	fmt.Println("  ./aeron-imgman -scope=track -search=\"Title\"")
	fmt.Println("  ./aeron-imgman -server [-port=8080]")
	fmt.Println("\nOpties:")
	fmt.Println("  -scope string      Verplicht: 'artist' of 'track'")
	fmt.Println("  -name string       Naam van artiest of track titel")
	fmt.Println("  -id string         UUID van artiest of track")
	fmt.Println("  -url string        URL van afbeelding")
	fmt.Println("  -file string       Lokaal bestand")
	fmt.Println("  -list              Toon items zonder afbeelding")
	fmt.Println("  -search string     Zoek items")
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
