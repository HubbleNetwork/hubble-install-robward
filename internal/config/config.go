package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/HubbleNetwork/hubble-install/internal/boards"
	"github.com/HubbleNetwork/hubble-install/internal/ui"
)

// Config holds the Hubble configuration
type Config struct {
	OrgID    string
	APIToken string
	Board    string
}

// validateCredentials checks if the credentials have the expected format
func validateCredentials(orgID, apiToken string) error {
	// Validate Org ID format (should be a UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	// Example: "0f61efd0-24a7-4a2e-ae0f-8549d14ed901"
	if len(orgID) != 36 {
		return fmt.Errorf("org_id must be a UUID (36 characters), got %d characters", len(orgID))
	}

	// Check UUID format (8-4-4-4-12 with hyphens)
	if orgID[8] != '-' || orgID[13] != '-' || orgID[18] != '-' || orgID[23] != '-' {
		return fmt.Errorf("org_id must be a valid UUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)")
	}

	// Validate that non-hyphen characters are hexadecimal
	for i, c := range orgID {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue // Skip hyphens
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("org_id contains invalid character: %c (must be hex digits and hyphens)", c)
		}
	}

	// Validate API Token format (should be a hex string, typically 96 characters)
	// Example: "eb31d24113fadb77c6d89d65a8007c0eed3595e2255aaf1d7d81783900ab33be4332457a27861f67cc78fe930ea52941"
	if len(apiToken) < 32 {
		return fmt.Errorf("api_token is too short (expected ~96 hex characters, got %d)", len(apiToken))
	}

	// Validate that it's a valid hex string
	for _, c := range apiToken {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("api_token must be a hexadecimal string (found invalid character: %c)", c)
		}
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PromptForConfig prompts the user for all required configuration
// Returns the config and a boolean indicating if credentials were pre-configured
func PromptForConfig() (*Config, bool, error) {
	config := &Config{}
	preConfigured := false

	// Check for base64 encoded credentials first (passed from install.sh)
	// Format: org_id:api_key or org_id:api_key:board_id
	if encodedCreds := os.Getenv("HUBBLE_CREDENTIALS"); encodedCreds != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodedCreds)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 3)
			if len(parts) >= 2 {
				config.OrgID = strings.TrimSpace(parts[0])
				config.APIToken = strings.TrimSpace(parts[1])
				if config.OrgID != "" && config.APIToken != "" {
					// Validate credential format
					if err := validateCredentials(config.OrgID, config.APIToken); err != nil {
						return nil, false, fmt.Errorf("invalid credentials from HUBBLE_CREDENTIALS: %w", err)
					}
					// Parse optional board_id (third parameter)
					if len(parts) == 3 {
						boardID := strings.TrimSpace(parts[2])
						if boardID != "" {
							// Validate board ID exists and resolve to canonical ID
							board, err := boards.GetBoard(boardID)
							if err != nil {
								return nil, false, fmt.Errorf("invalid board_id from HUBBLE_CREDENTIALS: %w", err)
							}
							config.Board = board.ID
						}
					}
					preConfigured = true
					return config, preConfigured, nil
				}
			}
		}
	}

	// Check environment variables
	envOrgID := os.Getenv("HUBBLE_ORG_ID")
	envAPIToken := os.Getenv("HUBBLE_API_TOKEN")

	// If both are present, use them
	if envOrgID != "" && envAPIToken != "" {
		config.OrgID = envOrgID
		config.APIToken = envAPIToken
		// Validate credential format
		if err := validateCredentials(config.OrgID, config.APIToken); err != nil {
			return nil, false, fmt.Errorf("invalid credentials from environment: %w", err)
		}
		preConfigured = true
		ui.PrintSuccess("Credentials found in environment")
		return config, preConfigured, nil
	}

	// Print info about where to find credentials
	ui.PrintInfo("Get your credentials at: https://dash.hubble.com/developer/api-tokens")
	fmt.Println()

	// Prompt for Org ID (if not in environment)
	if envOrgID != "" {
		config.OrgID = envOrgID
		ui.PrintSuccess(fmt.Sprintf("Using Org ID from environment: %s", envOrgID))
	} else {
		for {
			orgID := ui.PromptInput("Enter your Hubble Org ID")
			orgID = strings.TrimSpace(orgID)
			if orgID != "" {
				config.OrgID = orgID
				break
			}
			ui.PrintWarning("Org ID cannot be empty")
		}
	}

	// Prompt for API Token (if not in environment)
	if envAPIToken != "" {
		config.APIToken = envAPIToken
		ui.PrintSuccess("Using API Token from environment")
	} else {
		for {
			apiToken := ui.PromptPassword("Enter your Hubble API Token (hidden)")
			apiToken = strings.TrimSpace(apiToken)
			if apiToken != "" {
				config.APIToken = apiToken
				break
			}
			ui.PrintWarning("API Token cannot be empty")
		}
	}

	// Validate the final credentials
	if err := validateCredentials(config.OrgID, config.APIToken); err != nil {
		return nil, false, fmt.Errorf("invalid credentials: %w. Please check the format at https://dash.hubble.com/developer/api-tokens", err)
	}

	ui.PrintSuccess("Credentials configured")

	return config, preConfigured, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.OrgID == "" {
		return fmt.Errorf("org ID is required")
	}
	if c.APIToken == "" {
		return fmt.Errorf("API token is required")
	}
	if c.Board == "" {
		return fmt.Errorf("board selection is required")
	}
	return nil
}
