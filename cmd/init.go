package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/startower-observability/blackcat/internal/workspace"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a BlackCat workspace",
	Long: `init creates workspace bootstrap files and an example configuration
in the specified directory (default: current directory).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool("force", false, "Overwrite existing files")
}

func runInit(cmd *cobra.Command, args []string) error {
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Initialize workspace using the workspace package
	if err := workspace.InitWorkspace(targetDir); err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	// Create marketplace directory
	marketplaceDir := filepath.Join(targetDir, "marketplace")
	if err := os.MkdirAll(marketplaceDir, 0755); err != nil {
		slog.Warn("failed to create marketplace directory", "path", marketplaceDir, "error", err)
	}

	// Create example config file if it doesn't exist
	exampleConfigPath := filepath.Join(targetDir, "blackcat.yaml")
	if _, err := os.Stat(exampleConfigPath); err == nil {
		fmt.Printf("Config file already exists: %s\n", exampleConfigPath)
	} else if os.IsNotExist(err) {
		// Copy example config from embedded resources or create a reference
		exampleContent := `# BlackCat Configuration
# For full configuration options, see blackcat.example.yaml

server:
  addr: ":8080"
  port: 8080

opencode:
  addr: "http://127.0.0.1:4096"
  password: "" # Set via env or vault

llm:
  provider: "openai"
  model: "gpt-4"
  apiKey: "" # Set via env or vault
  temperature: 0.7
  maxTokens: 4096

channels:
  telegram:
    enabled: false
    token: "" # Set via env or vault

security:
  vaultPath: "~/.blackcat/vault.json"
  denyPatterns: []
  autoPermit: false

memory:
  filePath: "MEMORY.md"
  consolidationThreshold: 50

logging:
  level: "info"
  format: "text"
`
		if err := os.WriteFile(exampleConfigPath, []byte(exampleContent), 0o644); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		fmt.Printf("Created: %s\n", exampleConfigPath)
	} else {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	// Print summary
	fmt.Printf("\nWorkspace initialized in: %s\n", targetDir)
	fmt.Println("\nCreated files:")
	fmt.Println("  - AGENTS.md")
	fmt.Println("  - SOUL.md")
	fmt.Println("  - MEMORY.md")
	fmt.Println("  - skills/ (directory)")
	fmt.Println("  - marketplace/ (directory)")
	fmt.Println("  - blackcat.yaml")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit blackcat.yaml with your configuration")
	fmt.Println("  2. Set secrets using: blackcat vault set <key>")
	fmt.Println("  3. Run: blackcat serve")

	return nil
}
