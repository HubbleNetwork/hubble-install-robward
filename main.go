package main

import (
	"fmt"
	"os"
	"time"

	"github.com/HubbleNetwork/hubble-install/internal/boards"
	"github.com/HubbleNetwork/hubble-install/internal/config"
	"github.com/HubbleNetwork/hubble-install/internal/platform"
	"github.com/HubbleNetwork/hubble-install/internal/ui"
)

func main() {
	// Print welcome banner
	ui.PrintBanner()
	fmt.Println()

	// Show what will happen
	ui.PrintInfo("This installer will:")
	fmt.Println("  • Confirm your developer board model")
	fmt.Println("  • Check for and install required dependencies")
	fmt.Println("  • Configure your Hubble credentials")
	fmt.Println("  • Register your board to your organization, and give it a name")
	fmt.Println("  • Provision your board, or generate a hex file for you to flash")
	fmt.Println()

	// Prompt user to continue
	if !ui.PromptYesNo("Ready to install?", true) {
		ui.PrintWarning("Installation cancelled")
		os.Exit(0)
	}
	fmt.Println()

	// Start timer for the installation
	startTime := time.Now()

	// Detect platform
	installer, err := platform.GetInstaller()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Platform detection failed: %v", err))
		os.Exit(1)
	}

	// =========================================================================
	// Step 1: Get credentials (may include pre-configured board)
	// =========================================================================
	currentStep := 1
	totalSteps := 0
	ui.PrintStep("Configuring credentials", currentStep, totalSteps)

	cfg, preConfigured, err := config.PromptForConfig()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Configuration failed: %v", err))
		os.Exit(1)
	}

	if preConfigured {
		fmt.Println()
		ui.PrintSuccess("We've handled your setup details")
		fmt.Println()
		ui.PrintInfo("We've pre-filled your credentials for this command.")
		fmt.Println()
		fmt.Println("Your Hubble Org ID and API Token are used to register your board to your organization.")
		fmt.Println()
	}

	// =========================================================================
	// Step 2: Select board (if not pre-configured)
	// =========================================================================
	currentStep++
	ui.PrintStep("Selecting developer board", currentStep, totalSteps)

	var selectedBoard boards.Board
	if cfg.Board != "" {
		// Board was pre-configured via credentials
		board, err := boards.GetBoard(cfg.Board)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Invalid pre-configured board: %v", err))
			os.Exit(1)
		}
		selectedBoard = *board
		ui.PrintSuccess(fmt.Sprintf("Using pre-configured board: %s", selectedBoard.Name))
	} else {
		// Prompt user to select a board
		boardOptions := make([]string, len(boards.AvailableBoards))
		for i, board := range boards.AvailableBoards {
			boardOptions[i] = fmt.Sprintf("%s - %s (%s)", board.Name, board.Description, board.Vendor)
		}

		selectedIndex := ui.PromptChoice("Available developer boards:", boardOptions)
		selectedBoard = boards.AvailableBoards[selectedIndex]
		cfg.Board = selectedBoard.ID

		ui.PrintSuccess(fmt.Sprintf("Selected: %s", selectedBoard.Name))
	}

	fmt.Println()
	if selectedBoard.RequiresJLink() {
		ui.PrintInfo("This board uses SEGGER J-Link for direct flashing.")
		ui.PrintWarning("Make sure your board is connected via USB with a data-capable cable.")
	} else {
		ui.PrintInfo("This board uses TI Uniflash. A hex file will be generated for you.")
		ui.PrintInfo("You'll need Uniflash installed to complete the flashing process.")
	}
	fmt.Println()

	// =========================================================================
	// Step 3: Check prerequisites (based on selected board)
	// =========================================================================
	currentStep++
	ui.PrintStep("Checking prerequisites", currentStep, totalSteps)

	requiredDeps := selectedBoard.GetDependencies()
	missing, err := installer.CheckPrerequisites(requiredDeps)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Prerequisites check failed: %v", err))
		os.Exit(1)
	}

	totalSteps = 4
	if len(missing) > 0 {
		totalSteps++
	}

	if len(missing) > 0 {
		ui.PrintWarning("Missing dependencies detected:")
		for _, dep := range missing {
			fmt.Printf("  • %s: %s\n", dep.Name, dep.Status)
		}
		fmt.Println()

		if !ui.PromptYesNo("Would you like to install missing dependencies?", true) {
			ui.PrintError("Cannot proceed without dependencies")
			os.Exit(1)
		}
	} else {
		ui.PrintSuccess("All prerequisites satisfied")
	}

	// =========================================================================
	// Step 4: Install dependencies (only if needed)
	// =========================================================================
	if len(missing) > 0 {
		currentStep++
		ui.PrintStep("Installing dependencies", currentStep, totalSteps)

		// Check if we need to install package manager first
		needsPackageManager := false
		for _, dep := range missing {
			if dep.Name == "Homebrew" {
				needsPackageManager = true
				break
			}
		}

		if needsPackageManager {
			if err := installer.InstallPackageManager(); err != nil {
				ui.PrintError(fmt.Sprintf("Package manager installation failed: %v", err))
				os.Exit(1)
			}
		}

		// Install board-specific dependencies
		if err := installer.InstallDependencies(requiredDeps); err != nil {
			ui.PrintError(fmt.Sprintf("Dependency installation failed: %v", err))
			os.Exit(1)
		}

		ui.PrintSuccess("All dependencies installed")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		ui.PrintError(fmt.Sprintf("Invalid configuration: %v", err))
		os.Exit(1)
	}

	// =========================================================================
	// Final Step: Flash board or generate hex file
	// =========================================================================
	currentStep++

	if selectedBoard.RequiresJLink() {
		// J-Link path: Direct flash
		if !ui.PromptYesNo(fmt.Sprintf("Would you like to flash your %s now?", selectedBoard.Name), true) {
			ui.PrintWarning("Flashing skipped. You can flash later using:")
			fmt.Printf("  uv tool run --from pyhubbledemo hubbledemo flash %s -o %s -t <your_token>\n", cfg.Board, cfg.OrgID)
			os.Exit(0)
		}

		// Prompt for optional device name
		deviceName := ui.PromptOptionalInput("What should the device name be?")

		ui.PrintStep("Flashing board", currentStep, totalSteps)
		result, err := installer.FlashBoard(cfg.OrgID, cfg.APIToken, cfg.Board, deviceName)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Board flashing failed: %v", err))
			os.Exit(1)
		}

		// Print J-Link completion banner
		duration := time.Since(startTime)
		ui.PrintCompletionBanner(duration, cfg.OrgID, cfg.APIToken, result.DeviceName)

	} else {
		// Uniflash path: Generate hex file
		if !ui.PromptYesNo(fmt.Sprintf("Would you like to generate the hex file for your %s now?", selectedBoard.Name), true) {
			ui.PrintWarning("Hex generation skipped. You can generate later using:")
			fmt.Printf("  uv tool run --from pyhubbledemo hubbledemo flash %s -o %s -t <your_token>\n", cfg.Board, cfg.OrgID)
			os.Exit(0)
		}

		// Prompt for optional device name
		deviceName := ui.PromptOptionalInput("What should the device name be?")

		ui.PrintStep("Generating hex file", currentStep, totalSteps)
		result, err := installer.GenerateHexFile(cfg.OrgID, cfg.APIToken, cfg.Board, deviceName)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Hex file generation failed: %v", err))
			os.Exit(1)
		}

		// Print Uniflash completion banner
		duration := time.Since(startTime)
		ui.PrintUniflashCompletionBanner(duration, result.HexFilePath, selectedBoard.Name)
	}

	os.Exit(0)
}
