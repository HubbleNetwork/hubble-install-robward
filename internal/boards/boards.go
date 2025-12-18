package boards

import "fmt"

// Flash methods
const (
	FlashMethodJLink    = "jlink"    // Direct flash via SEGGER J-Link
	FlashMethodUniflash = "uniflash" // Generate hex file for TI Uniflash
)

// Board represents a developer board that can be flashed
type Board struct {
	ID          string
	Name        string
	Description string
	Vendor      string
	FlashMethod string // "jlink" or "uniflash"
}

// RequiresJLink returns true if this board requires SEGGER J-Link
func (b *Board) RequiresJLink() bool {
	return b.FlashMethod == FlashMethodJLink
}

// GetDependencies returns the list of dependencies required for this board
func (b *Board) GetDependencies() []string {
	if b.RequiresJLink() {
		// Nordic DKs use a J-Link probe (often on-board). We need:
		// - uv: to run our Python flashing tool
		// - nrfutil: Nordic CLI used by the flashing tool
		// - segger-jlink: provides J-Link drivers/DLLs needed to talk to the probe
		return []string{"uv", "nrfutil", "segger-jlink"}
	}
	return []string{"uv"}
}

// Available boards for flashing
var AvailableBoards = []Board{
	{
		ID:          "nrf21540dk",
		Name:        "nRF21540 DK",
		Description: "Nordic Semiconductor nRF21540 Development Kit",
		Vendor:      "Nordic",
		FlashMethod: FlashMethodJLink,
	},
	{
		ID:          "nrf52840dk",
		Name:        "nRF52840 DK",
		Description: "Nordic Semiconductor nRF52840 Development Kit",
		Vendor:      "Nordic",
		FlashMethod: FlashMethodJLink,
	},
	{
		ID:          "lp_em_cc2340r5",
		Name:        "TI CC2340R5",
		Description: "Texas Instruments CC2340R5 LaunchPad",
		Vendor:      "Texas Instruments",
		FlashMethod: FlashMethodUniflash,
	},
	{
		ID:          "lp_em_cc2340r53",
		Name:        "TI CC2340R53",
		Description: "Texas Instruments CC2340R53 LaunchPad",
		Vendor:      "Texas Instruments",
		FlashMethod: FlashMethodUniflash,
	},
	// {
	// 	ID:          "xg22_ek4108a",
	// 	Name:        "xG22 EK4108A",
	// 	Description: "Silicon Labs xG22 Explorer Kit",
	// 	Vendor:      "Silicon Labs",
	// },
	// {
	// 	ID:          "xg24_ek2703a",
	// 	Name:        "xG24 EK2703A",
	// 	Description: "Silicon Labs xG24 Explorer Kit",
	// 	Vendor:      "Silicon Labs",
	// },
}

// GetBoard returns a board by its ID
func GetBoard(id string) (*Board, error) {
	for _, board := range AvailableBoards {
		if board.ID == id {
			return &board, nil
		}
	}
	return nil, fmt.Errorf("board not found: %s", id)
}

// FormatBoardList returns a formatted string of all available boards
func FormatBoardList() string {
	result := ""
	for i, board := range AvailableBoards {
		result += fmt.Sprintf("%d. %s - %s\n", i+1, board.Name, board.Description)
	}
	return result
}
