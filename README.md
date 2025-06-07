# Aeron Afbeelding Batch Processor

Een command-line tool voor het optimaliseren en verwerken van artiest afbeeldingen voor de Aeron database van Streekomroep ZuidWest.

## Functies

- Pure Go implementatie zonder externe afhankelijkheden
- Afbeelding optimalisatie met Google's Jpegli encoder
- Ondersteunt JPG, JPEG, PNG invoerformaten
- Verkleint afbeeldingen groter dan 640x640px, weigert kleinere afbeeldingen
- Directe PostgreSQL database toegang
- Hoofdlettergevoelige artiest naam matching
- Dry run modus voor het bekijken van wijzigingen

## Installatie

```bash
git clone https://github.com/streekomroep-zuidwest/aeron-imgbatch.git
cd aeron-imgbatch
go mod tidy
go build -o aeron-imgbatch .
```

## Gebruik

### Basis Commando's

```bash
# Artiest afbeelding bijwerken vanuit URL
./aeron-imgbatch -artist="OneRepublic" -url="https://example.com/image.jpg"

# Artiest afbeelding bijwerken vanuit lokaal bestand
./aeron-imgbatch -artist="OneRepublic" -file="/pad/naar/image.jpg"

# Artiesten zonder afbeeldingen tonen
./aeron-imgbatch -list

# Optimalisatie tools status tonen
./aeron-imgbatch -tools

# Dry run (voorvertoning zonder wijzigingen)
./aeron-imgbatch -artist="OneRepublic" -url="image.jpg" -dry-run
```

### Configuratie

#### Config bestand gebruiken
```bash
# Gebruikt config.yaml uit huidige directory standaard
./aeron-imgbatch -artist="Artiest Naam" -url="image.jpg"

# Of specificeer aangepaste config
./aeron-imgbatch -config="/pad/naar/config.yaml" -artist="Artiest" -url="image.jpg"
```

#### Command line opties
```bash
./aeron-imgbatch \
  -db-host=localhost \
  -db-port=5432 \
  -db-name=aeron_db \
  -db-user=aeron_user \
  -db-password=wachtwoord \
  -db-schema=aeron \
  -artist="Artiest Naam" \
  -url="image.jpg"
```

#### Omgevingsvariabelen
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=aeron_db
export DB_USER=aeron_user
export DB_PASSWORD=wachtwoord
export DB_SCHEMA=aeron

./aeron-imgbatch -artist="Artiest Naam" -url="image.jpg"
```

## Configuratie Bestand

Maak een `config.yaml` bestand:

```yaml
database:
  host: localhost
  port: 5432
  name: aeron_db
  user: aeron_user
  password: aeron_password
  schema: aeron
  sslmode: disable

image:
  target_width: 640
  target_height: 640
  quality: 90
  use_jpegli: true
  max_file_size_mb: 20
  reject_smaller: true
```

## Afbeelding Verwerking

- Doelgrootte: 640x640 pixels
- Kwaliteit: 90 (configureerbaar)
- Encoder: Jpegli met terugval naar standaard Go JPEG
- Formaat: Alle invoer geconverteerd naar JPEG uitvoer
- Vergroting/verkleining: Grotere afbeeldingen worden verkleind naar doelgrootte
- Validatie: Kleinere afbeeldingen geweigerd (configureerbaar)

## Vereisten

- Go 1.24+
- PostgreSQL database met artiest tabel (Aeron database)
- Netwerk toegang voor het downloaden van afbeeldingen (bij gebruik van URLs)

## Database Schema

De tool verwacht een PostgreSQL tabel zoals gebruikt in Aeron:

```sql
CREATE TABLE {schema}.artist (
    artistid UUID PRIMARY KEY,
    artist VARCHAR NOT NULL,
    picture BYTEA
);
```

## Fout Afhandeling

- Artiest niet gevonden: Exacte hoofdlettergevoelige naam matching vereist
- Ongeldige afbeeldingen: Automatische validatie en foutrapportage
- Database fouten: Duidelijke foutmeldingen met verbindingsdetails
- Bestandsgrootte limieten: 20MB maximum invoer bestandsgrootte

## Over dit Project

Dit project is ontwikkeld door Streekomroep ZuidWest om te werken met de laatste versie van het Aeron broadcast systeem. Het helpt bij het beheren en optimaliseren van artiest afbeeldingen in de Aeron database.

## Licentie

MIT Licentie - Zie LICENSE bestand voor details.