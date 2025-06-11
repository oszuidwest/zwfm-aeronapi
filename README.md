# Aeron Image Manager API

Een REST API-service voor het beheren van artiest- en trackafbeeldingen in het Aeron radio-automatiseringssysteem.

> [!WARNING]
> Aeron is een product van Broadcast Partners. Deze tool is onofficieel en niet ontwikkeld door of in samenwerking met Broadcast Partners. Gebruik op eigen risico. Maak altijd een back-up van je database voordat je de tool gebruikt.

## Functionaliteiten

- **Afbeeldingbeheer**: Upload en beheer afbeeldingen voor artiesten en tracks via REST API
- **Automatische optimalisatie**: Afbeeldingen worden automatisch verkleind en geoptimaliseerd
- **Dubbele encoder**: Vergelijkt jpegli met standaard JPEG en selecteert de kleinste bestandsgrootte
- **Playlist API**: Haal playlistinformatie op met flexibele query-opties
- **Statistieken**: Krijg inzicht in afbeeldingdekking voor artiesten en tracks
- **Bulkoperaties**: Verwijder alle afbeeldingen voor een specifiek type
- **Authenticatie**: Optionele API key-authenticatie

## Installatie

### Met Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v ./config.yaml:/app/config.yaml \
  --name aeron-imgman \
  ghcr.io/oszuidwest/aeron-imgman:latest
```

### Vanaf broncode

```bash
git clone https://github.com/oszuidwest/aeron-imgman.git
cd aeron-imgman
go mod tidy
go build -o aeron-imgman .
./aeron-imgman -config=config.yaml -port=8080
```

### Voorgecompileerde binaries

Download gecompileerde uitvoerbare bestanden van de [Releases-pagina](https://github.com/oszuidwest/aeron-imgman/releases):

- **Linux**: amd64, arm64, armv7
- **Windows**: amd64, arm64  
- **macOS**: Intel (amd64), Apple Silicon (arm64)

## Configuratie

Maak een `config.yaml`-bestand:

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

## API-server starten

```bash
# Standaardpoort 8080
./aeron-imgman

# Aangepaste poort
./aeron-imgman -port=9090

# Met aangepaste configuratie
./aeron-imgman -config=/pad/naar/config.yaml -port=8080

# Toon versie
./aeron-imgman -version
```

## API-endpoints

### Health check

```http
GET /api/health
```

Geen authenticatie vereist. Retourneert serverstatus en versie.

### Artiesten

#### Artieststatistieken ophalen
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
    "without_images": 70
  }
}
```

#### Individuele artiest ophalen
```http
GET /api/artists/{artistid}
X-API-Key: jouw-api-key
```

Response:
```json
{
  "success": true,
  "data": {
    "artistid": "123e4567-e89b-12d3-a456-426614174000",
    "artist": "Queen",
    "info": "British rock band formed in London in 1970",
    "website": "https://www.queenонline.com",
    "twitter": "@QueenWillRock",
    "instagram": "@officialqueen",
    "has_image": true,
    "repeat_value": 5
  }
}
```

#### Artiestafbeelding beheren
Het endpoint `/api/artists/{artistid}/image` ondersteunt ophalen (GET), uploaden (POST) en verwijderen (DELETE) van afbeeldingen.

##### Afbeelding ophalen
```http
GET /api/artists/{artistid}/image
X-API-Key: jouw-api-key
```

Returns:
- **Content-Type**: `image/jpeg` or `image/png` (detected automatically)
- **Status**: 200 (OK) with binary image data
- **Status**: 404 (Not Found) if artist doesn't exist or has no image

##### Afbeelding uploaden
```http
POST /api/artists/{artistid}/image
X-API-Key: jouw-api-key
Content-Type: application/json

{
  "url": "https://example.com/afbeelding.jpg"
}
```

Of met base64-afbeelding:
```json
{
  "image": "data:image/jpeg;base64,..."
}
```

##### Afbeelding verwijderen
```http
DELETE /api/artists/{artistid}/image
X-API-Key: jouw-api-key
```

Response:
```json
{
  "success": true,
  "data": {
    "message": "Artiest afbeelding succesvol verwijderd",
    "artist_id": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

**Ondersteunde formaten**: JPEG (.jpg, .jpeg) en PNG (.png)

**Note**: 
- De artiest ID wordt uit de URL gehaald. Name-based lookup wordt niet meer ondersteund op dit endpoint.
- Het oude `/api/artists/upload` endpoint is vervangen door dit geharmoniseerde endpoint.
- Andere formaten (WEBP, GIF, BMP, etc.) worden geweigerd bij upload

#### Alle artiestafbeeldingen verwijderen
```http
DELETE /api/artists/bulk-delete
X-API-Key: jouw-api-key
X-Confirm-Bulk-Delete: VERWIJDER ALLES
```

### Tracks

#### Trackstatistieken ophalen
```http
GET /api/tracks
X-API-Key: jouw-api-key
```

#### Individuele track ophalen
```http
GET /api/tracks/{trackid}
X-API-Key: jouw-api-key
```

Response:
```json
{
  "success": true,
  "data": {
    "titleid": "123e4567-e89b-12d3-a456-426614174000",
    "tracktitle": "Bohemian Rhapsody",
    "artist": "Queen",
    "artistid": "987e6543-e21b-12d3-a456-426614174000",
    "year": "1975",
    "knownlength": 354000,
    "introtime": 5000,
    "outrotime": 10000,
    "tempo": 120,
    "bpm": 76,
    "gender": 1,
    "language": 1,
    "mood": 3,
    "exporttype": 1,
    "repeat_value": 5,
    "rating": 9,
    "has_image": true,
    "website": "https://www.queen.com",
    "conductor": "",
    "orchestra": ""
  }
}
```

#### Trackafbeelding beheren

##### Afbeelding ophalen
```http
GET /api/tracks/{trackid}/image
X-API-Key: jouw-api-key
```

Returns:
- **Content-Type**: `image/jpeg` or `image/png` (detected automatically)
- **Status**: 200 (OK) with binary image data
- **Status**: 404 (Not Found) if track doesn't exist or has no image

##### Afbeelding verwijderen
```http
DELETE /api/tracks/{trackid}/image
X-API-Key: jouw-api-key
```

Response:
```json
{
  "success": true,
  "data": {
    "message": "Track afbeelding succesvol verwijderd",
    "track_id": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

#### Trackafbeelding uploaden
```http
POST /api/tracks/upload
X-API-Key: jouw-api-key
Content-Type: application/json

{
  "name": "Tracktitel",
  "url": "https://example.com/afbeelding.jpg"
}
```

**Ondersteunde formaten**: JPEG (.jpg, .jpeg) en PNG (.png)

#### Alle trackafbeeldingen verwijderen
```http
DELETE /api/tracks/bulk-delete
X-API-Key: jouw-api-key
X-Confirm-Bulk-Delete: VERWIJDER ALLES
```

### Playlist

#### Playlist ophalen
```http
GET /api/playlist
X-API-Key: jouw-api-key
```

Query-parameters:
- `date`: Specifieke datum (JJJJ-MM-DD), standaard: vandaag
- `from`: Starttijdfilter (UU:MM)
- `to`: Eindtijdfilter (UU:MM)
- `limit`: Maximum aantal items
- `offset`: Pagineringsoffset
- `track_image`: Filter op trackafbeelding (yes/no)
- `artist_image`: Filter op artiestafbeelding (yes/no)
- `sort`: Sorteer op veld (time/artist/track)
- `desc`: Sorteer aflopend (true/false)

Voorbeelden:
```http
# Vandaag middagplaylist
GET /api/playlist?from=14:00&to=18:00

# Specifieke datum met paginering
GET /api/playlist?date=2024-01-15&limit=20&offset=40

# Alleen items zonder trackafbeeldingen, gesorteerd op artiest
GET /api/playlist?track_image=no&sort=artist

# Items met artiestafbeelding maar zonder trackafbeelding
GET /api/playlist?artist_image=yes&track_image=no

# Ochtendshow morgen
GET /api/playlist?date=2024-01-16&from=06:00&to=10:00
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
      "end_time": "14:38:30",
      "duration": 210000,
      "has_track_image": true,
      "has_artist_image": false
    }
  ]
}
```

## Authenticatie

Authenticatie kan worden geconfigureerd in `config.yaml`:

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

# Via query-parameter
curl http://localhost:8080/api/artists?key=jouw-api-key-hier
```

## Response-formaat

Succesvolle responses:
```json
{
  "success": true,
  "data": { ... }
}
```

Foutresponses:
```json
{
  "success": false,
  "error": "Foutmelding"
}
```

## Afbeeldingverwerking

- **Ondersteunde formaten**: JPEG (.jpg, .jpeg) en PNG (.png) - andere formaten worden geweigerd
- **Doelafmetingen**: Configureerbaar (standaard 1280×1280)
- **Kwaliteit**: Configureerbaar (standaard 90)
- **Slimme verwerking**:
  - **Perfect sized**: Origineel formaat behouden (PNG blijft PNG, JPEG blijft JPEG)
  - **Needs resize**: Altijd geconverteerd naar JPEG met jpegli/standaard optimalisatie
- **Validatie**: Kleinere afbeeldingen worden geweigerd (configureerbaar)

### Encoding-optimalisatie

De API gebruikt intelligente compressie voor afbeeldingen die moeten worden verkleind:

1. **Dubbele encoding**: Afbeeldingen die geresized worden, worden gecodeerd met zowel jpegli als standaard Go JPEG-encoder
2. **Automatische selectie**: De encoder die het kleinste bestand produceert wordt automatisch gekozen
3. **Origineel behoud**: Perfect sized afbeeldingen behouden hun originele formaat zonder re-encoding
4. **Rapportage**: Toont welke encoder is gebruikt en hoeveel ruimte is bespaard

Voorbeeldresponse:
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

## Databaseschema

Vereist PostgreSQL-tabellen zoals gebruikt door Aeron:

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
- PostgreSQL met Aeron-database
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

MIT-licentie - zie het LICENSE-bestand voor details.