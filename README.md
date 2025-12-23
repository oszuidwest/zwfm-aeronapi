# Aeron Toolbox

Een **onofficiële** REST API voor het Aeron-radioautomatiseringssysteem met tools voor afbeeldingenbeheer, mediabrowser, database-onderhoud en backups.

> [!WARNING]
> Aeron is een product van Broadcast Partners. Deze API is volledig onofficieel en wordt niet ontwikkeld door of in samenwerking met Broadcast Partners. Gebruik is op eigen risico. Maak altijd eerst een back-up van je database voordat je deze tool gebruikt.

## Functionaliteiten

| Module | Beschrijving |
|--------|--------------|
| **Afbeeldingenbeheer** | Upload, optimaliseer en beheer albumhoezen en artiestfoto's |
| **Mediabrowser** | Bekijk artiesten, tracks en playlists met uitgebreide metadata |
| **Database-onderhoud** | Health monitoring, VACUUM en ANALYZE operaties |
| **Backup-management** | Maak, valideer, download en beheer database backups |

## Systeemvereisten

| Vereiste | Beschrijving |
|----------|--------------|
| **Go 1.25+** | Alleen nodig bij bouwen vanaf broncode |
| **PostgreSQL client tools** | `pg_dump` en `pg_restore` - alleen vereist als backup functionaliteit is ingeschakeld |

De PostgreSQL client tools worden bij het opstarten gevalideerd wanneer `backup.enabled: true`. Zonder deze tools weigert de applicatie te starten met een duidelijke foutmelding.

**Installatie PostgreSQL client tools:**
```bash
# Debian/Ubuntu
apt-get install postgresql-client

# Alpine Linux (Docker)
apk add postgresql16-client

# macOS
brew install libpq

# Windows
# Installeer via PostgreSQL installer of: choco install postgresql
```

## Installatie

### Docker (aanbevolen)

**Met Docker Compose:**
```bash
# Download configuratiebestanden
wget https://raw.githubusercontent.com/oszuidwest/zwfm-aerontoolbox/main/config.example.json -O config.json
wget https://raw.githubusercontent.com/oszuidwest/zwfm-aerontoolbox/main/docker-compose.example.yml -O docker-compose.yml

# Pas config.json aan en start
docker compose up -d
```

**Of met Docker run:**
```bash
docker run -d -p 8080:8080 \
  -v $(pwd)/config.json:/app/config.json:ro \
  -v $(pwd)/backups:/backups \
  --name zwfm-aerontoolbox \
  --restart unless-stopped \
  ghcr.io/oszuidwest/zwfm-aerontoolbox:latest
```

### Binaries

Download voor je platform via de [releases-pagina](https://github.com/oszuidwest/zwfm-aerontoolbox/releases).

### Vanaf broncode

```bash
git clone https://github.com/oszuidwest/zwfm-aerontoolbox.git
cd zwfm-aerontoolbox
cp config.example.json config.json
go build -o zwfm-aerontoolbox .
./zwfm-aerontoolbox -config=config.json -port=8080
```

## Configuratie

Kopieer `config.example.json` naar `config.json` en configureer:

| Sectie | Beschrijving |
|--------|--------------|
| `database` | PostgreSQL verbinding (host, port, user, password, schema) |
| `image` | Afbeeldingsoptimalisatie (afmetingen, kwaliteit) |
| `api` | Authenticatie (API-sleutels) |
| `backup` | Backup-instellingen (pad, retentie, scheduler, S3 sync) |
| `log` | Logging-instellingen (level, format) |

Zie [`config.example.json`](config.example.json) voor alle opties en standaardwaarden.

## Gebruik

```bash
# Health check
curl http://localhost:8080/api/health

# Artiestafbeelding uploaden
curl -X POST http://localhost:8080/api/artists/{id}/image \
  -H "X-API-Key: jouw-api-sleutel" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/artist.jpg"}'

# Database backup maken
curl -X POST http://localhost:8080/api/db/backup \
  -H "X-API-Key: jouw-api-sleutel"
```

Volledige API-documentatie: [API.md](API.md)

## Ontwikkeling

**Vereisten:** Go 1.25+, PostgreSQL

```bash
go build -o zwfm-aerontoolbox .
go test ./...
```

## Licentie

MIT-licentie - zie [LICENSE](LICENSE)
