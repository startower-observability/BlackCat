package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/startower-observability/blackcat/internal/service"
)

var doctorFix bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check BlackCat configuration health",
	Long:  "Run diagnostic checks on your BlackCat installation and configuration. Use --fix to auto-repair simple issues.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("BlackCat Doctor")
		fmt.Println()

		passed := 0
		failed := 0
		warned := 0

		// Check 1: Config file exists and is valid.
		home, _ := os.UserHomeDir()
		configPath := filepath.Join(home, ".blackcat", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			printCheck(true, fmt.Sprintf("Config file valid (%s)", configPath))
			passed++
		} else {
			if doctorFix {
				dir := filepath.Dir(configPath)
				_ = os.MkdirAll(dir, 0o755)
				if err := os.WriteFile(configPath, []byte("# BlackCat configuration\n"), 0o644); err == nil {
					printCheck(true, fmt.Sprintf("Config file created (%s)", configPath))
					passed++
				} else {
					printFail(fmt.Sprintf("Config file missing (%s) — could not create", configPath))
					failed++
				}
			} else {
				printFail(fmt.Sprintf("Config file missing (%s)", configPath))
				failed++
			}
		}

		// Check 2: LLM provider configured.
		provider := viper.GetString("llm.provider")
		if provider != "" {
			printCheck(true, fmt.Sprintf("LLM provider configured (%s)", provider))
			passed++
		} else {
			printFail("No LLM provider configured — run 'blackcat configure'")
			failed++
		}

		// Check 3: At least one channel enabled.
		channelFound := false
		for _, ch := range []string{"telegram", "discord", "whatsapp"} {
			if viper.GetBool(fmt.Sprintf("channels.%s.enabled", ch)) {
				channelFound = true
				break
			}
		}
		if channelFound {
			printCheck(true, "At least one channel enabled")
			passed++
		} else {
			printFail("No channels enabled — run 'blackcat channels add'")
			failed++
		}

		// Check 4: Vault passphrase.
		vaultPass := viper.GetString("vault.passphrase")
		if vaultPass == "" {
			vaultPass = os.Getenv("BLACKCAT_VAULT_PASSPHRASE")
		}
		if vaultPass != "" {
			printCheck(true, "Vault passphrase set")
			passed++
		} else {
			printWarn("Vault passphrase not set (env BLACKCAT_VAULT_PASSPHRASE)")
			warned++
		}

		// Check 5: OpenCode binary accessible.
		if ocPath, err := exec.LookPath("opencode"); err == nil {
			printCheck(true, fmt.Sprintf("OpenCode binary found (%s)", ocPath))
			passed++
		} else {
			printWarn("OpenCode binary not found in PATH")
			warned++
		}

		// Check 6: Daemon service installed.
		mgr := service.New()
		if mgr.IsInstalled() {
			printCheck(true, "Daemon service installed")
			passed++
		} else {
			printFail("Daemon not installed — run 'blackcat onboard'")
			failed++
		}

		// Check 7: Daemon running + health.
		st, _ := mgr.Status()
		if st.Running {
			printCheck(true, fmt.Sprintf("Daemon running (PID %d)", st.PID))
			passed++

			addr := viper.GetString("addr")
			if addr == "" {
				addr = "http://127.0.0.1:8080"
			}
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(addr + "/health")
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					printCheck(true, "Health check passed")
					passed++
				} else {
					printWarn(fmt.Sprintf("Health check returned HTTP %d", resp.StatusCode))
					warned++
				}
			} else {
				printWarn("Health endpoint unreachable")
				warned++
			}
		} else if mgr.IsInstalled() {
			printFail("Daemon not running — run 'blackcat start'")
			failed++
		}

		// Check 8: WhatsApp session.
		if viper.GetBool("channels.whatsapp.enabled") {
			waDB := filepath.Join(home, ".blackcat", "whatsapp.db")
			if _, err := os.Stat(waDB); err == nil {
				printCheck(true, "WhatsApp session database exists")
				passed++
			} else {
				printFail("WhatsApp enabled but no session — run 'blackcat channels login --channel whatsapp'")
				failed++
			}
		}

		// Check 9: Node.js version (required for Google Workspace CLI and other npm-based tools).
		nodeVersion, nodeErr := exec.Command("node", "--version").Output()
		if nodeErr != nil {
			printWarn("Node.js not found — Google Workspace CLI and npm-based skills unavailable")
			fmt.Println("    Install Node.js 18+ from https://nodejs.org")
			warned++
		} else {
			vStr := strings.TrimSpace(string(nodeVersion)) // e.g. "v20.11.0"
			major := 0
			fmt.Sscanf(vStr, "v%d.", &major)
			if major < 18 {
				printWarn(fmt.Sprintf("Node.js %s found but version 18+ required for npm-based skills", vStr))
				warned++
			} else {
				printCheck(true, fmt.Sprintf("Node.js %s", vStr))
				passed++
			}
		}

		// Check 10: Marketplace directory.
		marketplaceDir := viper.GetString("skills.marketplace_dir")
		if marketplaceDir == "" {
			marketplaceDir = "marketplace"
		}
		fullMarketplacePath := filepath.Join(home, ".blackcat", marketplaceDir)
		if info, err := os.Stat(fullMarketplacePath); err != nil {
			printWarn(fmt.Sprintf("Marketplace directory not found (%s) — run 'blackcat init' to create it", fullMarketplacePath))
			warned++
		} else if !info.IsDir() {
			printFail(fmt.Sprintf("Marketplace path exists but is not a directory (%s)", fullMarketplacePath))
			failed++
		} else {
			printCheck(true, fmt.Sprintf("Marketplace directory found (%s)", fullMarketplacePath))
			passed++
		}

		// Check 11: npx availability (only if allow_external_install is true).
		if viper.GetBool("skills.allow_external_install") {
			if _, err := exec.LookPath("npx"); err != nil {
				printFail("npx not found — required for marketplace skill installation (allow_external_install is true)")
				failed++
			} else {
				printCheck(true, "npx available for marketplace skill installation")
				passed++
			}
		}

		// Summary.
		fmt.Println()
		fmt.Printf("  %d passed, %d warnings, %d failed\n", passed, warned, failed)

		if failed > 0 {
			return fmt.Errorf("%d checks failed", failed)
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "auto-repair simple issues")
	rootCmd.AddCommand(doctorCmd)
}

func printCheck(ok bool, msg string) {
	if ok {
		fmt.Printf("  ✓ %s\n", msg)
	}
}

func printFail(msg string) {
	fmt.Printf("  ✗ %s\n", msg)
}

func printWarn(msg string) {
	fmt.Printf("  ⚠ %s\n", msg)
}

// doctorOutput is the JSON structure for --json output (future enhancement).
type doctorOutput struct {
	Checks []doctorCheck `json:"checks"`
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "fail", "warn"
	Detail string `json:"detail,omitempty"`
}

func printDoctorJSON(checks []doctorCheck) {
	out := doctorOutput{Checks: checks}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
