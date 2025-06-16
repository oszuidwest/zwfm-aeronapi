# ZuidWest FM Aeron API - Universele Windows Installer
# Dit script kan de service installeren, upgraden of verwijderen
# 
# Gebruik:
#   Installeren:   irm https://raw.githubusercontent.com/oszuidwest/zwfm-aeronapi/main/install.ps1 | iex
#   Upgraden:      .\install.ps1 -Upgrade
#   Verwijderen:   .\install.ps1 -Remove [-RemoveFiles]

param(
    [switch]$Upgrade,
    [switch]$Remove,
    [switch]$RemoveFiles,
    [string]$InstallPath = "$env:ProgramFiles\zwfm-aeronapi",
    [string]$Port = "8080"
)

$ErrorActionPreference = "Stop"
$ServiceName = "zwfm-aeronapi"
$DisplayName = "ZuidWest FM Aeron API"
$Description = "REST API voor Aeron afbeeldingenbeheer"
$RepoOwner = "oszuidwest"
$RepoName = "zwfm-aeronapi"

# Helper functions
function Write-Info { Write-Host $args[0] -ForegroundColor Cyan }
function Write-Success { Write-Host $args[0] -ForegroundColor Green }
function Write-Error { Write-Host $args[0] -ForegroundColor Red }
function Write-Warning { Write-Host $args[0] -ForegroundColor Yellow }

# Controleer of script als admin draait
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (-not $isAdmin) {
    Write-Error "Dit script vereist Administrator rechten!"
    Write-Host "Start PowerShell als Administrator en probeer opnieuw."
    exit 1
}

Write-Host "`nZuidWest FM Aeron API - Windows Installer`n" -ForegroundColor Green

# Verwijderen afhandelen
if ($Remove) {
    Write-Warning "ZuidWest FM Aeron API wordt verwijderd..."
    
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -eq 'Running') {
            Write-Info "Service stoppen..."
            Stop-Service -Name $ServiceName -Force
            Start-Sleep -Seconds 2
        }
        
        Write-Info "Service verwijderen..."
        sc.exe delete $ServiceName | Out-Null
        Write-Success "Service verwijderd!"
    }
    
    if ($RemoveFiles -and (Test-Path $InstallPath)) {
        Write-Info "Installatiebestanden verwijderen..."
        Remove-Item -Path $InstallPath -Recurse -Force
        Write-Success "Bestanden verwijderd!"
    }
    
    Write-Success "`nVerwijderen voltooid!"
    exit 0
}

# Haal laatste release info op
Write-Info "Laatste release informatie ophalen..."
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
    $version = $release.tag_name
    Write-Success "Laatste versie: $version"
} catch {
    Write-Error "Kon release informatie niet ophalen!"
    Write-Host "Fout: $_"
    exit 1
}

# Zoek Windows amd64 bestand
$asset = $release.assets | Where-Object { $_.name -like "*windows*amd64*.exe" -or $_.name -like "*windows*x64*.exe" } | Select-Object -First 1
if (-not $asset) {
    Write-Error "Geen Windows executable gevonden in laatste release!"
    exit 1
}

$downloadUrl = $asset.browser_download_url
$exeName = "zwfm-aeronapi.exe"

# Controleer of upgrade nodig is
if ($Upgrade) {
    if (-not (Test-Path (Join-Path $InstallPath $exeName))) {
        Write-Error "Geen bestaande installatie gevonden in $InstallPath"
        Write-Host "Voer uit zonder -Upgrade voor een nieuwe installatie."
        exit 1
    }
    
    Write-Info "Service stoppen voor upgrade..."
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
}

# Maak installatie directory
if (-not (Test-Path $InstallPath)) {
    Write-Info "Installatie directory aanmaken..."
    New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
}

# Download executable
Write-Info "$version downloaden..."
$exePath = Join-Path $InstallPath $exeName
try {
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $downloadUrl -OutFile $exePath
    Write-Success "Download voltooid!"
} catch {
    Write-Error "Download mislukt!"
    Write-Host "Fout: $_"
    exit 1
}

# Configuratie bestand afhandelen
$configPath = Join-Path $InstallPath "config.yaml"
if (-not (Test-Path $configPath)) {
    Write-Info "Voorbeeld configuratie aanmaken..."
    
    # Download config.example.yaml
    $configUrl = "https://raw.githubusercontent.com/$RepoOwner/$RepoName/main/config.example.yaml"
    try {
        Invoke-WebRequest -Uri $configUrl -OutFile $configPath
        Write-Warning "`nBELANGRIJK: Pas config.yaml aan met je database gegevens!"
        Write-Host "config.yaml wordt geopend..."
        Start-Process notepad.exe -ArgumentList $configPath -Wait
    } catch {
        Write-Error "Kon voorbeeld configuratie niet downloaden!"
        Write-Host "Maak config.yaml handmatig aan."
    }
}

# Service aanmaken of updaten
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($service) {
    if ($Upgrade) {
        Write-Info "Service bestaat al, wordt gestart na upgrade..."
    } else {
        Write-Info "Bestaande service updaten..."
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }
}

if (-not $Upgrade -or -not $service) {
    Write-Info "Windows service aanmaken..."
    $binPath = "`"$exePath`" -config=`"$configPath`" -port=$Port"
    
    $result = sc.exe create $ServiceName `
        binPath= $binPath `
        DisplayName= "$DisplayName" `
        start= auto
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Service aanmaken mislukt!"
        exit 1
    }
    
    # Service configureren
    sc.exe description $ServiceName "$Description" | Out-Null
    sc.exe failure $ServiceName reset= 86400 actions= restart/60000/restart/60000// | Out-Null
}

# Logs directory aanmaken
New-Item -ItemType Directory -Force -Path (Join-Path $InstallPath "logs") | Out-Null

# Service starten
Write-Info "Service starten..."
Start-Service -Name $ServiceName -ErrorAction Stop

# Eindcontrole
Start-Sleep -Seconds 2
$service = Get-Service -Name $ServiceName
if ($service.Status -eq 'Running') {
    Write-Success "`n✅ Installatie geslaagd!`n"
    Write-Host "Versie        : $version"
    Write-Host "Installatie   : $InstallPath"
    Write-Host "Service naam  : $ServiceName"
    Write-Host "API endpoint  : http://localhost:$Port/api/health"
    Write-Host "Configuratie  : $configPath`n"
    
    # Test API
    Write-Info "API testen..."
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:$Port/api/health" -Method Get
        Write-Success "API reageert! Status: $($response.data.status)"
    } catch {
        Write-Warning "API reageert nog niet. Controleer logs in: $(Join-Path $InstallPath 'logs')"
    }
    
    Write-Host "`nHandige commando's:" -ForegroundColor Yellow
    Write-Host "  Get-Service $ServiceName"
    Write-Host "  Restart-Service $ServiceName"
    Write-Host "  .\install.ps1 -Upgrade              # Upgrade naar laatste versie"
    Write-Host "  .\install.ps1 -Remove               # Verwijder service"
    Write-Host "  .\install.ps1 -Remove -RemoveFiles  # Verwijder alles`n"
} else {
    Write-Error "Service installatie voltooid maar service draait niet!"
    Write-Host "Controleer Event Viewer voor fouten."
}