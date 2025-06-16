# Windows Installatie Handleiding

Deze handleiding beschrijft hoe je zwfm-aeronapi installeert op Windows, met name op een server waar Aeron draait.

## Snelstart

Open PowerShell als **Administrator** en voer uit:

```powershell
irm https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/install.ps1 | iex
```

Dit installeert automatisch de laatste versie. De installer:
- Downloadt de laatste release van GitHub
- Maakt de installatie directory aan
- Installeert als Windows Service
- Kopieert voorbeeld configuratie
- Opent config.yaml voor bewerking
- Start de service

## Handmatige Installatie

### Vereisten
- Windows 10/11 of Windows Server 2016+
- PowerShell 5.1+ (standaard aanwezig)
- Administrator rechten (voor service installatie)

### Stap 1: Bestanden voorbereiden

1. Maak installatie directory:
   ```powershell
   New-Item -Path "C:\Program Files\zwfm-aeronapi" -ItemType Directory
   ```

2. Kopieer de executable naar deze map

3. Maak configuratie bestand:
   ```powershell
   cd "C:\Program Files\zwfm-aeronapi"
   copy config.example.yaml config.yaml
   # Bekijk het voorbeeld voor alle opties:
   type config.example.yaml
   # Bewerk de configuratie:
   notepad config.yaml
   ```

### Stap 2: Configuratie aanpassen

Bewerk `config.yaml` met je Aeron database gegevens. Zie [`config.example.yaml`](config.example.yaml) voor een volledig voorbeeld met uitleg.

**Belangrijke instellingen voor Windows/Aeron integratie:**
- `database.host`: Gebruik `localhost` als Aeron op dezelfde server draait
- `database.name`: Meestal `aeron_db` of vergelijkbaar
- `api.enabled`: Zet op `true` voor productie
- `api.keys`: Genereer veilige API keys (tip in PowerShell: `[System.Web.Security.Membership]::GeneratePassword(32,8)`)

### Stap 3: Installeren

```powershell
# Download en run installer
irm https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/install.ps1 | iex

# Of download eerst voor inspectie:
Invoke-WebRequest https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/install.ps1 -OutFile install.ps1
.\install.ps1
```

## Service Beheer

### Upgraden naar laatste versie
```powershell
.\install.ps1 -Upgrade
```

### Status controleren
```powershell
# Service status
Get-Service zwfm-aeronapi

# API health check
Invoke-RestMethod http://localhost:8080/api/health | ConvertTo-Json
```

### Service beheren
```powershell
# Stoppen
Stop-Service zwfm-aeronapi

# Starten
Start-Service zwfm-aeronapi

# Herstarten
Restart-Service zwfm-aeronapi
```

### Logs bekijken
```powershell
# Windows Event logs
Get-EventLog -LogName Application -Source zwfm-aeronapi -Newest 20

# Of in Event Viewer:
# Windows Logs → Application → Filter op "zwfm-aeronapi"
```

## Verwijderen

```powershell
# Alleen service verwijderen (bestanden blijven)
.\install.ps1 -Remove

# Service + alle bestanden verwijderen
.\install.ps1 -Remove -RemoveFiles
```

## Troubleshooting

### Service start niet

1. Test eerst handmatig:
   ```powershell
   cd "C:\Program Files\zwfm-aeronapi"
   .\zwfm-aeronapi.exe -config=config.yaml -port=8080
   ```

2. Controleer configuratie:
   - Database connectie gegevens kloppen
   - PostgreSQL accepteert connecties
   - Firewall staat connectie toe

3. Check logs:
   ```powershell
   Get-EventLog -LogName Application -Source zwfm-aeronapi -Newest 10
   ```

### API reageert niet

1. Check of service draait:
   ```powershell
   Get-Service zwfm-aeronapi
   ```

2. Test health endpoint:
   ```powershell
   Invoke-RestMethod http://localhost:8080/api/health
   ```

3. Windows Firewall:
   ```powershell
   # Firewall regel toevoegen
   New-NetFirewallRule -DisplayName "ZWFM Aeron API" `
     -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
   ```

## Integratie met Aeron

Typische setup op een Aeron server:

1. Installeer zwfm-aeronapi op dezelfde server als Aeron
2. Gebruik `localhost` als database host
3. Gebruik dezelfde database credentials als Aeron
4. API draait op poort 8080 (of andere vrije poort)

## Beveiliging

1. **API Keys**: Gebruik altijd sterke API keys in productie
2. **Firewall**: Beperk toegang tot de API poort
3. **HTTPS**: Overweeg een reverse proxy met SSL voor externe toegang
4. **Database**: Gebruik alleen read/write rechten, geen admin rechten

## Install Script Opties

De `install.ps1` ondersteunt:

| Optie | Beschrijving |
|-------|--------------|
| *(geen)* | Nieuwe installatie |
| `-Upgrade` | Upgrade naar laatste versie |
| `-Remove` | Verwijder service |
| `-Remove -RemoveFiles` | Verwijder alles |
| `-Port 9090` | Gebruik andere poort |
| `-InstallPath "C:\custom"` | Aangepaste installatie locatie |