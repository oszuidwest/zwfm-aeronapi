# Aeron Image Manager API

Een **onofficiële** REST API voor het Aeron-radioautomatiseringssysteem, speciaal ontwikkeld voor het **toevoegen en beheren van afbeeldingen** voor tracks en artiesten.

> [!WARNING]
> Aeron is een product van Broadcast Partners. Deze API is volledig onofficieel en wordt niet ontwikkeld door of in samenwerking met Broadcast Partners. Gebruik is op eigen risico. Maak altijd eerst een back-up van je database voordat je deze tool gebruikt.

## Waarom deze API?

Het Aeron-systeem biedt standaard geen mogelijkheid om programmatisch afbeeldingen toe te voegen aan tracks en artiesten. Deze onofficiële API vult deze leemte door directe toegang tot de Aeron-database te bieden, waardoor je:
- **Automatisch albumhoezen kunt toevoegen** aan tracks
- **Artiestfoto's kunt beheren** voor een visueel aantrekkelijke weergave
- **Bulkbewerkingen kunt uitvoeren** om efficiënt grote aantallen afbeeldingen te verwerken

## Hoofdfunctionaliteit

De primaire functie van deze API is het **beheren van afbeeldingen** voor:
- **Artiesten**: Toevoegen en beheren van artiestfoto's
- **Tracks**: Toevoegen en beheren van albumhoezen

### Aanvullende functionaliteiten

- **Automatische optimalisatie**: Afbeeldingen worden automatisch geschaald en geoptimaliseerd voor gebruik in Aeron
- **Geavanceerde compressie**: Vergelijkt jpegli met standaard JPEG en selecteert automatisch de kleinste bestandsgrootte
- **Playlist-integratie**: Bekijk playlist-informatie inclusief de beschikbaarheid van afbeeldingen
- **Statistieken**: Monitor hoeveel artiesten en tracks reeds van afbeeldingen zijn voorzien
- **Bulkbewerkingen**: Verwijder alle afbeeldingen van een specifiek type in één handeling
- **Beveiliging**: Optionele API-sleutelverificatie voor veilige toegang

### Database onderhoud

- **Health monitoring**: Real-time inzicht in databaseprestaties, tabelstatistieken en bloat-detectie
- **Vacuum**: Voer VACUUM uit op tabellen om ruimte terug te winnen en prestaties te verbeteren
- **Analyze**: Werk tabelstatistieken bij voor de query optimizer
- **Aanbevelingen**: Automatische suggesties gebaseerd op configureerbare drempelwaarden

### Backup functionaliteit

- **Backup aanmaken**: Maak database backups in custom (binair) of plain (SQL) formaat
- **Backup beheer**: Lijst, download en verwijder backups via de API
- **Automatische opschoning**: Configureerbare retentie en maximum aantal backups

## Snelstart

### Docker (aanbevolen)
```bash
# Starten met voorbeeldconfiguratie
docker run -d -p 8080:8080 --name zwfm-aeronapi ghcr.io/oszuidwest/zwfm-aeronapi:latest

# Of met eigen configuratie
docker run -d -p 8080:8080 -v $(pwd)/config.yaml:/config.yaml --name zwfm-aeronapi ghcr.io/oszuidwest/zwfm-aeronapi:latest
```

### Uitvoerbare bestanden
Download uitvoerbare bestanden voor verschillende platformen via de [releases-pagina](https://github.com/oszuidwest/zwfm-aeronapi/releases):
- **Linux**: amd64, arm64, armv7
- **Windows**: amd64, arm64
- **macOS**: Intel (amd64), Apple Silicon (arm64)

## Installatie

### Met Docker

```bash
# Download voorbeeldconfiguratie
wget https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/config.example.yaml -O config.yaml
# Pas config.yaml aan naar jouw situatie

# Start de container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config.yaml \
  --name zwfm-aeronapi \
  --restart unless-stopped \
  ghcr.io/oszuidwest/zwfm-aeronapi:latest
```

### Met Docker (inclusief backups)

```bash
# Start de container met backup volume
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config.yaml \
  -v $(pwd)/backups:/backups \
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
# Bewerk config.yaml met jouw databasegegevens

# Compileer en start de applicatie
go build -o zwfm-aeronapi .
./zwfm-aeronapi -config=config.yaml -port=8080
```


## Configuratie

Kopieer `config.example.yaml` naar `config.yaml` en pas de waarden aan:

```bash
cp config.example.yaml config.yaml
```

Raadpleeg [`config.example.yaml`](config.example.yaml) voor:
- Volledige documentatie van alle configuratie-opties
- Voorbeelden voor verschillende omgevingen
- Gedetailleerde uitleg van elke instelling

### Belangrijke configuratie-opties

| Sectie | Optie | Beschrijving | Standaard |
|--------|-------|--------------|-----------|
| `database` | `max_open_conns` | Maximum aantal open database verbindingen | 25 |
| `database` | `max_idle_conns` | Maximum aantal idle verbindingen | 5 |
| `database` | `conn_max_lifetime` | Maximale levensduur verbinding (minuten) | 5 |
| `api` | `request_timeout` | Request timeout in seconden | 30 |
| `maintenance` | `bloat_threshold` | Bloat % drempel voor vacuum-aanbeveling | 10.0 |
| `maintenance` | `dead_tuple_threshold` | Dead tuple drempel voor vacuum-aanbeveling | 10000 |
| `backup` | `enabled` | Backup-endpoints inschakelen | false |
| `backup` | `path` | Directory voor backups | ./backups |
| `backup` | `retention_days` | Backups ouder dan dit automatisch verwijderen | 30 |
| `backup` | `max_backups` | Maximum aantal backups bewaren | 10 |

## De API-server starten

```bash
# Standaardpoort (8080)
./zwfm-aeronapi

# Aangepaste poort
./zwfm-aeronapi -port=9090

# Met aangepaste configuratie
./zwfm-aeronapi -config=/pad/naar/config.yaml -port=8080

# Versie-informatie tonen
./zwfm-aeronapi -version
```

## Snelstartgids

### 1. Start de API-server
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
  -H "X-API-Key: jouw-api-sleutel" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/artist.jpg"}'
```

### 4. Upload een albumhoes
```bash
curl -X POST http://localhost:8080/api/tracks/{trackid}/image \
  -H "X-API-Key: jouw-api-sleutel" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/album.jpg"}'
```

### 5. Bekijk database health
```bash
curl http://localhost:8080/api/db/health \
  -H "X-API-Key: jouw-api-sleutel"
```

### 6. Maak een backup (indien ingeschakeld)
```bash
curl -X POST http://localhost:8080/api/db/backup \
  -H "X-API-Key: jouw-api-sleutel" \
  -H "Content-Type: application/json" \
  -d '{"format":"custom"}'
```

Voor uitgebreide API-documentatie, inclusief alle endpoints, voorbeelden en gebruiksscenario's, raadpleeg [API.md](API.md).

## Databaseschema

Vereiste PostgreSQL-tabellen conform het Aeron-systeem:

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

### Systeemvereisten
- Go 1.25 of recenter
- PostgreSQL met Aeron-database

### Compileren
```bash
go mod download
go build -o zwfm-aeronapi .
```

### Testen
```bash
go test ./...
```

## Licentie

MIT-licentie - raadpleeg het LICENSE-bestand voor details.
