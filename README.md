# Aeron Image Manager

Een command-line tool voor het beheer van afbeeldingen in Aeron databases met slimme beeldoptimalisatie.

> \[!WARNING]
> Aeron is een product van Broadcast Partners. Deze tool is niet officieel en is niet ontwikkeld in opdracht van of in samenwerking met Broadcast Partners. Gebruik ervan is volledig op eigen risico. Maak altijd eerst een backup van de database voordat je deze tool gebruikt.

## Functionaliteit

* Voegt afbeeldingen toe aan artiesten- en trackrecords in PostgreSQL database
* **Dubbele encoder optimalisatie**: Vergelijkt automatisch Jpegli vs standaard JPEG en kiest de kleinste bestandsgrootte
* Ondersteunt JPG, JPEG en PNG invoerbestanden
* Toont artiesten of tracks zonder afbeeldingen met totaaltellingen
* Zoekt artiesten of tracks op gedeeltelijke naam
* Verwijdert afbeeldingen per scope (nuke-functie)
* Dry-run modus voor het testen van operaties
* REST API server modus voor integratie met andere applicaties
* Vereist config.yaml bestand voor alle instellingen
* Uniforme interface met verplichte -scope parameter

## Installatie

```bash
git clone https://github.com/oszuidwest/aeron-imgman.git
cd aeron-imgman
go mod tidy
go build -o aeron-imgman .
```

### Gecompileerde versies

Je kunt ook gecompileerde executables downloaden via de [Releases pagina](https://github.com/oszuidwest/aeron-imgman/releases):

- **Linux**: amd64, arm64, armv7
- **Windows**: amd64, arm64  
- **macOS**: Intel (amd64), Apple Silicon (arm64)

Download het juiste bestand voor je platform, maak het uitvoerbaar (`chmod +x`) en gebruik het direct.

## Gebruik

### Basiscommando's

```bash
# Artiestafbeelding bijwerken via URL
./aeron-imgman -scope=artist -name="OneRepublic" -url="https://example.com/image.jpg"

# Trackafbeelding bijwerken via lokaal bestand
./aeron-imgman -scope=track -name="Counting Stars" -file="/pad/naar/image.jpg"

# Statistieken tonen voor artiesten (totaal, met/zonder afbeeldingen)
./aeron-imgman -scope=artist -list

# Artiesten zonder afbeelding tonen met voorbeelden
./aeron-imgman -scope=artist -list -filter=without

# Artiesten MET afbeelding tonen met voorbeelden
./aeron-imgman -scope=artist -list -filter=with

# Tracks statistieken tonen
./aeron-imgman -scope=track -list

# Tracks zonder afbeelding tonen met voorbeelden
./aeron-imgman -scope=track -list -filter=without

# Tracks MET afbeelding tonen met voorbeelden
./aeron-imgman -scope=track -list -filter=with

# Zoeken naar artiesten met gedeeltelijke naam
./aeron-imgman -scope=artist -search="Chef"

# Zoeken naar tracks met gedeeltelijke naam
./aeron-imgman -scope=track -search="Stars"

# Afbeeldingen verwijderen per scope
./aeron-imgman -scope=artist -nuke
./aeron-imgman -scope=track -nuke

# Versie-informatie tonen
./aeron-imgman -version

# Dry-run (voorbeeld zonder wijzigingen)
./aeron-imgman -scope=artist -name="OneRepublic" -url="image.jpg" -dry-run
./aeron-imgman -scope=track -nuke -dry-run
```

### Alle beschikbare opties

```bash
# Volledige lijst van opties
./aeron-imgman

# Opties:
  -scope string     Verplicht: 'artist' of 'track'
  -name string      Naam van artiest of track titel
  -id string        UUID van artiest of track
  -url string       URL van de afbeelding om te downloaden
  -file string      Lokaal pad naar afbeelding
  -search string    Zoek items met gedeeltelijke naam match
  -config string    Pad naar config bestand (standaard: config.yaml)
  -dry-run          Toon wat gedaan zou worden zonder bij te werken
  -list             Toon statistieken en voorbeelden
  -filter string    Filter voor list voorbeelden: 'with', 'without' of 'stats-only'
  -nuke             Verwijder afbeeldingen van opgegeven scope
  -server           Start REST API server
  -port string      Server poort (standaard: 8080)
  -version          Toon versie-informatie
```

### Nuke modus

De `-nuke` functie verwijdert afbeeldingen uit de database op basis van de opgegeven scope:

* `-scope=artist -nuke`: Verwijdert alle artiestafbeeldingen
* `-scope=track -nuke`: Verwijdert alle trackafbeeldingen

Veiligheidsmaatregelen:
* **Voorvertoning**: Toont eerst hoeveel en welke items geraakt worden
* **Bevestiging vereist**: Je moet expliciet "VERWIJDER ALLES" typen om door te gaan
* **Dry-run ondersteuning**: Test de functie zonder veranderingen door te voeren met `-dry-run`

> **⚠️ WAARSCHUWING**: De nuke-functie is onomkeerbaar. Maak altijd eerst een backup van je database!

### Configuratie

#### Configuratiebestand gebruiken

```bash
# Gebruikt standaard config.yaml in huidige directory
./aeron-imgman -scope=artist -name="Naam Artiest" -url="image.jpg"
./aeron-imgman -scope=track -name="Track Titel" -url="image.jpg"

# Of aangepaste configuratie gebruiken
./aeron-imgman -config="/pad/naar/config.yaml" -scope=artist -name="Naam Artiest" -url="image.jpg"
```

#### Configuratie vereist

**Let op:** Een `config.yaml` bestand is verplicht. De tool heeft geen ingebouwde standaardwaarden meer om verwarring te voorkomen. Alle database- en afbeeldinginstellingen moeten expliciet worden geconfigureerd.

## Afbeeldingverwerking

* Doelafmeting: Configureerbaar via config.yaml (bijvoorbeeld 1280x1280 pixels)
* Kwaliteit: Configureerbaar via config.yaml (bijvoorbeeld 90)
* **Slimme encoder vergelijking**: Vergelijkt automatisch Jpegli vs standaard Go JPEG encoder en kiest de kleinste bestandsgrootte
* Uitvoerformaat: altijd JPEG, ongeacht invoerformaat
* Schaling: afbeeldingen groter dan doelafmetingen worden verkleind
* Validatie: kleinere afbeeldingen worden geweigerd (configureerbaar)

### Encoding optimalisatie

De tool gebruikt een slimme aanpak voor bestandscompressie:

1. **Dubbele encoding**: Elke afbeelding wordt geëncodeerd met zowel Jpegli als de standaard Go JPEG encoder
2. **Automatische selectie**: De encoder met de kleinste bestandsgrootte wordt automatisch gekozen
3. **Rapportage**: De tool toont welke encoder werd gebruikt en hoeveel ruimte werd bespaard
4. **Geen kwaliteitsverlies**: Beide encoders gebruiken dezelfde kwaliteitsinstellingen

Voorbeeld output:
```
✓ Queen: 2048KB → 187KB (jpegli)
✓ Coldplay: 1536KB → 201KB (standaard)
```

Dit zorgt ervoor dat je altijd de best mogelijke compressie krijgt, ongeacht het type bronafbeelding.

## Vereisten

* Go versie 1.24 of hoger
* PostgreSQL-database met artiesttabel (Aeron-database)
* Netwerktoegang voor downloaden van afbeeldingen bij gebruik van webadresssen

## Databaseschema

De tool vereist de volgende PostgreSQL-tabellen zoals gebruikt door Aeron:

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
    picture BYTEA,
    -- andere velden
);
```

## REST API

Start de API server:
```bash
./aeron-imgman -server -port=8080
```

### API Endpoints

#### Health Check
```bash
GET /api/health
```

#### Artists
```bash
# Artist statistieken
GET /api/artists
Response: {
  "success": true,
  "data": {
    "total": 80,
    "with_images": 10,
    "without_images": 70,
    "orphaned": 5
  }
}

# Search artists
GET /api/artists/search?q=searchterm

# Upload image
POST /api/artists/upload
{
  "name": "Artist Name",  # of gebruik "id": "UUID"
  "url": "https://example.com/image.jpg"  # of "image": "base64_encoded_data"
}

# Delete all artist images (requires header)
DELETE /api/artists/nuke
Header: X-Confirm-Nuke: VERWIJDER ALLES
```

#### Tracks
```bash
# Track statistieken
GET /api/tracks
Response: {
  "success": true,
  "data": {
    "total": 120,
    "with_images": 10,
    "without_images": 110,
    "orphaned": 3
  }
}

# Search tracks
GET /api/tracks/search?q=searchterm

# Upload image
POST /api/tracks/upload
{
  "name": "Track Title",  # of gebruik "id": "UUID"
  "url": "https://example.com/image.jpg"  # of "image": "base64_encoded_data"
}

# Delete all track images (requires header)
DELETE /api/tracks/nuke
Header: X-Confirm-Nuke: VERWIJDER ALLES
```

### API Authenticatie

De API ondersteunt optionele authenticatie via API keys:

```yaml
# In config.yaml
api:
  enabled: true  # Schakel authenticatie in
  keys:
    - "your-api-key-here"
    - "another-api-key"
```

Gebruik de API key op één van deze manieren:
```bash
# Via header (aanbevolen)
curl -H "X-API-Key: your-api-key-here" http://localhost:8080/api/artists

# Via query parameter
curl http://localhost:8080/api/artists?key=your-api-key-here
```

### API Response Format

Succesvolle responses:
```json
{
  "success": true,
  "data": { ... },
  "total": 70,   // Bij list endpoints: totaal aantal items
  "shown": 50    // Bij list endpoints: aantal getoonde items
}
```

Error responses:
```json
{
  "success": false,
  "error": "Error message"
}
```

## Over dit project

Dit project ondersteunt het beheer en optimaliseren van artiestafbeeldingen in Aeron-databases. Het is compatibel met de nieuwste versie van de AerOn Studio radio-automatisering.

## Licentie

MIT-licentie – zie LICENSE-bestand voor details.
