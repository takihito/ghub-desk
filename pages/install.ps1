# ghub-desk installer for Windows
# Usage: irm https://takihito.github.io/ghub-desk/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = "takihito/ghub-desk"
$InstallDir = if ($env:GHUB_DESK_INSTALL_DIR) { $env:GHUB_DESK_INSTALL_DIR } else { "$env:LOCALAPPDATA\ghub-desk\bin" }

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x86_64" }
} else {
    Write-Error "32-bit systems are not supported"; exit 1
}

# Get latest version
$Version = $env:GHUB_DESK_VERSION
if (-not $Version) {
    Write-Host "Fetching latest version..."
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $Release.tag_name
}
Write-Host "Version: $Version"

# Normalize version tag: ensure leading 'v' for release URL
$VersionTag = if ($Version.StartsWith('v')) { $Version } else { "v$Version" }

# Strip leading 'v' for artifact name (tag: v0.2.3 -> artifact: 0.2.3)
$VersionNum = $VersionTag.TrimStart('v')

# Download
$Artifact = "ghub-desk_${VersionNum}_Windows_${Arch}.tar.gz"
$Checksums = "checksums.txt"
$BaseUrl = "https://github.com/$Repo/releases/download/$VersionTag"
$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "ghub-desk-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "Downloading $Artifact..."
    Invoke-WebRequest -Uri "$BaseUrl/$Artifact" -OutFile "$TmpDir\$Artifact"
    Invoke-WebRequest -Uri "$BaseUrl/$Checksums" -OutFile "$TmpDir\$Checksums"

    # Verify checksum
    Write-Host "Verifying checksum..."
    $Expected = (Get-Content "$TmpDir\$Checksums" | Select-String "  $Artifact$").ToString().Split(" ")[0].ToLower()
    $Actual = (Get-FileHash "$TmpDir\$Artifact" -Algorithm SHA256).Hash.ToLower()
    if ($Expected -ne $Actual) {
        Write-Error "Checksum mismatch: expected $Expected, got $Actual"
        exit 1
    }

    # Extract and install (requires tar, available on Windows 10 1903+)
    Write-Host "Installing to $InstallDir..."
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    tar -xzf "$TmpDir\$Artifact" -C $TmpDir ghub-desk.exe
    Copy-Item "$TmpDir\ghub-desk.exe" -Destination "$InstallDir\ghub-desk.exe" -Force

    # Add to PATH if not already present
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to PATH (restart your terminal to apply)"
    }

    Write-Host ""
    Write-Host "ghub-desk $Version installed successfully!"
    Write-Host "Run 'ghub-desk version' to verify."
} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
