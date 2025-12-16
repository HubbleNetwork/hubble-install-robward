package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/HubbleNetwork/hubble-install/internal/ui"
)

// PackageManager represents the type of package manager
type PackageManager int

const (
	PackageManagerUnknown PackageManager = iota
	PackageManagerAPT                    // Debian, Ubuntu, etc.
	PackageManagerYUM                    // RHEL, CentOS (older)
	PackageManagerDNF                    // Fedora, RHEL 8+
)

// LinuxInstaller implements the Installer interface for Linux
type LinuxInstaller struct {
	pkgManager PackageManager
}

// NewLinuxInstaller creates a new Linux installer
func NewLinuxInstaller() *LinuxInstaller {
	return &LinuxInstaller{
		pkgManager: detectPackageManager(),
	}
}

// Name returns the platform name
func (l *LinuxInstaller) Name() string {
	return "Linux"
}

// ensureSudoAccess validates sudo access upfront to avoid multiple password prompts
func (l *LinuxInstaller) ensureSudoAccess() error {
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
func (l *LinuxInstaller) CheckPrerequisites(requiredDeps []string) ([]MissingDependency, error) {
	var missing []MissingDependency

	// Check if package manager is supported
	if l.pkgManager == PackageManagerUnknown {
		return nil, fmt.Errorf("unsupported Linux distribution - only apt, dnf, and yum are supported")
	}

	// Check each required dependency
	for _, dep := range requiredDeps {
		switch dep {
		case "uv":
			if !l.commandExists("uv") {
				missing = append(missing, MissingDependency{
					Name:   "uv",
					Status: "Not installed",
				})
			}
		case "segger-jlink":
			// Check for SEGGER J-Link (must be installed manually on Linux)
			if !l.commandExists("JLinkExe") {
				fmt.Println("") // blank line for readability
				ui.PrintError("SEGGER J-Link was not found")
				ui.PrintInfo("Due to license requirements, it must be downloaded manually from:")
				ui.PrintInfo("  https://www.segger.com/downloads/jlink/")
				fmt.Println("") // blank line
				ui.PrintInfo("After downloading, install with:")

				switch l.pkgManager {
				case PackageManagerAPT:
					ui.PrintInfo("  sudo dpkg -i JLink_Linux_*.deb")
				case PackageManagerDNF:
					ui.PrintInfo("  sudo dnf install JLink_Linux_*.rpm")
				case PackageManagerYUM:
					ui.PrintInfo("  sudo yum install JLink_Linux_*.rpm")
				default:
					ui.PrintInfo("  tar xzf JLink_Linux_*.tgz -C ~/opt/SEGGER")
					ui.PrintInfo("  sudo cp ~/opt/SEGGER/JLink*/99-jlink.rules /etc/udev/rules.d/")
				}

				fmt.Println("") // blank line
				return nil, fmt.Errorf("J-Link must be installed before running this installer")
			}
		}
	}

	return missing, nil
}

// InstallPackageManager is not needed for Linux (uv and jlink use direct installers)
func (l *LinuxInstaller) InstallPackageManager() error {
	// Both uv (astral.sh) and jlink (SEGGER) use their own installers
	// No package manager operations needed
	return nil
}

// InstallDependencies installs the specified dependencies
func (l *LinuxInstaller) InstallDependencies(deps []string) error {
	for _, dep := range deps {
		switch dep {
		case "uv":
			// Install uv (must be installed via astral.sh installer)
			if !l.commandExists("uv") {
				ui.PrintInfo("Installing uv from astral.sh...")
				if err := l.installUV(); err != nil {
					return fmt.Errorf("failed to install uv: %w", err)
				}
				ui.PrintSuccess("uv installed successfully")
			} else {
				ui.PrintSuccess("uv already installed")
			}
		case "segger-jlink":
			// J-Link must be installed manually on Linux - verified in CheckPrerequisites
			if l.commandExists("JLinkExe") {
				ui.PrintSuccess("segger-jlink already installed")
			}
		}
	}

	return nil
}

// installUV installs uv using the official astral.sh installer
func (l *LinuxInstaller) installUV() error {
	// Download and run the uv installer script
	cmd := exec.Command("sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uv installation failed: %w", err)
	}

	// Add uv to PATH for current process
	// The installer puts it in ~/.cargo/bin
	homeDir := os.Getenv("HOME")
	cargoPath := filepath.Join(homeDir, ".cargo", "bin")

	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, cargoPath) {
		os.Setenv("PATH", cargoPath+":"+currentPath)
	}

	return nil
}

// FlashBoard flashes the specified board using uvx (for J-Link boards)
func (l *LinuxInstaller) FlashBoard(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
	ui.PrintInfo(fmt.Sprintf("Flashing board: %s", board))
	ui.PrintInfo("This may take 10-15 seconds...")

	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return nil, fmt.Errorf("uv not found in PATH: %w", err)
	}

	// Build the command
	args := []string{"tool", "run", "--from", "pyhubbledemo", "hubbledemo", "flash", board, "-o", orgID, "-t", apiToken}
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
func (l *LinuxInstaller) GenerateHexFile(orgID, apiToken, board, deviceName string) (*FlashResult, error) {
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

	// Build the command with -f for output file
	args := []string{"tool", "run", "--from", "pyhubbledemo", "hubbledemo", "flash", board, "-o", orgID, "-t", apiToken, "-f", hexFilePath}
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

// detectPackageManager detects which package manager is available
func detectPackageManager() PackageManager {
	if commandExistsGlobal("apt-get") {
		return PackageManagerAPT
	}
	if commandExistsGlobal("dnf") {
		return PackageManagerDNF
	}
	if commandExistsGlobal("yum") {
		return PackageManagerYUM
	}
	return PackageManagerUnknown
}

// commandExists checks if a command is available in PATH
func (l *LinuxInstaller) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// commandExistsGlobal checks if a command is available (global function for init)
func commandExistsGlobal(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// installPackage installs a package using the detected package manager
func (l *LinuxInstaller) installPackage(pkg string, showOutput bool) error {
	var cmd *exec.Cmd

	switch l.pkgManager {
	case PackageManagerAPT:
		cmd = exec.Command("sudo", "apt-get", "install", "-y", pkg)
	case PackageManagerDNF:
		cmd = exec.Command("sudo", "dnf", "install", "-y", pkg)
	case PackageManagerYUM:
		cmd = exec.Command("sudo", "yum", "install", "-y", pkg)
	default:
		return fmt.Errorf("unsupported package manager")
	}

	// Show output if requested
	if showOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}
