# API Documentatie

Dit document bevat uitgebreide documentatie voor alle API-endpoints van de ZWFM Aeron API.

## API-endpoints

### Overzicht

| Methode | Endpoint | Authenticatie | Beschrijving |
|---------|----------|---------------|--------------|
| **GET** | `/api/health` | Nee | Statuscontrole en versie-informatie |
| **GET** | `/api/artists` | Ja | Statistieken van alle artiesten |
| **GET** | `/api/artists/{id}` | Ja | Details van een specifieke artiest |
| **GET** | `/api/artists/{id}/image` | Ja | Afbeelding van artiest ophalen |
| **POST** | `/api/artists/{id}/image` | Ja | Afbeelding voor artiest uploaden |
| **DELETE** | `/api/artists/{id}/image` | Ja | Afbeelding van artiest verwijderen |
| **DELETE** | `/api/artists/bulk-delete` | Ja* | Alle artiestafbeeldingen verwijderen |
| **GET** | `/api/tracks` | Ja | Statistieken van alle tracks |
| **GET** | `/api/tracks/{id}` | Ja | Details van een specifieke track |
| **GET** | `/api/tracks/{id}/image` | Ja | Afbeelding van track ophalen |
| **POST** | `/api/tracks/{id}/image` | Ja | Afbeelding voor track uploaden |
| **DELETE** | `/api/tracks/{id}/image` | Ja | Afbeelding van track verwijderen |
| **DELETE** | `/api/tracks/bulk-delete` | Ja* | Alle trackafbeeldingen verwijderen |
| **GET** | `/api/playlist` | Ja | Playlist met filters ophalen |

*Vereist extra bevestigingsheader

### Algemene informatie

#### Authenticatie
Alle endpoints behalve `/api/health` vereisen een API-sleutel:
```http
X-API-Key: jouw-api-key
```

#### Response Headers
- **Content-Type**: `application/json` (standaard) of `image/*` voor afbeeldingen
- **X-Request-ID**: Unieke ID voor request tracking
- Ondersteunt gzip-compressie

#### Statuscodes
| Code | Betekenis | Wanneer |
|------|-----------|---------|
| **200** | OK | Succesvolle operatie |
| **400** | Bad Request | Ongeldige parameters, formaat niet ondersteund, afbeelding te klein |
| **401** | Unauthorized | Ongeldige of ontbrekende API-sleutel |
| **404** | Not Found | Entiteit bestaat niet of heeft geen afbeelding |
| **405** | Method Not Allowed | HTTP-methode niet toegestaan |
| **500** | Internal Server Error | Database- of verwerkingsfout |

##### 404 Not Found - Twee scenario's

De API retourneert een 404-statuscode in twee verschillende situaties bij het ophalen van afbeeldingen:

1. **Entiteit bestaat niet**
   - Foutmelding: `"artiest-ID 'xxx' bestaat niet"` of `"track-ID 'xxx' bestaat niet"`
   - De opgegeven ID komt niet voor in de database

2. **Entiteit bestaat maar heeft geen afbeelding**
   - Foutmelding: `"artiest heeft geen afbeelding"` of `"track heeft geen afbeelding"`
   - De artiest/track bestaat wel, maar er is geen afbeelding opgeslagen

**Waarom beide 404?** Dit is een bewuste ontwerpkeuze die aansluit bij industriestandaarden (zoals GitHub's API). Voor de meeste clients maakt het niet uit waarom de afbeelding niet beschikbaar is - ze tonen in beide gevallen een placeholder. Clients die wel onderscheid willen maken kunnen de foutmelding analyseren.

#### Foutrespons
```json
{
  "success": false,
  "error": "Beschrijving van de fout"
}
```

### Statuscontrole

```http
GET /api/health
```

Geen authenticatie vereist. Retourneert de serverstatus en versie.

### Artiesten

#### Statistieken van artiesten ophalen
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

#### Enkele artiest ophalen
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
Het endpoint `/api/artists/{artistid}/image` ondersteunt het ophalen (GET), uploaden (POST) en verwijderen (DELETE) van afbeeldingen.

##### Afbeelding ophalen
```http
GET /api/artists/{artistid}/image
X-API-Key: jouw-api-key
```

Retourneert:
- **Content-Type**: `image/jpeg` of `image/png` (automatisch gedetecteerd)
- **Status**: 200 (OK) met binaire afbeeldingsdata
- **Status**: 404 (Not Found) als de artiest niet bestaat of geen afbeelding heeft

**Let op**: Een 404-status kan twee dingen betekenen:
- De artiest bestaat niet: `{"error": "artiest-ID 'xxx' bestaat niet"}`
- De artiest heeft geen afbeelding: `{"error": "artiest heeft geen afbeelding"}`

Tip: Gebruik eerst `GET /api/artists/{id}` om te controleren of de artiest bestaat en een afbeelding heeft (`has_image` veld).

##### Afbeelding uploaden
```http
POST /api/artists/{artistid}/image
X-API-Key: jouw-api-key
Content-Type: application/json

{
  "url": "https://example.com/afbeelding.jpg"
}
```

Of met een base64-gecodeerde afbeelding:
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
    "message": "Artiestafbeelding succesvol verwijderd",
    "artist_id": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

**Ondersteunde formaten**: JPEG (.jpg, .jpeg) en PNG (.png)

**Let op**: 
- De artiest-ID wordt uit de URL gehaald
- Andere formaten (WEBP, GIF, BMP, enz.) worden geweigerd bij het uploaden

#### Alle artiestafbeeldingen verwijderen
```http
DELETE /api/artists/bulk-delete
X-API-Key: jouw-api-key
X-Confirm-Bulk-Delete: VERWIJDER ALLES
```

### Tracks

#### Statistieken van tracks ophalen
```http
GET /api/tracks
X-API-Key: jouw-api-key
```

#### Enkele track ophalen
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
Het endpoint `/api/tracks/{trackid}/image` ondersteunt het ophalen (GET), uploaden (POST) en verwijderen (DELETE) van afbeeldingen.

##### Afbeelding ophalen
```http
GET /api/tracks/{trackid}/image
X-API-Key: jouw-api-key
```

Retourneert:
- **Content-Type**: `image/jpeg` of `image/png` (automatisch gedetecteerd)
- **Status**: 200 (OK) met binaire afbeeldingsdata
- **Status**: 404 (Not Found) als de track niet bestaat of geen afbeelding heeft

**Let op**: Een 404-status kan twee dingen betekenen:
- De track bestaat niet: `{"error": "track-ID 'xxx' bestaat niet"}`
- De track heeft geen afbeelding: `{"error": "track heeft geen afbeelding"}`

Tip: Gebruik eerst `GET /api/tracks/{id}` om te controleren of de track bestaat en een afbeelding heeft (`has_image` veld).

##### Afbeelding uploaden
```http
POST /api/tracks/{trackid}/image
X-API-Key: jouw-api-key
Content-Type: application/json

{
  "url": "https://example.com/albumcover.jpg"
}
```

Of met een base64-gecodeerde afbeelding:
```json
{
  "image": "data:image/jpeg;base64,..."
}
```

Response:
```json
{
  "success": true,
  "data": {
    "track": "Bohemian Rhapsody",
    "artist": "Queen",
    "original_size": 524288,
    "optimized_size": 98304,
    "savings_percent": 81.25,
    "encoder": "jpegli (98 KB) versus standaard (120 KB)"
  }
}
```

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
    "message": "Trackafbeelding succesvol verwijderd",
    "track_id": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

**Let op**: 
- De track-ID wordt uit de URL gehaald
- Andere formaten (WEBP, GIF, BMP, enz.) worden geweigerd bij het uploaden

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

Queryparameters:
- `date`: Specifieke datum (YYYY-MM-DD), standaard: vandaag
- `from`: Starttijdfilter (HH:MM)
- `to`: Eindtijdfilter (HH:MM)
- `limit`: Maximum aantal items
- `offset`: Paginering-offset
- `track_image`: Filter op trackafbeelding (yes/no)
- `artist_image`: Filter op artiestafbeelding (yes/no)
- `sort`: Sorteer op veld (time/artist/track)
- `desc`: Sorteer aflopend (true/false)

Voorbeelden:
```http
# Playlist van vanmiddag
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

Authenticatie kan geconfigureerd worden in `config.yaml`:

```yaml
api:
  enabled: true  # Schakel authenticatie in
  keys:
    - "jouw-api-key-hier"
    - "nog-een-api-key"
```

Gebruik de API-sleutel in verzoeken:
```bash
# Via header
curl -H "X-API-Key: jouw-api-key-hier" http://localhost:8080/api/artists
```

## Responsformaat

Succesvolle responsen:
```json
{
  "success": true,
  "data": { ... }
}
```

Foutresponsen:
```json
{
  "success": false,
  "error": "Foutmelding"
}
```

## Veelvoorkomende use cases

### 1. Bulk upload van albumhoezen
```python
# Python voorbeeld
import requests

api_key = "jouw-api-key"
base_url = "http://localhost:8080/api"

# Upload albumhoezen voor meerdere tracks
tracks = [
    {"id": "track-uuid-1", "image_url": "https://covers.com/album1.jpg"},
    {"id": "track-uuid-2", "image_url": "https://covers.com/album2.jpg"},
]

for track in tracks:
    response = requests.post(
        f"{base_url}/tracks/{track['id']}/image",
        headers={"X-API-Key": api_key},
        json={"url": track["image_url"]}
    )
    print(f"Track {track['id']}: {response.json()}")
```

### 2. Controleer welke artiesten geen afbeelding hebben
```bash
# Haal statistieken op
curl -H "X-API-Key: jouw-api-key" http://localhost:8080/api/artists

# Filter playlist op items zonder afbeeldingen
curl -H "X-API-Key: jouw-api-key" \
  "http://localhost:8080/api/playlist?artist_image=no"
```

### 3. Robuust omgaan met afbeeldingen (404 handling)
```python
# Python voorbeeld voor robuuste afbeelding handling
import requests

def get_artist_image(api_key, artist_id):
    # Eerst controleren of artiest bestaat en afbeelding heeft
    response = requests.get(
        f"http://localhost:8080/api/artists/{artist_id}",
        headers={"X-API-Key": api_key}
    )
    
    if response.status_code == 404:
        print(f"Artiest {artist_id} bestaat niet")
        return None
    
    artist_data = response.json()['data']
    if not artist_data.get('has_image'):
        print(f"Artiest {artist_data['artist']} heeft geen afbeelding")
        return None
    
    # Nu veilig de afbeelding ophalen
    image_response = requests.get(
        f"http://localhost:8080/api/artists/{artist_id}/image",
        headers={"X-API-Key": api_key}
    )
    
    return image_response.content if image_response.status_code == 200 else None
```

### 4. Automatische synchronisatie met externe bronnen
```javascript
// Node.js voorbeeld
const axios = require('axios');

async function syncArtistImages() {
  // Haal playlist op
  const playlist = await axios.get('/api/playlist', {
    headers: { 'X-API-Key': apiKey },
    params: { artist_image: 'no', limit: 100 }
  });

  // Voor elke artiest zonder afbeelding
  for (const item of playlist.data.data) {
    // Zoek afbeelding in externe bron (bijv. Spotify, Last.fm)
    const imageUrl = await findArtistImage(item.artistname);
    
    if (imageUrl) {
      // Upload naar Aeron
      await axios.post(
        `/api/artists/${item.artistid}/image`,
        { url: imageUrl },
        { headers: { 'X-API-Key': apiKey } }
      );
    }
  }
}
```

### 5. Backup en restore van afbeeldingen
```bash
# Export alle artiestafbeeldingen
for id in $(curl -s -H "X-API-Key: $API_KEY" http://localhost:8080/api/artists \
  | jq -r '.data.artist_ids[]'); do
  curl -s -H "X-API-Key: $API_KEY" \
    http://localhost:8080/api/artists/$id/image \
    -o "backup/artist_$id.jpg"
done

# Restore vanaf backup
for file in backup/artist_*.jpg; do
  id=$(basename $file .jpg | cut -d_ -f2)
  base64_image=$(base64 -w 0 < $file)
  curl -X POST http://localhost:8080/api/artists/$id/image \
    -H "X-API-Key: $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"image\":\"data:image/jpeg;base64,$base64_image\"}"
done
```

## Afbeeldingverwerking

- **Ondersteunde bestandsformaten**: JPEG (.jpg, .jpeg) en PNG (.png) - andere formaten worden geweigerd
- **Doelafmetingen**: Configureerbaar (standaard 640×640)
- **Kwaliteit**: Configureerbaar (standaard 90)
- **Intelligente verwerking**:
  - **Perfecte grootte**: Origineel formaat behouden (PNG blijft PNG, JPEG blijft JPEG)
  - **Verkleining nodig**: Altijd geconverteerd naar JPEG met Jpegli/standaardoptimalisatie
- **Validatie**: Kleinere afbeeldingen worden geweigerd (configureerbaar)

### Encoding-optimalisatie

De API gebruikt intelligente compressie voor afbeeldingen die verkleind moeten worden:

1. **Dubbele codering**: Afbeeldingen die verkleind worden, worden gecodeerd met zowel Jpegli als de standaard Go JPEG-encoder
2. **Automatische selectie**: De encoder die het kleinste bestand produceert, wordt automatisch gekozen
3. **Behoud van origineel**: Afbeeldingen met de perfecte grootte behouden hun originele formaat zonder hercodering
4. **Rapportage**: Toont welke encoder is gebruikt en hoeveel ruimte is bespaard

Voorbeeldrespons:
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