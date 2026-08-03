$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$ProgressPreference = "SilentlyContinue"

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
}
catch {
    Write-Host "Warning: could not explicitly enable TLS 1.2; continuing with the system defaults." -ForegroundColor Yellow
}

function Invoke-GUtilDownload {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$OutFile,
        [Parameter(Mandatory = $true)][string]$Description
    )

    Write-Host "$Description..."
    try {
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing -TimeoutSec 60
    }
    catch {
        throw "$Description failed: $($_.Exception.Message). Check access to github.com and try again."
    }
}

$Repository = "pabloLopezSanchezz/gutil"
$InstallDir = if ($env:GUTIL_INSTALL_DIR) { $env:GUTIL_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$Version = $env:GUTIL_VERSION
if (-not $Version) {
    Write-Host "Checking the latest gUtil release..."
    try {
        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest" -UseBasicParsing -TimeoutSec 30
        $Version = $Release.tag_name
    }
    catch {
        throw "Could not determine the latest gUtil version: $($_.Exception.Message). Check access to api.github.com and try again."
    }
}

$Architecture = $env:PROCESSOR_ARCHITEW6432
if (-not $Architecture) {
    $Architecture = $env:PROCESSOR_ARCHITECTURE
}
if (-not $Architecture) {
    throw "Could not determine the Windows processor architecture."
}

$Arch = switch ($Architecture.ToUpperInvariant()) {
    "ARM64" { "arm64" }
    "X64" { "amd64" }
    "AMD64" { "amd64" }
    default { throw "Unsupported architecture: $Architecture" }
}

$Archive = "gutil_${Version}_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repository/releases/download/$Version"
$TempDir = Join-Path ([IO.Path]::GetTempPath()) ("gutil-" + [Guid]::NewGuid())

try {
    Write-Host "Preparing gUtil $Version for Windows $Architecture..."
    New-Item -ItemType Directory -Path $TempDir | Out-Null
    Invoke-GUtilDownload -Uri "$BaseUrl/$Archive" -OutFile (Join-Path $TempDir $Archive) -Description "Downloading gUtil $Version"
    Invoke-GUtilDownload -Uri "$BaseUrl/checksums.txt" -OutFile (Join-Path $TempDir "checksums.txt") -Description "Downloading checksums"

    Write-Host "Verifying checksum..."
    $ChecksumLine = Get-Content (Join-Path $TempDir "checksums.txt") | Where-Object { $_ -match [regex]::Escape($Archive) } | Select-Object -First 1
    if (-not $ChecksumLine) { throw "No checksum found for $Archive." }
    $Expected = ($ChecksumLine -split '\s+')[0].ToLowerInvariant()
    $Actual = (Get-FileHash (Join-Path $TempDir $Archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected -ne $Actual) { throw "Checksum verification failed for $Archive." }

    Write-Host "Installing gUtil to $InstallDir..."
    Expand-Archive (Join-Path $TempDir $Archive) -DestinationPath $TempDir -Force
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $TemporaryTarget = Join-Path $InstallDir "gutil.exe.new"
    Move-Item -Force (Join-Path $TempDir "gutil.exe") $TemporaryTarget
    Move-Item -Force $TemporaryTarget (Join-Path $InstallDir "gutil.exe")

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Entries = @($UserPath -split ';' | Where-Object { $_ })
    if ($Entries -notcontains $InstallDir) {
        $NewPath = (($Entries + $InstallDir) -join ';')
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Host "Added $InstallDir to the user PATH."
    }

    $CurrentEntries = @($env:Path -split ';' | Where-Object { $_ })
    if ($CurrentEntries -notcontains $InstallDir) {
        $env:Path = "$InstallDir;$env:Path"
    }

    Write-Host "Verifying the installed command..."
    $InstalledBinary = Join-Path $InstallDir "gutil.exe"
    $InstalledVersion = & $InstalledBinary version
    if ($LASTEXITCODE -ne 0) {
        throw "The downloaded executable returned exit code $LASTEXITCODE during verification."
    }
    Write-Host "Installed and verified $InstalledVersion at $InstalledBinary" -ForegroundColor Green
    Write-Host "You can now run: gutil version"
}
finally {
    if (Test-Path $TempDir) { Remove-Item -Recurse -Force $TempDir }
}
