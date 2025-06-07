# Aeron Afbeelding Batchprocessor

Een commandline-tool voor het optimaliseren en verwerken van artiestafbeeldingen voor de Aeron-database van Streekomroep ZuidWest.

> \[!WARNING]
> Aeron is een product van Broadcast Partners. Deze tool is niet officieel en is niet ontwikkeld in opdracht van of in samenwerking met Broadcast Partners. Gebruik ervan is volledig op eigen risico. Maak altijd eerst een backup van de database voordat je deze tool gebruikt.

## Kenmerken

* Volledig geschreven in Go, zonder externe afhankelijkheden
* Optimaliseert afbeeldingen met Google's Jpegli-encoder
* Ondersteunt invoerformaten: JPG, JPEG, PNG
* Verkleint afbeeldingen groter dan 640x640 px; weigert kleinere afbeeldingen
* Rechtstreekse toegang tot PostgreSQL-database
* Hoofdlettergevoelige artiestnaamvergelijking
* Dry-run-modus om wijzigingen vooraf te bekijken

## Installatie

```bash
git clone https://github.com/streekomroep-zuidwest/aeron-imgbatch.git
cd aeron-imgbatch
go mod tidy
go build -o aeron-imgbatch .
```

## Gebruik

### Basiscommando's

```bash
# Artiestafbeelding bijwerken via URL
./aeron-imgbatch -artist="OneRepublic" -url="https://example.com/image.jpg"

# Artiestafbeelding bijwerken via lokaal bestand
./aeron-imgbatch -artist="OneRepublic" -file="/pad/naar/image.jpg"

# Artiesten zonder afbeelding tonen
./aeron-imgbatch -list

# Status optimalisatietools tonen
./aeron-imgbatch -tools

# Dry-run (voorbeeld zonder wijzigingen)
./aeron-imgbatch -artist="OneRepublic" -url="image.jpg" -dry-run
```

### Configuratie

#### Configuratiebestand gebruiken

```bash
# Gebruikt standaard config.yaml in huidige directory
./aeron-imgbatch -artist="Naam Artiest" -url="image.jpg"

# Of aangepaste configuratie gebruiken
./aeron-imgbatch -config="/pad/naar/config.yaml" -artist="Naam Artiest" -url="image.jpg"
```

#### Commandline-opties

```bash
./aeron-imgbatch \
  -db-host=localhost \
  -db-port=5432 \
  -db-name=aeron_db \
  -db-user=aeron_user \
  -db-password=wachtwoord \
  -db-schema=aeron \
  -artist="Naam Artiest" \
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

./aeron-imgbatch -artist="Naam Artiest" -url="image.jpg"
```

## Configuratiebestand

Maak een bestand `config.yaml` aan:

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

## Afbeeldingverwerking

* Doelafmeting: 640x640 pixels
* Kwaliteit: 90 (instelbaar)
* Encoder: Jpegli, met terugval naar standaard Go JPEG
* Uitvoerformaat: altijd JPEG, ongeacht invoerformaat
* Schaling: afbeeldingen groter dan doelafmetingen worden verkleind
* Validatie: kleinere afbeeldingen worden geweigerd (instelbaar)

## Vereisten

* Go versie 1.24 of hoger
* PostgreSQL-database met artiesttabel (Aeron-database)
* Netwerktoegang voor downloaden van afbeeldingen bij gebruik van URLs

## Databaseschema

De tool vereist de volgende PostgreSQL-tabel zoals gebruikt door Aeron:

```sql
CREATE TABLE {schema}.artist (
    artistid UUID PRIMARY KEY,
    artist VARCHAR NOT NULL,
    picture BYTEA
);
```

## Foutafhandeling

* Niet-gevonden artiest: hoofdlettergevoelige vergelijking vereist
* Ongeldige afbeeldingen: automatische validatie met duidelijke foutmeldingen
* Databasefouten: duidelijke meldingen inclusief verbindingsdetails
* Bestandsgrootte: maximaal 20 MB per invoerbestand

## Over dit project

Dit project is ontwikkeld door Streekomroep ZuidWest voor gebruik met de nieuwste versie van het Aeron-broadcastsysteem. Het ondersteunt het beheer en optimaliseren van artiestafbeeldingen in de Aeron-database.

## Licentie

MIT-licentie – zie LICENSE-bestand voor details.
