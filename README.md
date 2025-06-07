# Aeron Image Manager

Een command-line tool voor het beheer van afbeeldingen in Aeron databases.

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
git clone https://github.com/oszuidwest/aeron-imgman.git
cd aeron-imgman
go mod tidy
go build -o aeron-imgman .
```

### Pre-built Releases

Je kunt ook pre-compiled binaries downloaden van de [Releases pagina](https://github.com/oszuidwest/aeron-imgman/releases):

- **Linux**: amd64, arm64, armv7
- **Windows**: amd64, arm64  
- **macOS**: Intel (amd64), Apple Silicon (arm64)

Download het juiste bestand voor jouw platform, maak het uitvoerbaar (`chmod +x`) en gebruik het direct.

## Gebruik

### Basiscommando's

```bash
# Artiestafbeelding bijwerken via URL
./aeron-imgman -artist="OneRepublic" -url="https://example.com/image.jpg"

# Artiestafbeelding bijwerken via lokaal bestand
./aeron-imgman -artist="OneRepublic" -file="/pad/naar/image.jpg"

# Artiesten zonder afbeelding tonen
./aeron-imgman -list

# ALLE afbeeldingen uit database verwijderen (vereist bevestiging)
./aeron-imgman -nuke

# Status optimalisatietools tonen
./aeron-imgman -tools

# Versie-informatie tonen
./aeron-imgman -version

# Dry-run (voorbeeld zonder wijzigingen)
./aeron-imgman -artist="OneRepublic" -url="image.jpg" -dry-run
./aeron-imgman -nuke -dry-run
```

### Afbeeldingen verwijderen

De `-nuke` functie verwijdert **alle** artiestafbeeldingen uit de database in één keer. Deze functie is bedoeld voor situaties waarin je de complete afbeeldingencollectie wilt opschonen.

#### Veiligheidsmaatregelen

* **Voorvertoning**: Toont eerst hoeveel en welke artiesten geraakt worden
* **Bevestiging vereist**: Je moet expliciet "VERWIJDER ALLES" typen om door te gaan
* **Dry-run ondersteuning**: Test de functie veilig met `-dry-run`

#### Voorbeeld van nuke-modus

```bash
# Veilig testen zonder wijzigingen
./aeron-imgman -nuke -dry-run

# Uitvoer:
# WAARSCHUWING: Deze actie zal ALLE afbeeldingen verwijderen van 647 artiesten:
#
#   ABBA (ID: c1d6b3db-a26b-43f6-97e2-e9519db0c520)
#   Adele (ID: 3ecd5284-9f11-40ba-b854-fc2e081f21dd)
#   ...en 645 meer artiesten
#
# Totaal: 647 artiesten zullen hun afbeelding verliezen.
# DRY RUN: Zou alle afbeeldingen verwijderen maar doet dit niet daadwerkelijk

# Daadwerkelijk uitvoeren (vereist bevestiging)
./aeron-imgman -nuke
```

> **⚠️ WAARSCHUWING**: De nuke-functie is onomkeerbaar. Maak altijd eerst een backup van je database!

### Configuratie

#### Configuratiebestand gebruiken

```bash
# Gebruikt standaard config.yaml in huidige directory
./aeron-imgman -artist="Naam Artiest" -url="image.jpg"

# Of aangepaste configuratie gebruiken
./aeron-imgman -config="/pad/naar/config.yaml" -artist="Naam Artiest" -url="image.jpg"
```

#### Commandline-opties

```bash
./aeron-imgman \
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

./aeron-imgman -artist="Naam Artiest" -url="image.jpg"
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

Dit project ondersteunt het beheer en optimaliseren van artiestafbeeldingen in Aeron-databases. Het is compatibel met de nieuwste versie van de AerOn Studio radio-automatisering.

## Licentie

MIT-licentie – zie LICENSE-bestand voor details.
