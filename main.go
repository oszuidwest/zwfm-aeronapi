package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"slices"

	"github.com/gen2brain/jpegli"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Constanten
const (
	MaxFileSize     = 20 * 1024 * 1024 // 20MB
	DefaultWidth    = 640
	DefaultHeight   = 640
	DefaultQuality  = 90
	DefaultHost     = "localhost"
	DefaultPort     = "5432"
	DefaultDatabase = "aeron_db"
	DefaultUser     = "aeron_user"
	DefaultPassword = "aeron_password"
	DefaultSchema   = "aeron"
	DefaultSSLMode  = "disable"
)

// Ondersteunde formaten
var SupportedFormats = []string{"jpeg", "jpg", "png"}

// Artist vertegenwoordigt een artiest record
type Artist struct {
	ID       string
	Name     string
	HasImage bool
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Schema   string `yaml:"schema"`
	SSLMode  string `yaml:"sslmode"`
}

type ImageConfig struct {
	TargetWidth   int  `yaml:"target_width"`
	TargetHeight  int  `yaml:"target_height"`
	Quality       int  `yaml:"quality"`
	UseJpegli     bool `yaml:"use_jpegli"`
	MaxFileSizeMB int  `yaml:"max_file_size_mb"`
	RejectSmaller bool `yaml:"reject_smaller"`
}

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Image    ImageConfig    `yaml:"image"`
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.Database.User, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.Name, c.Database.SSLMode)
}

type ImageOptimizer struct {
	Config ImageConfig
}

func NewImageOptimizer(config ImageConfig) *ImageOptimizer {
	return &ImageOptimizer{
		Config: config,
	}
}

func loadConfig(configPath string) (*Config, error) {
	// Standaardconfiguratie
	config := &Config{
		Database: DatabaseConfig{
			Host:     DefaultHost,
			Port:     DefaultPort,
			Name:     DefaultDatabase,
			User:     DefaultUser,
			Password: DefaultPassword,
			Schema:   DefaultSchema,
			SSLMode:  DefaultSSLMode,
		},
		Image: ImageConfig{
			TargetWidth:   DefaultWidth,
			TargetHeight:  DefaultHeight,
			Quality:       DefaultQuality,
			UseJpegli:     true,
			MaxFileSizeMB: MaxFileSize / (1024 * 1024),
			RejectSmaller: true,
		},
	}

	// Probeer config bestand te laden
	if configPath == "" {
		// Zoek naar config.yaml in huidige directory
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else {
			// Geen config.yaml gevonden, gebruik standaardwaarden
			return config, nil
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Config bestand %s niet gevonden, gebruik standaardwaarden\n", configPath)
			return config, nil
		}
		return nil, fmt.Errorf("kon config bestand niet lezen: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("kon config bestand niet parsen: %w", err)
	}

	return config, nil
}

func main() {
	var (
		artistName  = flag.String("artist", "", "Artiest naam om bij te werken (vereist)")
		imageURL    = flag.String("url", "", "URL van de afbeelding om te downloaden")
		imagePath   = flag.String("file", "", "Lokaal pad naar afbeelding")
		listMode    = flag.Bool("list", false, "Toon a;;e artiesten zonder afbeeldingen")
		dryRun      = flag.Bool("dry-run", false, "Toon wat gedaan zou worden zonder daadwerkelijk bij te werken")
		showTools   = flag.Bool("tools", false, "Toon beschikbare optimalisatie tools")
		showVersion = flag.Bool("version", false, "Toon versie-informatie")
		configFile  = flag.String("config", "", "Pad naar config bestand (standaard: config.yaml)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Aeron Image Manager\n")
		fmt.Printf("Versie: %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Build tijd: %s\n", BuildTime)
		fmt.Printf("Een tool voor het beheer van afbeeldingen in Aeron databases\n")
		fmt.Printf("Copyright 2025 Streekomroep ZuidWest\n")
		return
	}

	if *artistName == "" && !*listMode && !*showTools {
		fmt.Println("Gebruik:")
		fmt.Println("  Artiest afbeelding bijwerken vanuit URL:")
		fmt.Println("    ./aeron-imgman -artist=\"OneRepublic\" -url=\"https://example.com/image.jpg\"")
		fmt.Println("  Artiest afbeelding bijwerken vanuit lokaal bestand:")
		fmt.Println("    ./aeron-imgman -artist=\"OneRepublic\" -file=\"/pad/naar/image.jpg\"")
		fmt.Println("  Artiesten zonder afbeeldingen tonen:")
		fmt.Println("    ./aeron-imgman -list")
		fmt.Println("  Beschikbare optimalisatie tools tonen:")
		fmt.Println("    ./aeron-imgman -tools")
		fmt.Println("  Versie informatie tonen:")
		fmt.Println("    ./aeron-imgman -version")
		fmt.Println("")
		fmt.Println("Configuratie:")
		fmt.Println("  -config=/pad/naar/config.yaml   Gebruik aangepast config bestand")
		fmt.Println("  Standaard: zoekt naar config.yaml in huidige directory")
		fmt.Println("")
		fmt.Println("  Afbeelding Vereisten (configureerbaar in config.yaml):")
		fmt.Println("  - Doelgrootte: 640x640 pixels")
		fmt.Println("  - Kleinere afbeeldingen worden geweigerd")
		fmt.Println("  - Grotere afbeeldingen worden verkleind naar doelgrootte")
		fmt.Println("  - Ondersteunde formaten: JPG, JPEG, PNG (altijd toegestaan)")
		flag.Usage()
		os.Exit(1)
	}

	// Configuratie laden
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal("Kon configuratie niet laden:", err)
	}

	if *showTools {
		showAvailableTools()
		return
	}

	fmt.Printf("Verbinden met database: %s:%s/%s (schema: %s)\n",
		config.Database.Host, config.Database.Port, config.Database.Name, config.Database.Schema)
	fmt.Printf("Afbeelding instellingen: %dx%d pixels, kwaliteit %d, weiger_kleinere: %t, verklein_grotere: waar\n",
		config.Image.TargetWidth, config.Image.TargetHeight, config.Image.Quality,
		config.Image.RejectSmaller)

	db, err := sql.Open("postgres", config.DatabaseURL())
	if err != nil {
		log.Fatal("Kon niet verbinden met database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Kon database niet bereiken:", err)
	}

	if *listMode {
		if err := listArtistsWithoutImages(db, config.Database.Schema); err != nil {
			log.Fatal("Kon artiesten niet tonen:", err)
		}
		return
	}

	if *imageURL == "" && *imagePath == "" {
		log.Fatal("Zowel -url of -file moet gespecificeerd worden")
	}

	if *imageURL != "" && *imagePath != "" {
		log.Fatal("Kan niet zowel -url als -file specificeren")
	}

	if err := processArtistImage(db, config, *artistName, *imageURL, *imagePath, *dryRun); err != nil {
		log.Fatal("Kon artiest afbeelding niet verwerken:", err)
	}
}

func processArtistImage(db *sql.DB, config *Config, artistName, imageURL, imagePath string, dryRun bool) error {
	artistID, hasExistingImage, err := findArtistByExactName(db, config.Database.Schema, artistName)
	if err != nil {
		return fmt.Errorf("kon artiest niet vinden: %w", err)
	}

	if hasExistingImage {
		fmt.Printf("Artiest gevonden: %s (ID: %s) - bestaande afbeelding wordt vervangen\n", artistName, artistID)
	} else {
		fmt.Printf("Artiest gevonden: %s (ID: %s) - geen bestaande afbeelding\n", artistName, artistID)
	}

	var imageData []byte
	if imageURL != "" {
		fmt.Printf("Afbeelding downloaden van: %s\n", imageURL)
		imageData, err = downloadImage(imageURL)
		if err != nil {
			return fmt.Errorf("kon afbeelding niet downloaden: %w", err)
		}
	} else {
		fmt.Printf("Afbeelding lezen van: %s\n", imagePath)
		imageData, err = readImageFile(imagePath)
		if err != nil {
			return fmt.Errorf("kon afbeelding bestand niet lezen: %w", err)
		}
	}

	originalFormat, originalWidth, originalHeight, err := getImageInfo(imageData)
	if err != nil {
		return fmt.Errorf("kon afbeelding informatie niet verkrijgen: %w", err)
	}

	fmt.Printf("Originele afbeelding: %s, %dx%d, %d bytes\n",
		originalFormat, originalWidth, originalHeight, len(imageData))

	// Validate image format
	if err := validateImageFormat(originalFormat); err != nil {
		return err
	}

	// Controleer afbeelding afmetingen volgens configuratie
	targetWidth := config.Image.TargetWidth
	targetHeight := config.Image.TargetHeight

	if config.Image.RejectSmaller && (originalWidth < targetWidth || originalHeight < targetHeight) {
		return fmt.Errorf("afbeelding te klein: %dx%d (vereist: minimaal %dx%d)",
			originalWidth, originalHeight, targetWidth, targetHeight)
	}

	// Verklein grotere afbeeldingen naar doelgrootte
	if originalWidth > targetWidth || originalHeight > targetHeight {
		fmt.Printf("Afbeelding wordt verkleind van %dx%d naar max %dx%d\n",
			originalWidth, originalHeight, targetWidth, targetHeight)
	} else if originalWidth == targetWidth && originalHeight == targetHeight {
		fmt.Printf("Afbeelding heeft exacte doelafmetingen: %dx%d\n", targetWidth, targetHeight)
	}

	optimizer := NewImageOptimizer(config.Image)
	optimizedData, newFormat, err := optimizer.OptimizeImage(imageData)
	if err != nil {
		return fmt.Errorf("kon afbeelding niet optimaliseren: %w", err)
	}

	originalSize := len(imageData)
	optimizedSize := len(optimizedData)
	savings := originalSize - optimizedSize
	savingsPercent := float64(savings) / float64(originalSize) * 100

	_, optimizedWidth, optimizedHeight, err := getImageInfo(optimizedData)
	if err != nil {
		optimizedWidth, optimizedHeight = originalWidth, originalHeight
	}

	fmt.Printf("Geoptimaliseerde afbeelding: %s, %dx%d, %d bytes\n", newFormat, optimizedWidth, optimizedHeight, optimizedSize)
	fmt.Printf("Grootte reductie: %d bytes (%.1f%%)\n", savings, savingsPercent)

	if optimizedWidth != originalWidth || optimizedHeight != originalHeight {
		fmt.Printf("Verkleind van %dx%d naar %dx%d (max 640x640 voor albumhoezen)\n",
			originalWidth, originalHeight, optimizedWidth, optimizedHeight)
	}

	if dryRun {
		fmt.Println("DROGE RUN: Zou artiest afbeelding bijwerken maar doet dit niet daadwerkelijk")
		return nil
	}

	fmt.Printf("Database direct bijwerken...\n")
	if err := updateArtistImageInDB(db, config.Database.Schema, artistID, optimizedData); err != nil {
		return fmt.Errorf("kon database niet bijwerken: %w", err)
	}

	fmt.Printf("Artiest afbeelding voor %s succesvol bijgewerkt in database\n", artistName)
	return nil
}

func findArtistByExactName(db *sql.DB, schema, artistName string) (string, bool, error) {
	query := fmt.Sprintf(`SELECT artistid, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
	                      FROM %s.artist WHERE artist = $1`, schema)

	var artistID string
	var hasImage bool
	err := db.QueryRow(query, artistName).Scan(&artistID, &hasImage)

	if err == sql.ErrNoRows {
		return "", false, fmt.Errorf("geen artiest gevonden met exacte naam '%s'", artistName)
	}
	if err != nil {
		return "", false, fmt.Errorf("database fout: %w", err)
	}

	return artistID, hasImage, nil
}

func updateArtistImageInDB(db *sql.DB, schema, artistID string, imageData []byte) error {
	query := fmt.Sprintf(`UPDATE %s.artist SET picture = $1 WHERE artistid = $2`, schema)
	_, err := db.Exec(query, imageData, artistID)
	if err != nil {
		return fmt.Errorf("kon artiest afbeelding niet bijwerken: %w", err)
	}
	return nil
}

func listArtistsWithoutImages(db *sql.DB, schema string) error {
	query := fmt.Sprintf(`SELECT artistid, artist FROM %s.artist 
	                      WHERE picture IS NULL 
	                      ORDER BY artist 
	                      LIMIT 50`, schema)

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("kon artiesten niet opvragen: %w", err)
	}
	defer rows.Close()

	var artists []Artist

	for rows.Next() {
		var artist Artist
		if err := rows.Scan(&artist.ID, &artist.Name); err != nil {
			return fmt.Errorf("kon artiest niet scannen: %w", err)
		}
		artists = append(artists, artist)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("fout bij doorlopen van rijen: %w", err)
	}

	fmt.Printf("Artiesten zonder afbeeldingen (%d gevonden):\n", len(artists))
	for _, artist := range artists {
		fmt.Printf("  %s (ID: %s)\n", artist.Name, artist.ID)
	}

	return nil
}

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kon afbeelding niet downloaden: HTTP %d", resp.StatusCode)
	}

	return readImageFromReader(resp.Body)
}

func readImageFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readImageFromReader(file)
}

func readImageFromReader(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding data niet lezen: %w", err)
	}

	if err := validateImageSize(data); err != nil {
		return nil, err
	}

	if err := validateImageData(data); err != nil {
		return nil, err
	}

	return data, nil
}

func validateImageSize(data []byte) error {
	if len(data) > MaxFileSize {
		return fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)", len(data), MaxFileSize)
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

// Hulp functies voor fout afhandeling
func wrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Algemene JPEG encoding logica
func encodeToJPEG(img image.Image, config ImageConfig, originalSize int) ([]byte, error) {
	var optimizedData []byte

	// Probeer Jpegli eerst
	if config.UseJpegli {
		if data, err := encodeWithJpegli(img, config.Quality); err == nil {
			if len(data) > 0 && (originalSize == 0 || len(data) < originalSize) {
				optimizedData = data
			}
		}
	}

	// Val terug naar standaard JPEG als Jpegli faalde of niet ingeschakeld is
	if optimizedData == nil {
		data, err := encodeWithStandardJPEG(img, config.Quality)
		if err != nil {
			return nil, err
		}
		if originalSize == 0 || len(data) < originalSize {
			optimizedData = data
		}
	}

	return optimizedData, nil
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

func getImageInfo(data []byte) (format string, width, height int, err error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, fmt.Errorf("kon afbeelding configuratie niet decoderen: %w", err)
	}
	return format, config.Width, config.Height, nil
}

func (opt *ImageOptimizer) OptimizeImage(data []byte) ([]byte, string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("kon afbeelding configuratie niet decoderen: %w", err)
	}

	switch format {
	case "jpeg", "jpg":
		return opt.optimizeJPEGPure(data)
	case "png":
		return opt.convertPNGToJPEG(data)
	default:
		return data, format, nil
	}
}

func (opt *ImageOptimizer) optimizeJPEGPure(data []byte) ([]byte, string, error) {
	img, err := jpeg.Decode(createBytesReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("kon JPEG niet decoderen: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

func (opt *ImageOptimizer) convertPNGToJPEG(data []byte) ([]byte, string, error) {
	img, err := png.Decode(createBytesReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("kon PNG niet decoderen: %w", err)
	}

	return opt.processImage(img, data, "jpeg")
}

// Algemene afbeeldingverwerking logica
func (opt *ImageOptimizer) processImage(img image.Image, originalData []byte, outputFormat string) ([]byte, string, error) {
	// Verklein als groter dan doelgrootte
	width, height := getImageDimensions(img)
	if needsResize(width, height, opt.Config.TargetWidth, opt.Config.TargetHeight) {
		img = opt.resizeImage(img, opt.Config.TargetWidth, opt.Config.TargetHeight)
	}

	// Encode JPEG
	optimizedData, err := encodeToJPEG(img, opt.Config, len(originalData))
	if err != nil {
		return nil, "", err
	}

	// Geef geoptimaliseerde data terug als deze beter is, anders origineel
	if optimizedData != nil && len(optimizedData) > 0 {
		return optimizedData, outputFormat, nil
	}

	return originalData, outputFormat, nil
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

func getEnvOrFlag(envKey, flagValue string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	return flagValue
}

func showAvailableTools() {
	fmt.Println("Afbeelding Optimalisatie Tools Status:")
	fmt.Println("================================")

	fmt.Println("Ingebouwde Pure Go Bibliotheken:")
	fmt.Printf("%-15s Beschikbaar - Google's nieuwste JPEG encoder (WebAssembly)\n", "Jpegli:")
	fmt.Printf("%-15s Beschikbaar - Terugval Go JPEG encoder\n", "Go JPEG:")
	fmt.Printf("%-15s Beschikbaar - PNG decoder (geconverteerd naar JPEG)\n", "Go PNG:")

	fmt.Println("\nOptimalisatie Strategie:")
	fmt.Println("- Max afmetingen: 640x640 pixels")
	fmt.Println("- Kwaliteit: 90")
	fmt.Println("- Encoder: Jpegli met terugval naar standaard Go JPEG")
	fmt.Println("- Formaat: Alle invoer geconverteerd naar JPEG")
	fmt.Println("- Ondersteund: JPG, JPEG, PNG invoer formaten")

	fmt.Println("\nGebruik:")
	fmt.Println("- ./aeron-imgman -artist=\"Artist\" -url=\"image.jpg\"")
	fmt.Println("- ./aeron-imgman -artist=\"Artist\" -url=\"image.png\"")
	fmt.Println("- ./aeron-imgman -artist=\"Artist\" -file=\"/path/to/image.jpeg\"")

	fmt.Println("\nAlle tools zijn gecompileerd in dit programma - geen externe afhankelijkheden nodig.")
}
