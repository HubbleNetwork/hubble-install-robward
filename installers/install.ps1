# Hubble Network Installer Download and Run Script for Windows
# Usage: 
#   With credentials: iex "& { $(irm https://get.hubble.com) } <base64-credentials>"
#   Without credentials: iex "& { $(irm https://get.hubble.com) }"

param(
    [string]$Credentials = ""
)

# Set error action preference
$ErrorActionPreference = "Stop"

# Accept credentials as parameter (base64 encoded org_id:api_key)
if ($Credentials) {
    $ValidationFailed = $false
    
    try {
        # Validate base64 format and decode
        $DecodedBytes = [System.Convert]::FromBase64String($Credentials)
        $DecodedString = [System.Text.Encoding]::UTF8.GetString($DecodedBytes)
        
        # Validate format (should contain a colon)
        if (-not $DecodedString.Contains(':')) {
            $ValidationFailed = $true
        }
    } catch {
        $ValidationFailed = $true
    }
    
    if ($ValidationFailed) {
        Write-Host ""
        Write-Host "⚠️  We were unable to validate your credentials." -ForegroundColor Yellow
        Write-Host ""
        Write-Host "You can either:"
        Write-Host "  • Exit and check that you pasted the complete command correctly"
        Write-Host "  • Continue and enter your credentials manually"
        Write-Host ""
        $Response = Read-Host "Would you like to exit and try again? (Y/n)"
        if ([string]::IsNullOrEmpty($Response) -or $Response -match '^[Yy]') {
            Write-Host "Please check your command and run the installer again."
            exit 1
        }
        Write-Host "Continuing - you'll be prompted for credentials..."
        Write-Host ""
    } else {
        $env:HUBBLE_CREDENTIALS = $Credentials
        Write-Host "✓ Credentials provided" -ForegroundColor Green
    }
}

$InstallUrl = "https://github.com/HubbleNetwork/hubble-install/releases/latest/download"
$BinaryName = "hubble-install-windows-amd64.exe"

Write-Host "🛰️  Hubble Network Installer" -ForegroundColor Cyan
Write-Host "=============================="
Write-Host ""

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

# Currently only supporting amd64
if ($Arch -ne "amd64") {
    Write-Host "❌ Error: Only 64-bit Windows is supported" -ForegroundColor Red
    exit 1
}

Write-Host "✓ Detected platform: Windows/$Arch" -ForegroundColor Green
Write-Host "📥 Downloading installer..." -ForegroundColor Cyan
Write-Host ""

# Create temp directory
$TempDir = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TempDir | Out-Null

$TempBinary = Join-Path $TempDir $BinaryName
$TempChecksums = Join-Path $TempDir "checksums.txt"

$BinaryUrl = "$InstallUrl/$BinaryName"
$ChecksumUrl = "$InstallUrl/checksums.txt"

try {
    # Use TLS 1.2 for secure connection
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    
    # Download the binary
    $ProgressPreference = 'SilentlyContinue'
    try {
        Invoke-WebRequest -Uri $BinaryUrl -OutFile $TempBinary -UseBasicParsing
        Write-Host "✓ Binary downloaded" -ForegroundColor Green
    } catch {
        Write-Host "❌ Download failed from GitHub Releases" -ForegroundColor Red
        Write-Host "   URL: $BinaryUrl"
        exit 1
    }
    
    # Download checksums
    try {
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $TempChecksums -UseBasicParsing
        Write-Host "✓ Checksums downloaded" -ForegroundColor Green
    } catch {
        Write-Host "❌ Failed to download checksums" -ForegroundColor Red
        exit 1
    }
    
    $ProgressPreference = 'Continue'
    
    # Verify checksum
    Write-Host "🔒 Verifying checksum..." -ForegroundColor Cyan
    
    # Calculate SHA256 hash of downloaded binary
    $Hash = Get-FileHash -Path $TempBinary -Algorithm SHA256
    $CalculatedHash = $Hash.Hash.ToLower()
    
    # Read checksums file and find our binary's expected hash
    $ChecksumsContent = Get-Content $TempChecksums
    $ExpectedHash = $null
    foreach ($Line in $ChecksumsContent) {
        if ($Line -match "^([a-f0-9]+)\s+$BinaryName") {
            $ExpectedHash = $Matches[1].ToLower()
            break
        }
    }
    
    if (-not $ExpectedHash) {
        Write-Host "❌ Checksum not found for $BinaryName" -ForegroundColor Red
        exit 1
    }
    
    if ($CalculatedHash -ne $ExpectedHash) {
        Write-Host "❌ Checksum verification failed!" -ForegroundColor Red
        Write-Host "   This could indicate a corrupted download or security issue."
        Write-Host "   Expected: $ExpectedHash"
        Write-Host "   Got:      $CalculatedHash"
        exit 1
    }
    
    Write-Host "✓ Checksum verified" -ForegroundColor Green
    Write-Host ""
    Write-Host "🚀 Running installer..." -ForegroundColor Cyan
    Write-Host ""
    
    # Check if running as administrator
    $IsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    
    if (-not $IsAdmin) {
        Write-Host "⚠️  Administrator privileges required" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Attempting to restart with administrator privileges..."
        Write-Host "Please accept the UAC prompt to continue."
        Write-Host ""
        
        # Restart with admin privileges
        Start-Process -FilePath $TempBinary -Verb RunAs -Wait
    } else {
        # Run the installer directly
        & $TempBinary
    }
    
} catch {
    Write-Host "❌ Installation failed: $_" -ForegroundColor Red
    exit 1
} finally {
    # Clean up
    if (Test-Path $TempDir) {
        Remove-Item $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
