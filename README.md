# ZuidWest FM Aeron API

Een **onofficiële** REST-API voor het Aeron-radioautomatiseringssysteem, specifiek ontwikkeld voor het **toevoegen en beheren van afbeeldingen** bij tracks en artiesten.

> [!WARNING]
> Aeron is een product van Broadcast Partners. Deze API is volledig onofficieel en niet ontwikkeld door of in samenwerking met Broadcast Partners. Gebruik is op eigen risico. Maak altijd een back-up van je database voordat je deze tool gebruikt.

## Waarom deze API?

Het Aeron-systeem biedt standaard geen mogelijkheid om programmatisch afbeeldingen toe te voegen aan tracks en artiesten. Deze onofficiële API vult dat gat door directe toegang tot de Aeron-database te bieden, waardoor je:
- **Automatisch albumhoezen kunt toevoegen** aan tracks
- **Artiestfoto's kunt uploaden** voor een visueel aantrekkelijke presentatie
- **Bulk-operaties kunt uitvoeren** om snel grote hoeveelheden afbeeldingen te verwerken

## Hoofdfunctionaliteit: Afbeeldingen toevoegen

De primaire focus van deze API is het **toevoegen van afbeeldingen** aan:
- **Artiesten**: Voeg foto's toe aan artiestprofielen
- **Tracks**: Voeg albumhoezen of andere afbeeldingen toe aan individuele tracks

### Aanvullende functionaliteiten

- **Automatische optimalisatie**: Afbeeldingen worden automatisch verkleind en geoptimaliseerd voor gebruik in Aeron
- **Dubbele encoder**: Vergelijkt jpegli met standaard JPEG en selecteert de kleinste bestandsgrootte
- **Playlist-API**: Haal playlistinformatie op met inzicht in welke items afbeeldingen hebben
- **Statistieken**: Monitor hoeveel artiesten en tracks al afbeeldingen hebben
- **Bulkoperaties**: Verwijder alle afbeeldingen voor een specifiek type in één keer
- **Authenticatie**: Optionele API-sleutelauthenticatie voor beveiliging

## Snelstart

### 🐳 Docker (Aanbevolen)
```bash
# Start met example config
docker run -d -p 8080:8080 --name zwfm-aeronapi ghcr.io/oszuidwest/zwfm-aeronapi:latest

# Of met eigen config
docker run -d -p 8080:8080 -v $(pwd)/config.yaml:/config.yaml --name zwfm-aeronapi ghcr.io/oszuidwest/zwfm-aeronapi:latest
```

### 🪟 Windows
1. Download het `.exe` bestand van de [laatste release](https://github.com/oszuidwest/zwfm-aeronapi/releases/latest)
2. Plaats het in een map (bijvoorbeeld `C:\Program Files\zwfm-aeronapi`)
3. Maak een `config.yaml` bestand in dezelfde map
4. Start via command prompt: `zwfm-aeronapi.exe`

## Installatie

### Met Docker

```bash
# Download config voorbeeld
wget https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/config.example.yaml -O config.yaml
# Pas config.yaml aan

# Start container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config.yaml \
  --name zwfm-aeronapi \
  --restart unless-stopped \
  ghcr.io/oszuidwest/zwfm-aeronapi:latest
```

### Vanaf broncode

```bash
git clone https://github.com/oszuidwest/zwfm-aeronapi.git
cd zwfm-aeronapi

# Kopieer en pas de configuratie aan
cp config.example.yaml config.yaml
# Bewerk config.yaml met je database gegevens

# Bouw en start
go build -o zwfm-aeronapi .
./zwfm-aeronapi -config=config.yaml -port=8080
```

### Windows Service

Voor Windows is er een alles-in-één installer:

```powershell
# Open PowerShell als Administrator

# Optie 1: Direct installeren
irm https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/install.ps1 | iex

# Optie 2: Download script eerst (voor inspectie)
Invoke-WebRequest -Uri https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/install.ps1 -OutFile install.ps1
.\install.ps1
```

De installer:
- Downloadt automatisch de laatste versie
- Maakt de Windows service aan
- Kopieert een voorbeeld configuratie
- Start de service automatisch

Zie [Windows documentatie](WINDOWS.md) voor meer details.

### Binaire bestanden

Download voor andere platformen van de [Releases-pagina](https://github.com/oszuidwest/zwfm-aeronapi/releases):

- **Linux**: amd64, arm64, armv7
- **Windows**: amd64, arm64 (zie [Windows Service installatie](WINDOWS.md) voor Aeron-integratie)
- **macOS**: Intel (amd64), Apple Silicon (arm64)

## Configuratie

Kopieer `config.example.yaml` naar `config.yaml` en pas de waardes aan:

```bash
cp config.example.yaml config.yaml
```

Zie [`config.example.yaml`](config.example.yaml) voor:
- Volledige documentatie van alle configuratie opties
- Voorbeelden voor verschillende omgevingen (ontwikkeling, productie, Windows)
- Uitleg over elke instelling

## API-server starten

```bash
# Standaardpoort 8080
./zwfm-aeronapi

# Aangepaste poort
./zwfm-aeronapi -port=9090

# Met aangepaste configuratie
./zwfm-aeronapi -config=/pad/naar/config.yaml -port=8080

# Toon versie
./zwfm-aeronapi -version
```

## Quick Start

### 1. Start de API
```bash
./zwfm-aeronapi -config=config.yaml
```

### 2. Test de verbinding
```bash
curl http://localhost:8080/api/health
```

### 3. Upload een artiestafbeelding
```bash
curl -X POST http://localhost:8080/api/artists/{artistid}/image \
  -H "X-API-Key: jouw-api-key" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/artist.jpg"}'
```

### 4. Upload een trackafbeelding
```bash
curl -X POST http://localhost:8080/api/tracks/{trackid}/image \
  -H "X-API-Key: jouw-api-key" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/album.jpg"}'
```

Voor uitgebreide API-documentatie, inclusief alle endpoints, voorbeelden en use cases, zie [API.md](API.md).

## Windows Service installatie

Als je deze API naast Aeron op een Windows-server wilt draaien, zie [WINDOWS.md](WINDOWS.md) voor:
- Automatische service installatie
- Integratie met Aeron
- Service beheer en troubleshooting

## Databaseschema

Vereiste PostgreSQL-tabellen zoals gebruikt door Aeron:

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
- Jpegli (optioneel, voor verbeterde compressie)

### Bouwen
```bash
go mod download
go build -o zwfm-aeronapi .
```

### Testen
```bash
go test ./...
```

## Licentie

MIT-licentie - zie het LICENSE-bestand voor details.
