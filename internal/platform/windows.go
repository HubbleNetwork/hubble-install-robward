package platform

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/HubbleNetwork/hubble-install/internal/ui"
)

// WindowsInstaller implements the Installer interface for Windows
type WindowsInstaller struct{}

// RebootRequiredError is returned when a system reboot is required
type RebootRequiredError struct {
	Message string
}

func (e *RebootRequiredError) Error() string {
	return e.Message
}

// NewWindowsInstaller creates a new Windows installer
func NewWindowsInstaller() *WindowsInstaller {
	return &WindowsInstaller{}
}

// Name returns the platform name
func (w *WindowsInstaller) Name() string {
	return "Windows"
}

// CheckPendingReboot checks if Windows has a pending reboot
func (w *WindowsInstaller) CheckPendingReboot() error {
	// Use PowerShell to check for pending reboot indicators
	// This is more reliable than checking registry directly and works cross-platform
	psScript := `
		$rebootPending = $false
		$reasons = @()

		# Check Component Based Servicing
		if (Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending') {
			$rebootPending = $true
			$reasons += "Component Based Servicing"
		}

		# Check Windows Update
		if (Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired') {
			$rebootPending = $true
			$reasons += "Windows Update"
		}

		# Check Pending File Rename Operations
		$pfro = Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager' -Name 'PendingFileRenameOperations' -ErrorAction SilentlyContinue
		if ($pfro -and $pfro.PendingFileRenameOperations) {
			$rebootPending = $true
			$reasons += "Pending File Rename Operations"
		}

		# Check Pending Computer Rename
		$computerName = Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\ComputerName\ActiveComputerName' -ErrorAction SilentlyContinue
		$pendingComputerName = Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\ComputerName\ComputerName' -ErrorAction SilentlyContinue
		if ($computerName -and $pendingComputerName -and ($computerName.ComputerName -ne $pendingComputerName.ComputerName)) {
			$rebootPending = $true
			$reasons += "Computer Rename"
		}

		if ($rebootPending) {
			Write-Output "REBOOT_PENDING:$($reasons -join ',')"
		} else {
			Write-Output "NO_REBOOT"
		}
	`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		// If PowerShell fails, assume no reboot is pending
		// This prevents blocking installation if PowerShell has issues
		return nil
	}

	result := strings.TrimSpace(string(output))
	if strings.HasPrefix(result, "REBOOT_PENDING:") {
		reasons := strings.TrimPrefix(result, "REBOOT_PENDING:")
		return &RebootRequiredError{
			Message: fmt.Sprintf("pending reboot detected (%s)", reasons),
		}
	}

	return nil
}

// ensureAdminAccess checks if running with administrator privileges
func (w *WindowsInstaller) ensureAdminAccess() error {
	// Check if we have admin rights by trying to access a protected registry key
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		ui.PrintError("Administrator access required")
		ui.PrintInfo("Please run this installer as Administrator:")
		ui.PrintInfo("  Right-click the executable and select 'Run as administrator'")
		return fmt.Errorf("administrator privileges required")
	}
	return nil
}

// CheckPrerequisites checks for missing dependencies based on required deps
func (w *WindowsInstaller) CheckPrerequisites(requiredDeps []string) ([]MissingDependency, error) {
	var missing []MissingDependency

	// Check for Chocolatey (always required for installing other deps)
	if !w.commandExists("choco") {
		missing = append(missing, MissingDependency{
			Name:   "Chocolatey",
			Status: "Not installed",
		})
	}

	// Check each required dependency
	for _, dep := range requiredDeps {
		switch dep {
		case "uv":
			if !w.commandExists("uv") {
				missing = append(missing, MissingDependency{
					Name:   "uv",
					Status: "Not installed",
				})
			}
		case "nrfutil":
			// Check for Nordic nrfutil (preferred over nrfjprog/J-Link installer)
			if !w.nrfutilInstalled() {
				missing = append(missing, MissingDependency{
					Name:   "nrfutil",
					Status: "Not installed",
				})
			}
		}
	}

	return missing, nil
}

// downloadFile downloads a file from a URL to a destination path with progress indication
func (w *WindowsInstaller) downloadFile(url, destPath string) error {
	ui.PrintInfo(fmt.Sprintf("Downloading from %s...", url))

	// Create the file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	// Get the data
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	ui.PrintSuccess("Download complete")
	return nil
}

// installJLinkFromSEGGER downloads and installs J-Link from SEGGER's official installer
func (w *WindowsInstaller) installJLinkFromSEGGER() error {
	ui.PrintInfo("Installing SEGGER J-Link from official installer...")
	ui.PrintInfo("This may take a few minutes...")

	// Use a recent stable version
	// Format: https://www.segger.com/downloads/jlink/JLink_Windows_V794l.exe
	jlinkVersion := "V794l" // Update this periodically
	jlinkURL := fmt.Sprintf("https://www.segger.com/downloads/jlink/JLink_Windows_%s.exe", jlinkVersion)

	// Create temp directory for download
	tempDir := filepath.Join(os.TempDir(), "hubble-jlink-install")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after installation

	installerPath := filepath.Join(tempDir, "JLink_Installer.exe")

	// Download the installer
	if err := w.downloadFile(jlinkURL, installerPath); err != nil {
		ui.PrintWarning("Failed to download J-Link installer automatically")
		ui.PrintInfo("You can download it manually from: https://www.segger.com/downloads/jlink/")
		return fmt.Errorf("download failed: %w", err)
	}

	// Run the installer silently
	// SEGGER J-Link installer options for unattended installation:
	// Try multiple silent installation methods as SEGGER versions vary
	ui.PrintInfo("Running silent installer (this will take a few minutes)...")
	ui.PrintInfo("Accepting SEGGER license agreement automatically...")

	// Method 1: NSIS-style with license acceptance
	cmd := exec.Command(installerPath, "/S", "/ACCEPTLICENSE=yes")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Method 1 failed, try Method 2: Alternative flags
		ui.PrintWarning("First installation method failed, trying alternative...")
		cmd = exec.Command(installerPath, "/q", "/norestart", "ACCEPTLICENSE=yes")
		if err2 := cmd.Run(); err2 != nil {
			// Both methods failed
			ui.PrintError("Silent installation failed")
			ui.PrintInfo("The installer may require manual intervention")
			ui.PrintInfo("Alternative: Download and run manually from https://www.segger.com/downloads/jlink/")
			return fmt.Errorf("installer failed with both methods: %w / %v", err, err2)
		}
	}

	// Wait for installation to fully complete and verify
	// NSIS installers can spawn child processes
	ui.PrintInfo("Verifying installation...")

	jlinkPaths := []string{
		`C:\Program Files\SEGGER\JLink\JLink.exe`,
		`C:\Program Files (x86)\SEGGER\JLink\JLink.exe`,
	}

	// Poll for up to 60 seconds for the installation to complete
	maxWaitTime := 60 * time.Second
	checkInterval := 2 * time.Second
	elapsed := time.Duration(0)
	installed := false

	for elapsed < maxWaitTime {
		for _, path := range jlinkPaths {
			if _, err := os.Stat(path); err == nil {
				installed = true
				// Add to PATH for current process
				jlinkDir := filepath.Dir(path)
				currentPath := os.Getenv("PATH")
				if !strings.Contains(currentPath, jlinkDir) {
					os.Setenv("PATH", jlinkDir+";"+currentPath)
				}
				break
			}
		}

		if installed {
			break
		}

		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	if !installed {
		return fmt.Errorf("J-Link installation completed but JLink.exe not found in expected locations after %v", maxWaitTime)
	}

	ui.PrintSuccess("SEGGER J-Link installed successfully")
	return nil
}

// InstallPackageManager installs Chocolatey if not present
func (w *WindowsInstaller) InstallPackageManager() error {
	if w.commandExists("choco") {
		ui.PrintSuccess("Chocolatey already installed")
		return nil
	}

	// Ensure we have admin access
	if err := w.ensureAdminAccess(); err != nil {
		return err
	}

	ui.PrintInfo("Installing Chocolatey...")
	ui.PrintInfo("This may take a few minutes...")

	// Run the official Chocolatey installation script
	// Using PowerShell with execution policy bypass for the installation
	installScript := `Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))`

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", installScript)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Chocolatey: %w", err)
	}

	// Add Chocolatey to PATH for this process
	if err := w.setupChocoPath(); err != nil {
		return fmt.Errorf("chocolatey installation completed but could not find choco binary: %w", err)
	}

	// Verify that choco is actually working
	if !w.commandExists("choco") {
		return fmt.Errorf("chocolatey installation completed but choco command not found in PATH")
	}

	// Test choco with a simple command to ensure it's functional
	testCmd := exec.Command("choco", "--version")
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("chocolatey installed but not functioning correctly: %w", err)
	}

	ui.PrintSuccess("Chocolatey installed successfully")
	return nil
}

// InstallDependencies installs the specified dependencies
func (w *WindowsInstaller) InstallDependencies(deps []string) error {
	// First ensure Chocolatey is installed
	if !w.commandExists("choco") {
		if err := w.InstallPackageManager(); err != nil {
			return err
		}
	}

	// Ensure we have admin access for package installation
	if err := w.ensureAdminAccess(); err != nil {
		return err
	}

	// Install dependencies
	for _, dep := range deps {
		switch dep {
		case "uv":
			// Install uv via Chocolatey
			if w.commandExists("uv") {
				ui.PrintSuccess("uv already installed")
			} else {
				ui.PrintInfo("Installing uv...")
				if err := w.runChocoInstall("uv", true); err != nil {
					return fmt.Errorf("failed to install uv: %w", err)
				}
				// Update PATH to include uv location
				if err := w.setupUVPath(); err != nil {
					ui.PrintWarning(fmt.Sprintf("Could not update PATH for uv: %v", err))
				}
				ui.PrintSuccess("uv installed successfully")
			}

		case "nrfutil":
			if w.nrfutilInstalled() {
				ui.PrintSuccess("nrfutil already installed")
				break
			}

			ui.PrintInfo("Installing Nordic nrfutil (standalone binary)...")
			if err := w.installNRFUtil(); err != nil {
				return fmt.Errorf("failed to install nrfutil: %w", err)
			}
			ui.PrintSuccess("nrfutil installed successfully")
		}
	}

	return nil
}

// FlashBoard flashes the specified board using uvx (for J-Link boards)
func (w *WindowsInstaller) FlashBoard(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
	ui.PrintInfo(fmt.Sprintf("Flashing board: %s", board))
	ui.PrintInfo("This may take 10-15 seconds...")

	// Try to find uv executable
	uvPath, err := w.findUVPath()
	if err != nil {
		fmt.Println()
		ui.PrintError("Could not locate the 'uv' executable")
		fmt.Println()
		ui.PrintInfo("This usually happens because:")
		ui.PrintInfo("  1. The PATH environment variable hasn't been updated in this session")
		ui.PrintInfo("  2. A system reboot may be required")
		fmt.Println()
		ui.PrintInfo("To fix this:")
		ui.PrintInfo("  1. Close this terminal/PowerShell window")
		ui.PrintInfo("  2. Open a NEW terminal/PowerShell window")
		ui.PrintInfo("  3. Run this installer again")
		fmt.Println()
		ui.PrintInfo("If that doesn't work, try rebooting your computer and running again.")
		fmt.Println()
		return nil, fmt.Errorf("uv executable not found: %w", err)
	}

	// Build the command with --refresh to prevent stale versions
	args := []string{"tool", "run", "--refresh", "--from", "pyhubbledemo", "hubbledemo", "flash", board, "-o", orgID, "-t", apiToken}
	if deviceName != "" {
		args = append(args, "-n", deviceName)
	}
	cmd := exec.Command(uvPath, args...)

	cmd.Env = append(os.Environ(), "PYTHONWARNINGS=ignore")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Check if this is a network-related error
		errStr := err.Error()
		if strings.Contains(errStr, "dns error") ||
			strings.Contains(errStr, "No such host") ||
			strings.Contains(errStr, "client error") ||
			strings.Contains(errStr, "Failed to download") {
			fmt.Println()
			ui.PrintError("Network connectivity error during flashing")
			fmt.Println()
			ui.PrintInfo("The flashing tool failed to download required files from the internet.")
			fmt.Println()
			ui.PrintInfo("Possible causes:")
			ui.PrintInfo("  • Network connectivity issues")
			ui.PrintInfo("  • Corporate firewall or proxy blocking GitHub")
			ui.PrintInfo("  • DNS resolution problems")
			ui.PrintInfo("  • Antivirus or security software blocking downloads")
			fmt.Println()
			ui.PrintInfo("Troubleshooting steps:")
			ui.PrintInfo("  1. Check your internet connection")
			ui.PrintInfo("  2. Try accessing https://github.com in a browser")
			ui.PrintInfo("  3. If behind a corporate firewall, configure proxy settings:")
			ui.PrintInfo("     $env:HTTP_PROXY = 'http://proxy.company.com:8080'")
			ui.PrintInfo("     $env:HTTPS_PROXY = 'http://proxy.company.com:8080'")
			ui.PrintInfo("  4. Temporarily disable antivirus/firewall and try again")
			ui.PrintInfo("  5. Try again in a few minutes (GitHub may be temporarily unavailable)")
			fmt.Println()
		}
		return nil, fmt.Errorf("flash command failed: %w", err)
	}

	resultDeviceName := deviceName
	if resultDeviceName == "" {
		resultDeviceName = "your-device"
	}

	ui.PrintSuccess(fmt.Sprintf("Board %s flashed successfully!", board))
	return &FlashResult{DeviceName: resultDeviceName}, nil
}

// GenerateHexFile generates a hex file for Uniflash boards (TI)
func (w *WindowsInstaller) GenerateHexFile(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
	ui.PrintInfo(fmt.Sprintf("Generating hex file for board: %s", board))
	ui.PrintInfo("This may take a few seconds...")

	// Try to find uv executable
	uvPath, err := w.findUVPath()
	if err != nil {
		fmt.Println()
		ui.PrintError("Could not locate the 'uv' executable")
		fmt.Println()
		ui.PrintInfo("This usually happens because:")
		ui.PrintInfo("  1. The PATH environment variable hasn't been updated in this session")
		ui.PrintInfo("  2. A system reboot may be required")
		fmt.Println()
		ui.PrintInfo("To fix this:")
		ui.PrintInfo("  1. Close this terminal/PowerShell window")
		ui.PrintInfo("  2. Open a NEW terminal/PowerShell window")
		ui.PrintInfo("  3. Run this installer again")
		fmt.Println()
		ui.PrintInfo("If that doesn't work, try rebooting your computer and running again.")
		fmt.Println()
		return nil, fmt.Errorf("uv executable not found: %w", err)
	}

	// Determine hex file path in current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Use device name for filename if provided, otherwise use board name
	filename := board + ".hex"
	if deviceName != "" {
		filename = deviceName + ".hex"
	}
	hexFilePath := filepath.Join(currentDir, filename)

	// Build the command with --refresh to prevent stale versions and -f for output file
	args := []string{"tool", "run", "--refresh", "--from", "pyhubbledemo", "hubbledemo", "flash", board, "-o", orgID, "-t", apiToken, "-f", hexFilePath}
	if deviceName != "" {
		args = append(args, "-n", deviceName)
	}
	cmd := exec.Command(uvPath, args...)

	cmd.Env = append(os.Environ(), "PYTHONWARNINGS=ignore")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Check if this is a network-related error
		errStr := err.Error()
		if strings.Contains(errStr, "dns error") ||
			strings.Contains(errStr, "No such host") ||
			strings.Contains(errStr, "client error") ||
			strings.Contains(errStr, "Failed to download") {
			fmt.Println()
			ui.PrintError("Network connectivity error during hex file generation")
			fmt.Println()
			ui.PrintInfo("The tool failed to download required files from the internet.")
			fmt.Println()
			ui.PrintInfo("Possible causes:")
			ui.PrintInfo("  • Network connectivity issues")
			ui.PrintInfo("  • Corporate firewall or proxy blocking GitHub")
			ui.PrintInfo("  • DNS resolution problems")
			ui.PrintInfo("  • Antivirus or security software blocking downloads")
			fmt.Println()
			ui.PrintInfo("Troubleshooting steps:")
			ui.PrintInfo("  1. Check your internet connection")
			ui.PrintInfo("  2. Try accessing https://github.com in a browser")
			ui.PrintInfo("  3. If behind a corporate firewall, configure proxy settings:")
			ui.PrintInfo("     $env:HTTP_PROXY = 'http://proxy.company.com:8080'")
			ui.PrintInfo("     $env:HTTPS_PROXY = 'http://proxy.company.com:8080'")
			ui.PrintInfo("  4. Temporarily disable antivirus/firewall and try again")
			ui.PrintInfo("  5. Try again in a few minutes (GitHub may be temporarily unavailable)")
			fmt.Println()
		}
		return nil, fmt.Errorf("command failed: %w", err)
	}

	return &FlashResult{HexFilePath: hexFilePath}, nil
}

// Helper functions

// commandExists checks if a command is available in PATH
func (w *WindowsInstaller) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// setupChocoPath adds Chocolatey to PATH for the current process
func (w *WindowsInstaller) setupChocoPath() error {
	// Get Chocolatey install path from environment variable
	chocoInstall := os.Getenv("ChocolateyInstall")
	if chocoInstall == "" {
		// Fall back to default location
		chocoInstall = `C:\ProgramData\chocolatey`
	}

	chocoPath := filepath.Join(chocoInstall, "bin")

	if _, err := os.Stat(chocoPath); os.IsNotExist(err) {
		return fmt.Errorf("choco not found in expected location: %s", chocoPath)
	}

	// Update PATH for this process
	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, chocoPath) {
		newPath := chocoPath + ";" + currentPath
		os.Setenv("PATH", newPath)
	}

	return nil
}

// runChocoInstall runs a choco install command using the full path to choco.exe
func (w *WindowsInstaller) runChocoInstall(pkg string, showOutput bool) error {
	// Get Chocolatey install path from environment variable
	chocoInstall := os.Getenv("ChocolateyInstall")
	if chocoInstall == "" {
		chocoInstall = `C:\ProgramData\chocolatey`
	}

	// Use full path to avoid PATH lookup issues after fresh Chocolatey install
	chocoExe := filepath.Join(chocoInstall, "bin", "choco.exe")

	cmd := exec.Command(chocoExe, "install", pkg, "-y")

	// Show output if requested
	if showOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	if err != nil {
		// Exit code 3010 means "success, but reboot required"
		// This is a special case that requires user action
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 3010 {
				return &RebootRequiredError{
					Message: fmt.Sprintf("installation of %s requires a system reboot", pkg),
				}
			}
		}
		return err
	}

	return nil
}

// findUVPath attempts to locate the uv executable using multiple methods
func (w *WindowsInstaller) findUVPath() (string, error) {
	// Method 1: Try standard PATH lookup
	if uvPath, err := exec.LookPath("uv"); err == nil {
		return uvPath, nil
	}

	// Method 2: Check Chocolatey bin directory (where shims are)
	chocoInstall := os.Getenv("ChocolateyInstall")
	if chocoInstall == "" {
		chocoInstall = `C:\ProgramData\chocolatey`
	}

	chocoBin := filepath.Join(chocoInstall, "bin", "uv.exe")
	if _, err := os.Stat(chocoBin); err == nil {
		return chocoBin, nil
	}

	// Method 3: Search Chocolatey lib directory for uv installation
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`$uvLib = Get-ChildItem -Path "%s\lib" -Filter "uv*" -Directory | Select-Object -First 1; if ($uvLib) { $uvExe = Get-ChildItem -Path $uvLib.FullName -Filter "uv.exe" -Recurse | Select-Object -First 1; if ($uvExe) { Write-Output $uvExe.FullName } }`, chocoInstall))
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		uvPath := strings.TrimSpace(string(output))
		if uvPath != "" {
			if _, err := os.Stat(uvPath); err == nil {
				return uvPath, nil
			}
		}
	}

	// Method 4: Check common installation locations
	commonPaths := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "uv", "uv.exe"),
		filepath.Join(os.Getenv("USERPROFILE"), ".local", "bin", "uv.exe"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("uv executable not found in any expected location")
}

// setupUVPath adds uv to PATH for the current process after Chocolatey installation
func (w *WindowsInstaller) setupUVPath() error {
	// Get Chocolatey install path from environment variable
	chocoInstall := os.Getenv("ChocolateyInstall")
	if chocoInstall == "" {
		// Fall back to default location
		chocoInstall = `C:\ProgramData\chocolatey`
	}

	// Find uv tools directory using PowerShell
	// Get-ChildItem -Path "$env:ChocolateyInstall\lib" | Where-Object Name -Like "uv*"
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`(Get-ChildItem -Path "%s\lib" | Where-Object Name -Like "uv*" | Select-Object -First 1).FullName`, chocoInstall))
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		uvLibPath := strings.TrimSpace(string(output))
		if uvLibPath != "" {
			uvToolsPath := filepath.Join(uvLibPath, "tools")
			if _, err := os.Stat(uvToolsPath); err == nil {
				currentPath := os.Getenv("PATH")
				if !strings.Contains(currentPath, uvToolsPath) {
					os.Setenv("PATH", uvToolsPath+";"+currentPath)
				}
			}
		}
	}

	// Also ensure Chocolatey bin is in PATH (where shims live)
	chocoBin := filepath.Join(chocoInstall, "bin")
	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, chocoBin) {
		os.Setenv("PATH", chocoBin+";"+currentPath)
	}

	return nil
}

// nrfutilInstalled checks for nrfutil on PATH or the default install location
func (w *WindowsInstaller) nrfutilInstalled() bool {
	if w.commandExists("nrfutil") {
		return true
	}

	defaultPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "hubble", "nrfutil", "nrfutil.exe")
	if _, err := os.Stat(defaultPath); err == nil {
		_ = w.ensureNRFUtilPath()
		return true
	}

	return false
}

// ensureNRFUtilPath adds the default nrfutil install location to PATH for this process
func (w *WindowsInstaller) ensureNRFUtilPath() error {
	defaultDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "hubble", "nrfutil")
	currentPath := os.Getenv("PATH")
	if !strings.Contains(strings.ToLower(currentPath), strings.ToLower(defaultDir)) {
		os.Setenv("PATH", defaultDir+";"+currentPath)
	}
	return nil
}

// installNRFUtil downloads the official nrfutil binary and ensures it's available
func (w *WindowsInstaller) installNRFUtil() error {
	url := "https://developer.nordicsemi.com/.pc-tools/nrfutil/x64-win/nrfutil.exe"
	destDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "hubble", "nrfutil")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create nrfutil directory: %w", err)
	}

	destPath := filepath.Join(destDir, "nrfutil.exe")

	if err := w.downloadFile(url, destPath); err != nil {
		return fmt.Errorf("failed to download nrfutil: %w", err)
	}

	// Add to PATH for current process
	if err := w.ensureNRFUtilPath(); err != nil {
		return err
	}

	// Verify it runs
	cmd := exec.Command(destPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nrfutil download completed but binary did not run: %w", err)
	}

	return nil
}
