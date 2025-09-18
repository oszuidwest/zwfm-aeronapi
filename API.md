# Aeron Image Manager API-documentatie

## Inhoudsopgave

- [Overzicht](#overzicht)
- [Snel overzicht endpoints](#snel-overzicht-endpoints)
- [Authenticatie](#authenticatie)
- [Response-formaat](#response-formaat)
- [Foutmeldingen](#foutmeldingen)
- [Endpoints](#endpoints)
  - [Statuscontrole](#statuscontrole)
  - [Artiestendpoints](#artiestendpoints)
  - [Trackendpoints](#trackendpoints)
  - [Playlist-endpoints](#playlist-endpoints)
- [Codevoorbeelden](#codevoorbeelden)
- [Configuratie](#configuratie)

## Overzicht

De Aeron Image Manager API biedt RESTful-endpoints voor het beheren van afbeeldingen in het Aeron-radioautomatiseringssysteem. Deze API biedt directe databasetoegang om albumhoezen voor tracks en artiestfoto's toe te voegen, op te halen en te verwijderen.

**Basis-URL:** `http://localhost:8080/api`

## Snel overzicht endpoints

| Endpoint | Methode | Beschrijving | Auth |
|----------|---------|--------------|------|
| **Algemeen** |
| `/api/health` | GET | API-status controleren | Nee |
| **Artiesten** |
| `/api/artists` | GET | Statistieken over artiesten | Ja |
| `/api/artists/{id}` | GET | Specifieke artiest ophalen | Ja |
| `/api/artists/{id}/image` | GET | Artiestafbeelding ophalen | Ja |
| `/api/artists/{id}/image` | POST | Artiestafbeelding uploaden | Ja |
| `/api/artists/{id}/image` | DELETE | Artiestafbeelding verwijderen | Ja |
| `/api/artists/bulk-delete` | DELETE | Alle artiestafbeeldingen verwijderen | Ja |
| **Tracks** |
| `/api/tracks` | GET | Statistieken over tracks | Ja |
| `/api/tracks/{id}` | GET | Specifieke track ophalen | Ja |
| `/api/tracks/{id}/image` | GET | Trackafbeelding ophalen | Ja |
| `/api/tracks/{id}/image` | POST | Trackafbeelding uploaden | Ja |
| `/api/tracks/{id}/image` | DELETE | Trackafbeelding verwijderen | Ja |
| `/api/tracks/bulk-delete` | DELETE | Alle trackafbeeldingen verwijderen | Ja |
| **Playlist** |
| `/api/playlist` | GET | Playlistblokken voor datum | Ja |
| `/api/playlist?block_id={id}` | GET | Tracks in playlistblok | Ja |

## Authenticatie

Wanneer authenticatie is ingeschakeld in de configuratie, vereisen alle endpoints (behalve `/health`) een API-sleutel.

**Header:** `X-API-Key: jouw-api-sleutel`

**Response bij ontbrekende autorisatie:**
```json
{
  "error": "Niet geautoriseerd: ongeldige of ontbrekende API-sleutel",
  "request_id": "abc123"
}
```

## Algemene response-headers

Alle API-responses bevatten:
- `Content-Type: application/json; charset=utf-8` (uitgezonderd afbeeldingsendpoints)
- `X-Request-Id: uniek-verzoek-id`

## Response-formaat

Alle JSON-responses gebruiken een consistent wrapper-formaat:
```json
{
  "success": true,
  "data": { ... }  // Bij succesvolle requests
}
```

Of bij fouten:
```json
{
  "success": false,
  "error": "Foutmelding in het Nederlands"
}
```

**Let op:** In de voorbeelden hieronder wordt voor de leesbaarheid alleen de inhoud van het `data`-veld getoond, maar in werkelijkheid wordt altijd de complete wrapper geretourneerd.

## Foutmeldingen

Alle fouten volgen dit formaat:
```json
{
  "error": "Foutmelding",
  "request_id": "unieke-request-id"
}
```

**HTTP-statuscodes:**
- `400` Bad Request - Ongeldige invoerparameters
- `401` Unauthorized - Ongeldige of ontbrekende API-sleutel
- `404` Not Found - Bron niet gevonden
- `500` Internal Server Error - Serverfout

---

## Endpoints

### Statuscontrole

Controleer de status van de API.

**Endpoint:** `GET /api/health`
**Authenticatie:** Niet vereist

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "version": "dev",
    "database": "aeron"
  }
}
```

---

## Artiestendpoints

### Artieststatistieken ophalen

Verkrijg statistieken over artiesten en hun afbeeldingen.

**Endpoint:** `GET /api/artists`
**Authenticatie:** Vereist

**Response:** `200 OK`
```json
{
  "total": 1250,
  "with_images": 450,
  "without_images": 800
}
```

### Artiest ophalen via ID

Verkrijg artiestgegevens inclusief afbeeldingsstatus.

**Endpoint:** `GET /api/artists/{id}`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Artiest-UUID

**Response:** `200 OK`
```json
{
  "artistid": "123e4567-e89b-12d3-a456-426614174000",
  "artist": "The Beatles",
  "has_image": true
}
```

**Foutresponse:** `404 Not Found`
```json
{
  "error": "Artist niet gevonden"
}
```

### Artiestafbeelding ophalen

Verkrijg de afbeelding van de artiest.

**Endpoint:** `GET /api/artists/{id}/image`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Artiest-UUID

**Response:** `200 OK`
- Content-Type: `image/jpeg`, `image/png` of `image/webp`
- Binaire afbeeldingsdata

**Foutresponse:** `404 Not Found`
```json
{
  "error": "Afbeelding niet gevonden"
}
```

### Artiestafbeelding uploaden

Upload of werk een artiestafbeelding bij.

**Endpoint:** `POST /api/artists/{id}/image`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Artiest-UUID

**Request Body:**
```json
{
  "url": "https://voorbeeld.nl/artiest.jpg",
  "image": "base64-gecodeerde-afbeeldingsdata"
}
```
*Let op: Gebruik óf `url` óf `image`, niet beide tegelijk*

**Response:** `200 OK`
```json
{
  "artist": "The Beatles",
  "original_size": 245678,
  "optimized_size": 45678,
  "savings_percent": 81.4,
  "encoder": "jpegli"
}
```

**Foutresponses:**
- `400` Bad Request - Ongeldige invoer
- `404` Not Found - Artiest niet gevonden
- `422` Unprocessable Entity - Afbeeldingsvalidatie mislukt

### Artiestafbeelding verwijderen

Verwijder een artiestafbeelding.

**Endpoint:** `DELETE /api/artists/{id}/image`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Artiest-UUID

**Response:** `200 OK`
```json
{
  "message": "Artist-afbeelding succesvol verwijderd",
  "artist_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

**Foutresponse:** `404 Not Found`
```json
{
  "error": "Afbeelding niet gevonden"
}
```

### Bulkverwijdering artiestafbeeldingen

Verwijder alle artiestafbeeldingen uit de database.

**Endpoint:** `DELETE /api/artists/bulk-delete`
**Authenticatie:** Vereist

**Vereiste header:**
- `X-Confirm-Bulk-Delete: VERWIJDER ALLES`

**Response:** `200 OK`
```json
{
  "deleted": 450,
  "message": "450 artist-afbeeldingen verwijderd"
}
```

**Fout Response:** `400 Bad Request`
```json
{
  "error": "Ontbrekende bevestigingsheader: X-Confirm-Bulk-Delete"
}
```

---

## Trackendpoints

### Trackstatistieken ophalen

Verkrijg statistieken over tracks en hun afbeeldingen.

**Endpoint:** `GET /api/tracks`
**Authenticatie:** Vereist

**Response:** `200 OK`
```json
{
  "total": 5000,
  "with_images": 1200,
  "without_images": 3800
}
```

### Track ophalen via ID

Verkrijg trackgegevens inclusief afbeeldingsstatus.

**Endpoint:** `GET /api/tracks/{id}`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Track-UUID

**Response:** `200 OK`
```json
{
  "titleid": "456e7890-e89b-12d3-a456-426614174000",
  "tracktitle": "Hey Jude",
  "artist": "The Beatles",
  "artistid": "123e4567-e89b-12d3-a456-426614174000",
  "has_image": true,
  "exporttype": 0
}
```

**Foutresponse:** `404 Not Found`
```json
{
  "error": "Track niet gevonden"
}
```

### Trackafbeelding ophalen

Verkrijg de albumhoes van de track.

**Endpoint:** `GET /api/tracks/{id}/image`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Track-UUID

**Response:** `200 OK`
- Content-Type: `image/jpeg`, `image/png` of `image/webp`
- Binaire afbeeldingsdata

**Foutresponse:** `404 Not Found`
```json
{
  "error": "Afbeelding niet gevonden"
}
```

### Trackafbeelding uploaden

Upload of werk een albumhoes bij.

**Endpoint:** `POST /api/tracks/{id}/image`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Track-UUID

**Request Body:**
```json
{
  "url": "https://voorbeeld.nl/albumhoes.jpg",
  "image": "base64-gecodeerde-afbeeldingsdata"
}
```
*Let op: Gebruik óf `url` óf `image`, niet beide tegelijk*

**Response:** `200 OK`
```json
{
  "track": "Hey Jude",
  "artist": "The Beatles",
  "original_size": 345678,
  "optimized_size": 65678,
  "savings_percent": 81.0,
  "encoder": "jpegli"
}
```

**Fout Responses:**
- `400` Bad Request - Ongeldige invoer
- `404` Not Found - Track niet gevonden
- `422` Unprocessable Entity - Afbeelding validatie mislukt

### Trackafbeelding verwijderen

Verwijder de albumhoes van een track.

**Endpoint:** `DELETE /api/tracks/{id}/image`
**Authenticatie:** Vereist

**Parameters:**
- `id` (pad, vereist): Track-UUID

**Response:** `200 OK`
```json
{
  "message": "Track-afbeelding succesvol verwijderd",
  "track_id": "456e7890-e89b-12d3-a456-426614174000"
}
```

**Foutresponse:** `404 Not Found`
```json
{
  "error": "Afbeelding niet gevonden"
}
```

### Bulkverwijdering trackafbeeldingen

Verwijder alle trackafbeeldingen uit de database.

**Endpoint:** `DELETE /api/tracks/bulk-delete`
**Authenticatie:** Vereist

**Vereiste header:**
- `X-Confirm-Bulk-Delete: VERWIJDER ALLES`

**Response:** `200 OK`
```json
{
  "deleted": 1200,
  "message": "1200 track-afbeeldingen verwijderd"
}
```

**Fout Response:** `400 Bad Request`
```json
{
  "error": "Ontbrekende bevestigingsheader: X-Confirm-Bulk-Delete"
}
```

---

## Playlist-endpoints

### Playlistblokken ophalen

Verkrijg alle playlistblokken voor een specifieke datum.

**Endpoint:** `GET /api/playlist`
**Authenticatie:** Vereist

**Queryparameters:**
- `date` (optioneel): Datum in JJJJ-MM-DD-formaat (standaard: vandaag)

**Response:** `200 OK`
```json
[
  {
    "blockid": "block-uuid-1",
    "name": "Ochtend Show",
    "date": "2025-09-17",
    "start_time": "06:00:00",
    "end_time": "10:00:00",
    "tracks": [
      {
        "songid": "track-uuid-1",
        "songname": "Nummer Titel",
        "artistid": "artist-uuid-1",
        "artistname": "Artiest Naam",
        "start_time": "06:00:00",
        "end_time": "06:03:24",
        "duration": 204000,
        "has_track_image": true,
        "has_artist_image": false,
        "exporttype": 0,
        "mode": 2,
        "is_voicetrack": false,
        "is_commblock": false
      }
    ]
  }
]
```

### Playlisttracks per blok ophalen

Verkrijg tracks voor een specifiek playlistblok.

**Endpoint:** `GET /api/playlist?block_id={block_id}`
**Authenticatie:** Vereist

**Queryparameters:**
- `block_id` (vereist): Playlistblok-UUID
- `limit` (optioneel): Maximaal aantal tracks (standaard: 1000)
- `offset` (optioneel): Offset voor paginering (standaard: 0)
- `track_image` (optioneel): Filter op trackafbeeldingsstatus (`true`/`false`/`yes`/`no`/`1`/`0`)
- `artist_image` (optioneel): Filter op artiestafbeeldingsstatus (`true`/`false`/`yes`/`no`/`1`/`0`)
- `sort` (optioneel): Sorteerveld (`start_time`, `track`, `artist`, `duration`)
- `desc` (optioneel): Sorteer aflopend indien `true`

**Response:** `200 OK`
```json
[
  {
    "songid": "track-uuid-1",
    "songname": "Nummer Titel",
    "artistid": "artist-uuid-1",
    "artistname": "Artiest Naam",
    "start_time": "06:00:00",
    "end_time": "06:03:24",
    "duration": 204000,
    "has_track_image": true,
    "has_artist_image": false,
    "exporttype": 0,
    "mode": 2,
    "is_voicetrack": false,
    "is_commblock": false
  }
]
```

---

## Afbeeldingsverwerking

### Afbeeldingsoptimalisatie

Alle geüploade afbeeldingen worden automatisch:
1. Gevalideerd op formaat (JPEG, PNG, WebP)
2. Gecontroleerd op minimumafmetingen (configureerbaar, standaard: 200×200)
3. Geschaald naar maximumafmetingen (configureerbaar, standaard: 640×640)
4. Geoptimaliseerd met dubbele codering (standaard-JPEG en jpegli)
5. Opgeslagen met de kleinste bestandsgrootte

### Ondersteunde afbeeldingsbronnen

1. **URL-download**: Geef een URL op om de afbeelding te downloaden
   - Ondersteunt HTTPS-URL's
   - Valideert URL-veiligheid
   - Download met time-out van 30 seconden

2. **Base64-upload**: Verstuur base64-gecodeerde afbeeldingsdata
   - Ondersteunt standaard base64-codering
   - Maximumgrootte beperkt door verzoeklimieten

### Afbeeldingsvalidatieregels

- **Minimumafmetingen**: Configureerbaar (standaard: 200×200)
- **Maximumafmetingen**: Configureerbaar (standaard: 640×640)
- **Toegestane formaten**: JPEG, PNG, WebP
- **Beeldverhouding**: Wordt behouden tijdens schalen
- **Kwaliteit**: Configureerbare JPEG-kwaliteit (standaard: 85)

---

## Bedrijfsregels

### Afbeeldingsverwerking
- Afbeeldingen worden automatisch geoptimaliseerd voor gebruik in Aeron
- Zowel standaard JPEG als jpegli-compressie wordt toegepast
- De kleinste bestandsgrootte wordt automatisch geselecteerd

### UUID-validatie
- Alle artiest- en track-ID's moeten geldige UUID's zijn (versie 4-formaat)
- Ongeldige UUID's resulteren in 400 Bad Request met Nederlandse foutmelding
- Voorbeeld geldig UUID: `123e4567-e89b-12d3-a456-426614174000`

### Afbeeldingsopslag
- Afbeeldingen worden opgeslagen als BYTEA in PostgreSQL
- Originele afbeeldingen worden niet bewaard
- Uitsluitend geoptimaliseerde versies worden opgeslagen

---

## Frequentiebeperking

Geen ingebouwde frequentiebeperking. Implementeer deze indien nodig op proxy- of load-balancerniveau.

---

## Gebruiksvoorbeelden

### cURL-voorbeelden

**Artiestafbeelding uploaden via URL:**
```bash
curl -X POST "http://localhost:8080/api/artists/123e4567-e89b-12d3-a456-426614174000/image" \
  -H "X-API-Key: jouw-api-sleutel" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://voorbeeld.nl/artiest.jpg"}'
```

**Trackafbeelding ophalen:**
```bash
curl -X GET "http://localhost:8080/api/tracks/456e7890-e89b-12d3-a456-426614174000/image" \
  -H "X-API-Key: jouw-api-sleutel" \
  --output track-afbeelding.jpg
```

**Alle artiestafbeeldingen verwijderen (let op: onomkeerbaar!):**
```bash
curl -X DELETE "http://localhost:8080/api/artists/bulk-delete" \
  -H "X-API-Key: jouw-api-sleutel" \
  -H "X-Confirm-Bulk-Delete: VERWIJDER ALLES"
```

**Playlist voor vandaag ophalen:**
```bash
curl -X GET "http://localhost:8080/api/playlist" \
  -H "X-API-Key: jouw-api-sleutel"
```

### Python-voorbeeld

```python
import requests
import base64

API_KEY = "jouw-api-sleutel"
BASE_URL = "http://localhost:8080/api"

headers = {"X-API-Key": API_KEY}

# Afbeelding uploaden vanuit bestand
with open("albumhoes.jpg", "rb") as f:
    image_data = base64.b64encode(f.read()).decode()

response = requests.post(
    f"{BASE_URL}/tracks/456e7890-e89b-12d3-a456-426614174000/image",
    headers=headers,
    json={"image": image_data}
)

if response.status_code == 200:
    result = response.json()
    print(f"Afbeelding geoptimaliseerd: {result['savings_percent']}% ruimtebesparing")
```

### JavaScript/Node.js-voorbeeld

```javascript
const axios = require('axios');
const fs = require('fs');

const API_KEY = 'jouw-api-sleutel';
const BASE_URL = 'http://localhost:8080/api';

// Artiestafbeelding uploaden via URL
async function uploadArtiestAfbeelding(artistId, imageUrl) {
    try {
        const response = await axios.post(
            `${BASE_URL}/artists/${artistId}/image`,
            { url: imageUrl },
            { headers: { 'X-API-Key': API_KEY } }
        );
        console.log('Upload succesvol:', response.data);
    } catch (error) {
        console.error('Upload mislukt:', error.response.data);
    }
}

// Playlisttracks ophalen met filters
async function haalPlaylistTracksOp(blockId) {
    try {
        const response = await axios.get(
            `${BASE_URL}/playlist`,
            {
                params: {
                    block_id: blockId,
                    track_image: 'false',
                    limit: 50
                },
                headers: { 'X-API-Key': API_KEY }
            }
        );
        console.log(`${response.data.length} tracks zonder afbeeldingen gevonden`);
    } catch (error) {
        console.error('Verzoek mislukt:', error.response.data);
    }
}
```

---

## Configuratie

Het gedrag van de API kan worden geconfigureerd via `config.yaml`:

```yaml
# Databaseconfiguratie
database:
  host: localhost
  port: 5432
  user: aeron
  password: ""
  name: aeron
  schema: aeron
  sslmode: disable

# Afbeeldingsverwerking
image:
  target_width: 640
  target_height: 640
  quality: 85
  reject_smaller: false

# API-configuratie
api:
  enabled: true
  keys:
    - "jouw-veilige-api-sleutel-hier"
```

---

## Belangrijke opmerkingen

- Alle foutmeldingen zijn in het Nederlands conform het Aeron-systeem
- UUID's zijn hoofdletterongevoelig
- Het contenttype van afbeeldingen wordt automatisch gedetecteerd
- De API maakt gebruik van connection pooling voor optimale databaseprestaties
- Geoptimaliseerd voor PostgreSQL met de Aeron-schemastructuur