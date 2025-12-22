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
| **Backup-management** | Maak, download en beheer database backups |

## Installatie

### Docker (aanbevolen)

```bash
# Download en pas configuratie aan
wget https://raw.githubusercontent.com/oszuidwest/zwfm-aerontoolbox/main/config.example.json -O config.json

# Start container
docker run -d -p 8080:8080 \
  -v $(pwd)/config.json:/config.json \
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
| `backup` | Backup-instellingen (pad, retentie) |

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
