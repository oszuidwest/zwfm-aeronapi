# Aeron Image Manager

Een command-line tool voor het beheer van afbeeldingen in Aeron databases.

> \[!WARNING]
> Aeron is een product van Broadcast Partners. Deze tool is niet officieel en is niet ontwikkeld in opdracht van of in samenwerking met Broadcast Partners. Gebruik ervan is volledig op eigen risico. Maak altijd eerst een backup van de database voordat je deze tool gebruikt.

## Functionaliteit

* Voegt afbeeldingen toe aan artiestenrecords in PostgreSQL database
* Optimaliseert afbeeldingen met Jpegli-encoder 
* Ondersteunt JPG, JPEG en PNG invoerbestanden
* Toont artiesten zonder afbeeldingen
* Zoekt artiesten op gedeeltelijke naam
* Verwijdert alle afbeeldingen tegelijk (nuke-functie)
* Dry-run modus voor het testen van operaties
* Vereist config.yaml bestand voor alle instellingen

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
./aeron-imgman -artist="OneRepublic" -url="https://example.com/image.jpg"

# Artiestafbeelding bijwerken via lokaal bestand
./aeron-imgman -artist="OneRepublic" -file="/pad/naar/image.jpg"

# Artiesten zonder afbeelding tonen
./aeron-imgman -list

# Zoeken naar artiesten met gedeeltelijke naam
./aeron-imgman -search="Chef"

# ALLE afbeeldingen uit database verwijderen (vereist bevestiging)
./aeron-imgman -nuke

# Versie-informatie tonen
./aeron-imgman -version

# Dry-run (voorbeeld zonder wijzigingen)
./aeron-imgman -artist="OneRepublic" -url="image.jpg" -dry-run
./aeron-imgman -nuke -dry-run
```

### Alle beschikbare opties

```bash
# Volledige lijst van opties
./aeron-imgman

# Opties:
  -artist string    Artiest naam om bij te werken (vereist)
  -url string       URL van de afbeelding om te downloaden
  -file string      Lokaal pad naar afbeelding
  -search string    Zoek artiesten met gedeeltelijke naam match
  -config string    Pad naar config bestand (standaard: config.yaml)
  -dry-run          Toon wat gedaan zou worden zonder bij te werken
  -list             Toon artiesten zonder afbeeldingen
  -nuke             Verwijder ALLE afbeeldingen (vereist bevestiging)
  -version          Toon versie-informatie
```

### Nuke modus

De `-nuke` functie verwijdert **alle** artiestafbeeldingen uit de database in één keer. Deze functie is bedoeld voor situaties waarin je met een frisse start wil beginnen. Hierbij zijn een aantal veiligheidsmaatregelen

* **Voorvertoning**: Toont eerst hoeveel en welke artiesten geraakt worden
* **Bevestiging vereist**: Je moet expliciet "VERWIJDER ALLES" typen om door te gaan
* **Dry-run ondersteuning**: Test de functie zonder veranderingen doort te voeren met `-dry-run`

> **⚠️ WAARSCHUWING**: De nuke-functie is onomkeerbaar. Maak altijd eerst een backup van je database!

### Configuratie

#### Configuratiebestand gebruiken

```bash
# Gebruikt standaard config.yaml in huidige directory
./aeron-imgman -artist="Naam Artiest" -url="image.jpg"

# Of aangepaste configuratie gebruiken
./aeron-imgman -config="/pad/naar/config.yaml" -artist="Naam Artiest" -url="image.jpg"
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
* Maximum bestandsgrootte: Configureerbaar via config.yaml (bijvoorbeeld 200 MB)

### Encoding optimalisatie

De tool gebruikt een slimme aanpak voor bestandscompressie:

1. **Dubbele encoding**: Elke afbeelding wordt geëncodeerd met zowel Jpegli als de standaard Go JPEG encoder
2. **Automatische selectie**: De encoder met de kleinste bestandsgrootte wordt automatisch gekozen
3. **Rapportage**: De tool toont welke encoder werd gebruikt en hoeveel ruimte werd bespaard
4. **Geen kwaliteitsverlies**: Beide encoders gebruiken dezelfde kwaliteitsinstellingen

Dit zorgt ervoor dat je altijd de best mogelijke compressie krijgt, ongeacht het type bronafbeelding.

## Vereisten

* Go versie 1.24 of hoger
* PostgreSQL-database met artiesttabel (Aeron-database)
* Netwerktoegang voor downloaden van afbeeldingen bij gebruik van webadresssen

## Databaseschema

De tool vereist de volgende PostgreSQL-tabel zoals gebruikt door Aeron:

```sql
CREATE TABLE {schema}.artist (
    artistid UUID PRIMARY KEY,
    artist VARCHAR NOT NULL,
    picture BYTEA
);
```

## Over dit project

Dit project ondersteunt het beheer en optimaliseren van artiestafbeeldingen in Aeron-databases. Het is compatibel met de nieuwste versie van de AerOn Studio radio-automatisering.

## Licentie

MIT-licentie – zie LICENSE-bestand voor details.
