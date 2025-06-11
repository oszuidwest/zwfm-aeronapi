# Aeron Image Manager API

Een REST API service voor het beheren van artiest- en trackafbeeldingen in het Aeron radio automatiseringssysteem.

> [!WARNING]
> Aeron is een product van Broadcast Partners. Deze tool is onofficieel en niet ontwikkeld door of in samenwerking met Broadcast Partners. Gebruik op eigen risico. Maak altijd een backup van je database voor gebruik.

## Functies

- **Afbeeldingbeheer**: Upload en beheer afbeeldingen voor artiesten en tracks via REST API
- **Automatische Optimalisatie**: Afbeeldingen worden automatisch verkleind en geoptimaliseerd
- **Dubbele Encoder**: Vergelijkt jpegli met standaard JPEG en selecteert de kleinste bestandsgrootte
- **Playlist API**: Haal playlist informatie op met flexibele query opties
- **Statistieken**: Krijg inzicht in afbeeldingdekking voor artiesten en tracks
- **Bulk Operaties**: Verwijder alle afbeeldingen voor een specifiek type
- **Authenticatie**: Optionele API key authenticatie

## Installatie

### Met Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v ./config.yaml:/app/config.yaml \
  --name aeron-imgman \
  ghcr.io/oszuidwest/aeron-imgman:latest
```

### Vanaf Broncode

```bash
git clone https://github.com/oszuidwest/aeron-imgman.git
cd aeron-imgman
go mod tidy
go build -o aeron-imgman .
./aeron-imgman -config=config.yaml -port=8080
```

### Voorgecompileerde Binaries

Download gecompileerde uitvoerbare bestanden van de [Releases pagina](https://github.com/oszuidwest/aeron-imgman/releases):

- **Linux**: amd64, arm64, armv7
- **Windows**: amd64, arm64  
- **macOS**: Intel (amd64), Apple Silicon (arm64)

## Configuratie

Maak een `config.yaml` bestand:

```yaml
database:
  host: localhost
  port: 5432
  name: aeron_db
  user: aeron
  password: jouw_wachtwoord
  schema: aeron
  sslmode: disable

image:
  target_width: 1280
  target_height: 1280
  quality: 90
  reject_smaller: true

api:
  enabled: true
  keys:
    - "jouw-api-key-hier"
    - "nog-een-api-key"
```

## API Server Starten

```bash
# Standaard poort 8080
./aeron-imgman

# Aangepaste poort
./aeron-imgman -port=9090

# Met aangepaste config
./aeron-imgman -config=/pad/naar/config.yaml -port=8080

# Toon versie
./aeron-imgman -version
```

## API Endpoints

### Health Check

```http
GET /api/health
```

Geen authenticatie vereist. Retourneert server status en versie.

### Artiesten

#### Artiest Statistieken Ophalen
```http
GET /api/artists
X-API-Key: jouw-api-key
```

Response:
```json
{
  "success": true,
  "data": {
    "total": 80,
    "with_images": 10,
    "without_images": 70,
    "orphaned": 5
  }
}
```

#### Artiest Afbeelding Uploaden
```http
POST /api/artists/upload
X-API-Key: jouw-api-key
Content-Type: application/json

{
  "name": "Artiest Naam",
  "url": "https://example.com/afbeelding.jpg"
}
```

Of met base64 afbeelding:
```json
{
  "name": "Artiest Naam",
  "image": "data:image/jpeg;base64,..."
}
```

Of met ID:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "url": "https://example.com/afbeelding.jpg"
}
```

#### Alle Artiest Afbeeldingen Verwijderen
```http
DELETE /api/artists/bulk-delete
X-API-Key: jouw-api-key
X-Confirm-Nuke: VERWIJDER ALLES
```

### Tracks

#### Track Statistieken Ophalen
```http
GET /api/tracks
X-API-Key: jouw-api-key
```

#### Track Afbeelding Uploaden
```http
POST /api/tracks/upload
X-API-Key: jouw-api-key
Content-Type: application/json

{
  "name": "Track Titel",
  "url": "https://example.com/afbeelding.jpg"
}
```

#### Alle Track Afbeeldingen Verwijderen
```http
DELETE /api/tracks/bulk-delete
X-API-Key: jouw-api-key
X-Confirm-Nuke: VERWIJDER ALLES
```

### Playlist

#### Playlist Ophalen
```http
GET /api/playlist/today
X-API-Key: jouw-api-key
```

Query parameters:
- `date`: Specifieke datum (JJJJ-MM-DD), standaard: vandaag
- `from`: Start tijd filter (UU:MM)
- `to`: Eind tijd filter (UU:MM)
- `limit`: Maximum aantal items
- `offset`: Paginering offset
- `images`: Filter op aanwezigheid afbeelding (yes/no)
- `sort`: Sorteer op veld (time/artist/track)
- `desc`: Sorteer aflopend (true/false)

Voorbeelden:
```http
# Vanmiddag playlist
GET /api/playlist/today?from=14:00&to=18:00

# Specifieke datum met paginering
GET /api/playlist/today?date=2024-01-15&limit=20&offset=40

# Alleen items zonder afbeeldingen, gesorteerd op artiest
GET /api/playlist/today?images=no&sort=artist

# Morgenochtend show
GET /api/playlist/today?date=2024-01-16&from=06:00&to=10:00
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "songid": "123e4567-e89b-12d3-a456-426614174000",
      "songname": "Counting Stars",
      "artistid": "987e6543-e21b-12d3-a456-426614174000",
      "artistname": "OneRepublic",
      "start_time": "14:35:00",
      "has_image": true
    }
  ]
}
```

## Authenticatie

Authenticatie kan geconfigureerd worden in `config.yaml`:

```yaml
api:
  enabled: true  # Schakel authenticatie in
  keys:
    - "jouw-api-key-hier"
    - "nog-een-api-key"
```

Gebruik de API key in verzoeken:
```bash
# Via header (aanbevolen)
curl -H "X-API-Key: jouw-api-key-hier" http://localhost:8080/api/artists

# Via query parameter
curl http://localhost:8080/api/artists?key=jouw-api-key-hier
```

## Response Formaat

Succes responses:
```json
{
  "success": true,
  "data": { ... }
}
```

Fout responses:
```json
{
  "success": false,
  "error": "Foutmelding"
}
```

## Afbeelding Verwerking

- **Doel afmetingen**: Configureerbaar (standaard 1280x1280)
- **Kwaliteit**: Configureerbaar (standaard 90)
- **Slimme encoding**: Vergelijkt automatisch jpegli met standaard JPEG en selecteert kleinste bestand
- **Output formaat**: Altijd JPEG ongeacht input
- **Validatie**: Kleinere afbeeldingen worden geweigerd (configureerbaar)

### Encoding Optimalisatie

De API gebruikt intelligente compressie:

1. **Dubbele encoding**: Elke afbeelding wordt gecodeerd met zowel jpegli als standaard Go JPEG encoder
2. **Automatische selectie**: De encoder die het kleinste bestand produceert wordt automatisch gekozen
3. **Rapportage**: Toont welke encoder is gebruikt en hoeveel ruimte is bespaard

Voorbeeld response:
```json
{
  "success": true,
  "data": {
    "artist": "Queen",
    "original_size": 2097152,
    "optimized_size": 191488,
    "savings_percent": 90.87,
    "encoder": "jpegli"
  }
}
```

## Database Schema

Vereist PostgreSQL tabellen zoals gebruikt door Aeron:

```sql
CREATE TABLE {schema}.artist (
    artistid UUID PRIMARY KEY,
    artist VARCHAR NOT NULL,
    picture BYTEA
);

CREATE TABLE {schema}.track (
    titleid UUID PRIMARY KEY,
    tracktitle VARCHAR NOT NULL,
    artist VARCHAR,
    artistid UUID,
    picture BYTEA,
    exporttype INTEGER
);

CREATE TABLE {schema}.playlistitem (
    id SERIAL PRIMARY KEY,
    titleid UUID,
    startdatetime TIMESTAMP,
    itemtype INTEGER
);
```

## Ontwikkeling

### Vereisten
- Go 1.24 of hoger
- PostgreSQL met Aeron database
- ImageMagick
- jpegli (optioneel, voor verbeterde compressie)

### Bouwen
```bash
go mod download
go build -o aeron-imgman .
```

### Testen
```bash
go test ./...
```

## Licentie

MIT Licentie - zie LICENSE bestand voor details.