package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/HubbleNetwork/hubble-install/internal/ui"
)

// WindowsInstaller implements the Installer interface for Windows
type WindowsInstaller struct{}

// NewWindowsInstaller creates a new Windows installer
func NewWindowsInstaller() *WindowsInstaller {
	return &WindowsInstaller{}
}

// Name returns the platform name
func (w *WindowsInstaller) Name() string {
	return "Windows"
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
		case "segger-jlink":
			// Check for SEGGER J-Link (via nrfjprog package)
			// Try multiple possible command names
			if !w.commandExists("JLink") && !w.commandExists("JLinkExe") && !w.commandExists("nrfjprog") {
				missing = append(missing, MissingDependency{
					Name:   "segger-jlink",
					Status: "Not installed",
				})
			}
		}
	}

	return missing, nil
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

		case "segger-jlink":
			// Install J-Link via nrfjprog package (includes J-Link on Windows)
			// Try multiple possible command names
			if w.commandExists("JLink") || w.commandExists("JLinkExe") || w.commandExists("nrfjprog") {
				ui.PrintSuccess("segger-jlink already installed")
			} else {
				ui.PrintInfo("Installing segger-jlink via nrfjprog (this may take a few minutes)...")
				if err := w.runChocoInstall("nrfjprog", true); err != nil {
					return fmt.Errorf("failed to install nrfjprog: %w", err)
				}
				ui.PrintSuccess("segger-jlink installed successfully")
			}
		}
	}

	return nil
}

// FlashBoard flashes the specified board using uvx (for J-Link boards)
func (w *WindowsInstaller) FlashBoard(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
	ui.PrintInfo(fmt.Sprintf("Flashing board: %s", board))
	ui.PrintInfo("This may take 10-15 seconds...")

	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return nil, fmt.Errorf("uv not found in PATH: %w", err)
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

	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return nil, fmt.Errorf("uv not found in PATH: %w", err)
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

	return cmd.Run()
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
