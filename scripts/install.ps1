$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$Repository = "pablo/gutil"
$InstallDir = if ($env:GUTIL_INSTALL_DIR) { $env:GUTIL_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$Version = $env:GUTIL_VERSION
if (-not $Version) {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repository/releases/latest"
    $Version = $Release.tag_name
}

$Architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
$Arch = switch ($Architecture) {
    "ARM64" { "arm64" }
    "X64" { "amd64" }
    "AMD64" { "amd64" }
    default { throw "Unsupported architecture: $Architecture" }
}

$Archive = "gutil_${Version}_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repository/releases/download/$Version"
$TempDir = Join-Path ([IO.Path]::GetTempPath()) ("gutil-" + [Guid]::NewGuid())

try {
    New-Item -ItemType Directory -Path $TempDir | Out-Null
    Invoke-WebRequest "$BaseUrl/$Archive" -OutFile (Join-Path $TempDir $Archive)
    Invoke-WebRequest "$BaseUrl/checksums.txt" -OutFile (Join-Path $TempDir "checksums.txt")

    $ChecksumLine = Get-Content (Join-Path $TempDir "checksums.txt") | Where-Object { $_ -match [regex]::Escape($Archive) } | Select-Object -First 1
    if (-not $ChecksumLine) { throw "No checksum found for $Archive." }
    $Expected = ($ChecksumLine -split '\s+')[0].ToLowerInvariant()
    $Actual = (Get-FileHash (Join-Path $TempDir $Archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected -ne $Actual) { throw "Checksum verification failed for $Archive." }

    Expand-Archive (Join-Path $TempDir $Archive) -DestinationPath $TempDir
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $TemporaryTarget = Join-Path $InstallDir "gutil.exe.new"
    Move-Item -Force (Join-Path $TempDir "gutil.exe") $TemporaryTarget
    Move-Item -Force $TemporaryTarget (Join-Path $InstallDir "gutil.exe")

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Entries = @($UserPath -split ';' | Where-Object { $_ })
    if ($Entries -notcontains $InstallDir) {
        $NewPath = (($Entries + $InstallDir) -join ';')
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Host "Added $InstallDir to the user PATH. Open a new terminal."
    }
    Write-Host "Installed gUtil $Version to $InstallDir\gutil.exe"
}
finally {
    if (Test-Path $TempDir) { Remove-Item -Recurse -Force $TempDir }
}
