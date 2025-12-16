# Hubble Network Smart Installer

Cross-platform installer for Hubble Network developer boards. Flash Nordic and Silicon Labs boards in under 30 seconds.

## Quick Start

### One-Line Install

**macOS/Linux:**

```bash
curl -s get.hubble.com | bash
```

**Windows (PowerShell as Administrator):**

```powershell
iex (irm https://get.hubble.com)
```

The installer will:
1. Detect your OS and architecture automatically
2. Download the appropriate binary
3. Run the installer immediately
4. Clean up after completion

> **Note for Windows users:** You must run PowerShell as Administrator. Right-click PowerShell and select "Run as administrator".

## Security

### Verifying Downloads

All binaries are:
- Built from this open-source repository using [GoReleaser](https://goreleaser.com/)
- Published as GitHub Releases with checksums
- Downloaded over HTTPS

To verify a binary manually:
1. Download the binary and checksum file from [Releases](https://github.com/HubbleNetwork/hubble-install/releases)
2. Verify the checksum matches: `sha256sum -c checksums.txt`

### What the Installer Does

The installer requires network access and elevated permissions to:
- **Download dependencies** (uv, segger-jlink) via your system's package manager
- **Flash firmware** to your connected developer board via USB
- **Communicate with Hubble APIs** to fetch the appropriate firmware for your board

The installer does **not**:
- Collect telemetry or usage data
- Modify system files outside of standard package manager locations
- Run background processes after completion

### Credential Handling

Your Hubble credentials (Org ID and API Token) are:
- Passed directly to the board flashing tool
- Never stored on disk
- Never transmitted except to official Hubble APIs over HTTPS

For automated environments, use environment variables instead of interactive prompts:

```bash
export HUBBLE_ORG_ID="your-org-id"
export HUBBLE_API_TOKEN="your-api-token"
hubble-install
```

### Installing from the Hubble Dashboard

When you copy the install command from the [Hubble Dashboard](https://dash.hubble.com), your credentials are included as a base64-encoded string:

```bash
curl -s get.hubble.com | bash -s <base64-credentials>
```

This encoding:
- Combines your Org ID and API Token in the format `org_id:api_token`
- Is **not encryption** — it simply encodes the credentials for safe URL/shell transport
- Is decoded locally by the installer and never sent to any third party
- Can be decoded with: `echo "<base64-string>" | base64 -d`

If the credentials cannot be validated (invalid format or incomplete paste), the installer will prompt you to either retry or enter credentials manually.

## Supported Developer Boards

### Nordic Semiconductor
- nRF21540 DK
- nRF52840 DK

### Texas Instruments
- TI CC2340R5 Launchpad
- TI CC2340R53 Launchpad

## What It Does

The installer will:

1. 🔍 **Detect your operating system** and architecture
2. 🔑 **Prompt for your Hubble credentials** (Org ID & API Token)
3. 🎯 **Let you select your developer board** from the supported list
4. 📦 **Install required dependencies:**
   - **macOS**: Homebrew, uv, segger-jlink
   - **Linux**: uv, segger-jlink
   - **Windows**: Chocolatey, uv, nrfjprog
5. ⚡ **Flash your board** with the appropriate firmware
6. ✅ **Verify the installation** was successful

**Total time: < 30 seconds** (after dependencies are installed)

## Package Management

The installer uses your platform's standard package manager to install dependencies. If the package manager isn't already installed, the installer will set it up for you. If you prefer not to use a package manager, see [Manual Dependency Installation](#manual-dependency-installation) below.

### macOS — Homebrew

[Homebrew](https://brew.sh/) is the standard package manager for macOS.

```bash
# The installer runs these commands:
brew install astral-sh/tap/uv
brew install --cask segger-jlink
```

### Linux — apt, dnf, or yum

The installer detects your distribution and uses the appropriate package manager.

```bash
# uv is installed via the official installer:
curl -LsSf https://astral.sh/uv/install.sh | sh

# SEGGER J-Link must be downloaded from segger.com
# The installer will guide you through this process
```

### Windows — Chocolatey

[Chocolatey](https://chocolatey.org/) is a package manager for Windows. The installer will set it up if not present.

```powershell
# The installer runs these commands:
choco install uv -y
choco install nrfjprog -y  # Includes SEGGER J-Link
```

> **Note:** Windows installation requires Administrator privileges for Chocolatey to function properly.

### Manual Dependency Installation

If you prefer not to use a package manager, you can install the dependencies manually:

**uv** (all platforms):

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

Or on Windows PowerShell:

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"
```

**SEGGER J-Link** (all platforms):

Download and install from the official SEGGER website:
👉 https://www.segger.com/downloads/jlink/

Select the appropriate installer for your platform:
- macOS: `JLink_MacOSX_Vxxx_universal.pkg`
- Linux: `JLink_Linux_Vxxx_x86_64.deb` (or `.rpm` for Fedora/RHEL)
- Windows: `JLink_Windows_Vxxx.exe`

## Getting Your Credentials

Get your Hubble Org ID and API Token from:
👉 https://dash.hubble.com/developer/api-tokens

## Manual Installation

### Download Pre-built Binary

1. Download the appropriate binary for your platform from [Releases](https://github.com/HubbleNetwork/hubble-install/releases)
2. Make it executable (macOS/Linux): `chmod +x hubble-install-*`
3. Run it: `./hubble-install-*`

### Build from Source

**Prerequisites:**
- Go 1.21 or later

```bash
# Clone the repository
git clone https://github.com/HubbleNetwork/hubble-install.git
cd hubble-install

# Build
go build -o hubble-install .

# Run
./hubble-install
```

## Command Line Options

```bash
hubble-install [flags]

```

## Dependencies

The installer automatically installs these runtime dependencies:

| Dependency | Purpose |
|------------|---------|
| [uv](https://github.com/astral-sh/uv) | Fast Python package installer |
| [segger-jlink](https://www.segger.com/products/debug-probes/j-link/) | SEGGER J-Link tools for board flashing |

## Troubleshooting

### macOS: "Permission denied" when installing Homebrew
This is expected. Enter your password when prompted.

### Windows: "Administrator privileges required"
Right-click PowerShell and select "Run as administrator" before running the installer.

### Board flashing fails
- Ensure you're using a data-capable USB cable (not charge-only)
- Verify your Org ID and API Token are correct
- Check that the board is properly connected
- Try a different USB port

### Dependencies not found after installation
Restart your terminal to refresh your PATH.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- 📚 Documentation: https://docs.hubble.com
- 💬 GitHub Issues: https://github.com/HubbleNetwork/hubble-install/issues

---

Made with 🛰️ by Hubble Network
