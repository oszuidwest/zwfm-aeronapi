# Aeron Image Manager API

Een **onofficiële** REST API voor het Aeron-radioautomatiseringssysteem, speciaal ontwikkeld voor het **toevoegen en beheren van afbeeldingen** voor tracks en artiesten.

> [!WARNING]
> Aeron is een product van Broadcast Partners. Deze API is volledig onofficieel en wordt niet ontwikkeld door of in samenwerking met Broadcast Partners. Gebruik is op eigen risico. Maak altijd eerst een back-up van je database voordat je deze tool gebruikt.

## Functionaliteiten

- **Afbeeldingenbeheer** - Upload en beheer albumhoezen en artiestfoto's
- **Automatische optimalisatie** - Afbeeldingen worden geschaald en gecomprimeerd naar JPEG
- **Playlist-integratie** - Bekijk playlists inclusief afbeeldingsstatus
- **Database onderhoud** - Health monitoring, vacuum en analyze operaties
- **Backup/restore** - Maak en beheer database backups

## Installatie

### Docker (aanbevolen)

```bash
# Download en pas configuratie aan
wget https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/config.example.json -O config.json

# Start container
docker run -d -p 8080:8080 \
  -v $(pwd)/config.json:/config.json \
  --name zwfm-aeronapi \
  --restart unless-stopped \
  ghcr.io/oszuidwest/zwfm-aeronapi:latest
```

### Binaries

Download voor je platform via de [releases-pagina](https://github.com/oszuidwest/zwfm-aeronapi/releases).

### Vanaf broncode

```bash
git clone https://github.com/oszuidwest/zwfm-aeronapi.git
cd zwfm-aeronapi
cp config.example.json config.json
go build -o zwfm-aeronapi .
./zwfm-aeronapi -config=config.json -port=8080
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

# Trackafbeelding uploaden
curl -X POST http://localhost:8080/api/tracks/{id}/image \
  -H "X-API-Key: jouw-api-sleutel" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/album.jpg"}'
```

Volledige API-documentatie: [API.md](API.md)

## Ontwikkeling

**Vereisten:** Go 1.25+, PostgreSQL

```bash
go build -o zwfm-aeronapi .
go test ./...
```

## Licentie

MIT-licentie - zie [LICENSE](LICENSE)
