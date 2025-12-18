package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/HubbleNetwork/hubble-install/internal/ui"
)

// DarwinInstaller implements the Installer interface for macOS
type DarwinInstaller struct{}

// NewDarwinInstaller creates a new macOS installer
func NewDarwinInstaller() *DarwinInstaller {
	return &DarwinInstaller{}
}

// Name returns the platform name
func (d *DarwinInstaller) Name() string {
	return "macOS"
}

// CheckPendingReboot checks if a system reboot is pending (not typically needed on macOS)
func (d *DarwinInstaller) CheckPendingReboot() error {
	// macOS doesn't typically require reboot checks for package installations
	return nil
}

// ensureSudoAccess validates sudo access upfront to avoid multiple password prompts
func (d *DarwinInstaller) ensureSudoAccess() error {
	// Check if we already have valid sudo credentials
	checkCmd := exec.Command("sudo", "-n", "true")
	if err := checkCmd.Run(); err == nil {
		// Already have valid sudo, no need to prompt
		return nil
	}

	// Need to prompt for password
	ui.PrintWarning("Administrator access required for installation")
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to obtain sudo access: %w", err)
	}

	return nil
}

// CheckPrerequisites checks for missing dependencies based on required deps
func (d *DarwinInstaller) CheckPrerequisites(requiredDeps []string) ([]MissingDependency, error) {
	var missing []MissingDependency

	// Check for Homebrew (always required for installing other deps)
	if !d.commandExists("brew") {
		missing = append(missing, MissingDependency{
			Name:   "Homebrew",
			Status: "Not installed",
		})
	}

	// Check each required dependency
	for _, dep := range requiredDeps {
		switch dep {
		case "uv":
			if !d.commandExists("uv") {
				missing = append(missing, MissingDependency{
					Name:   "uv",
					Status: "Not installed",
				})
			}
		case "nrfutil":
			if !d.commandExists("nrfutil") {
				missing = append(missing, MissingDependency{
					Name:   "nrfutil",
					Status: "Not installed",
				})
			}
		case "segger-jlink":
			if !d.commandExists("JLinkExe") {
				missing = append(missing, MissingDependency{
					Name:   "segger-jlink",
					Status: "Not installed",
				})
			}
		}
	}

	return missing, nil
}

// InstallPackageManager installs Homebrew if not present
func (d *DarwinInstaller) InstallPackageManager() error {
	if d.commandExists("brew") {
		ui.PrintSuccess("Homebrew already installed")
		return nil
	}

	// Ensure we have sudo access upfront (single password prompt)
	// The Homebrew script will use sudo internally when needed (e.g., for Xcode Command Line Tools)
	if err := d.ensureSudoAccess(); err != nil {
		return err
	}

	ui.PrintInfo("Installing Homebrew...")
	ui.PrintInfo("This may take a few minutes...")

	// Run the official Homebrew installation script as regular user (not sudo)
	// The script will internally use sudo when needed, using our cached credentials
	// NONINTERACTIVE=1 suppresses the "running in noninteractive mode" warning
	cmd := exec.Command("/bin/bash", "-c", `NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Homebrew: %w", err)
	}

	// Add Homebrew to PATH for this process
	if err := d.setupBrewPath(); err != nil {
		return fmt.Errorf("homebrew installation completed but could not find brew binary: %w", err)
	}

	// Verify that brew is actually working
	if !d.commandExists("brew") {
		return fmt.Errorf("homebrew installation completed but brew command not found in PATH")
	}

	// Test brew with a simple command to ensure it's functional
	testCmd := exec.Command("brew", "--version")
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("homebrew installed but not functioning correctly: %w", err)
	}

	ui.PrintSuccess("Homebrew installed successfully")
	return nil
}

// InstallDependencies installs the specified dependencies
func (d *DarwinInstaller) InstallDependencies(deps []string) error {
	// First ensure Homebrew is installed
	if !d.commandExists("brew") {
		if err := d.InstallPackageManager(); err != nil {
			return err
		}
	}

	// Install dependencies in parallel for speed
	var wg sync.WaitGroup
	errChan := make(chan error, len(deps))

	for _, dep := range deps {
		dep := dep // capture loop variable
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch dep {
			case "uv":
				if d.commandExists("uv") {
					ui.PrintSuccess("uv already installed")
					return
				}
				ui.PrintInfo("Installing uv...")
				if err := d.runBrewInstall("uv", false); err != nil {
					errChan <- fmt.Errorf("failed to install uv: %w", err)
					return
				}
				ui.PrintSuccess("uv installed successfully")

			case "nrfutil":
				if d.commandExists("nrfutil") {
					ui.PrintSuccess("nrfutil already installed")
					return
				}
				uvPath, err := exec.LookPath("uv")
				if err != nil {
					errChan <- fmt.Errorf("uv not found in PATH (required to install nrfutil): %w", err)
					return
				}
				ui.PrintInfo("Installing nrfutil (via uv tool install)...")
				cmd := exec.Command(uvPath, "tool", "install", "nrfutil")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					errChan <- fmt.Errorf("failed to install nrfutil: %w", err)
					return
				}
				ui.PrintSuccess("nrfutil installed successfully")

			case "segger-jlink":
				if d.commandExists("JLinkExe") {
					ui.PrintSuccess("segger-jlink already installed")
					return
				}
				ui.PrintInfo("Installing segger-jlink (this may take a few minutes)...")
				if err := d.runBrewInstall("segger-jlink", true); err != nil {
					errChan <- fmt.Errorf("failed to install segger-jlink: %w", err)
					return
				}
				ui.PrintSuccess("segger-jlink installed successfully")
			}
		}()
	}

	// Wait for all installations to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// FlashBoard flashes the specified board using uvx (for J-Link boards)
func (d *DarwinInstaller) FlashBoard(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
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
func (d *DarwinInstaller) GenerateHexFile(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
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
func (d *DarwinInstaller) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// setupBrewPath adds Homebrew to PATH for the current process
func (d *DarwinInstaller) setupBrewPath() error {
	// Detect Homebrew installation path based on architecture
	// Apple Silicon: /opt/homebrew
	// Intel: /usr/local
	var brewPath string
	if _, err := os.Stat("/opt/homebrew/bin/brew"); err == nil {
		brewPath = "/opt/homebrew/bin"
	} else if _, err := os.Stat("/usr/local/bin/brew"); err == nil {
		brewPath = "/usr/local/bin"
	} else {
		return fmt.Errorf("brew not found in expected locations")
	}

	// Update PATH for this process
	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, brewPath) {
		newPath := brewPath + ":" + currentPath
		os.Setenv("PATH", newPath)
	}

	return nil
}

// runBrewInstall runs a brew install command
func (d *DarwinInstaller) runBrewInstall(pkg string, showOutput bool) error {
	cmd := exec.Command("brew", "install", pkg)

	// Show output if requested
	if showOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}
