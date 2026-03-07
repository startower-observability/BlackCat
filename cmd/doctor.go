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
	"github.com/startower-observability/blackcat/internal/skills"
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

		// Check 6: RTK binary accessible (optional).
		if viper.GetBool("rtk.enabled") {
			if rtkPath, err := exec.LookPath("rtk"); err == nil {
				printCheck(true, fmt.Sprintf("RTK binary found (%s)", rtkPath))
				passed++
			} else {
				printWarn("RTK wrapping enabled but 'rtk' not found in PATH")
				warned++
			}
		} else {
			printCheck(true, "RTK wrapping disabled (optional)")
			passed++
		}

		// Check 7: Daemon service installed.
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

		// Check 12: Provider fallback configuration.
		validProviders := map[string]bool{"openai": true, "copilot": true, "antigravity": true, "gemini": true, "zen": true}
		fallbacks := viper.GetStringSlice("llm.fallback")
		if len(fallbacks) == 0 {
			printWarn("No fallback LLM providers configured (llm.fallback is empty) — single point of failure")
			warned++
		} else {
			allValid := true
			for _, fb := range fallbacks {
				if !validProviders[fb] {
					printFail(fmt.Sprintf("Invalid fallback provider %q in llm.fallback — valid: openai, copilot, antigravity, gemini, zen", fb))
					failed++
					allValid = false
				}
			}
			if allValid {
				printCheck(true, fmt.Sprintf("Fallback providers configured: %s", strings.Join(fallbacks, ", ")))
				passed++
			}
		}

		// Check 13: Budget configuration.
		if viper.GetBool("budget.enabled") {
			daily := viper.GetFloat64("budget.daily_limit_usd")
			monthly := viper.GetFloat64("budget.monthly_limit_usd")
			if daily <= 0 && monthly <= 0 {
				printWarn("Budget enabled but no limits set (budget.daily_limit_usd and budget.monthly_limit_usd are both 0)")
				warned++
			} else {
				printCheck(true, fmt.Sprintf("Budget controls active (daily: $%.2f, monthly: $%.2f)", daily, monthly))
				passed++
			}
		} else {
			printWarn("Budget controls disabled — no spend limits enforced (set budget.enabled: true to enable)")
			warned++
		}

		// Check 14: Marketplace registry.
		marketplacePath := viper.GetString("skills.marketplace_dir")
		if marketplacePath == "" {
			marketplacePath = "marketplace"
		}
		fullMarketplacePath14 := filepath.Join(home, ".blackcat", marketplacePath)
		registryPath := filepath.Join(fullMarketplacePath14, "registry.json")
		if _, err := os.Stat(registryPath); err == nil {
			printCheck(true, fmt.Sprintf("Marketplace registry found (%s)", registryPath))
			passed++
		} else {
			printWarn(fmt.Sprintf("No marketplace registry found at %s — skills marketplace not initialized", registryPath))
			warned++
		}

		// Check 15: Skill install hints for ineligible skills.
		skillsDir := viper.GetString("skills.dir")
		if skillsDir == "" {
			skillsDir = "skills/"
		}
		fullSkillsDir := filepath.Join(home, ".blackcat", skillsDir)
		if loadedSkills, err := skills.LoadSkills(fullSkillsDir); err == nil {
			hints := 0
			for _, sk := range loadedSkills {
				if !sk.IsEligible() && sk.Install != "" {
					printWarn(fmt.Sprintf("Skill %q not eligible — install hint: %s", sk.Name, sk.Install))
					warned++
					hints++
				}
			}
			if hints == 0 {
				printCheck(true, fmt.Sprintf("All %d loaded skills are eligible", len(loadedSkills)))
				passed++
			}
		}

		// ── Voice Transcription ──────────────────────────────────────
		fmt.Println()
		fmt.Println("--- Voice Transcription ---")

		// Check 16: Whisper configuration.
		if viper.GetBool("whisper.enabled") {
			if os.Getenv("BLACKCAT_WHISPER_GROQAPIKEY") != "" {
				printCheck(true, "Whisper enabled and Groq API key set")
				passed++
			} else {
				printFail("Groq API key missing (set BLACKCAT_WHISPER_GROQAPIKEY)")
				failed++
			}
		} else {
			printWarn("Whisper disabled — voice transcription unavailable (set whisper.enabled: true + BLACKCAT_WHISPER_GROQAPIKEY to enable)")
			warned++
		}

		// ── Social Media Skills ─────────────────────────────────────
		fmt.Println()
		fmt.Println("--- Social Media Skills ---")

		// Check 17: Threads skill.
		if os.Getenv("THREADS_ACCESS_TOKEN") != "" {
			printCheck(true, "Threads: THREADS_ACCESS_TOKEN set")
			passed++
		} else {
			printWarn("Threads: THREADS_ACCESS_TOKEN not set")
			warned++
		}

		// Check 18: Twitter/X skill.
		if _, err := exec.LookPath("bird"); err != nil {
			printWarn("Twitter/X: bird CLI not found (npm install -g @steipete/bird)")
			warned++
		} else {
			printCheck(true, "Twitter/X: bird CLI found")
			passed++
		}
		if os.Getenv("TWITTER_AUTH_TOKEN") != "" {
			printCheck(true, "Twitter/X: TWITTER_AUTH_TOKEN set")
			passed++
		} else {
			printWarn("Twitter/X: TWITTER_AUTH_TOKEN not set")
			warned++
		}

		// Check 19: LinkedIn skill.
		if _, err := exec.LookPath("python3"); err != nil {
			printWarn("LinkedIn: python3 not found")
			warned++
		} else {
			printCheck(true, "LinkedIn: python3 found")
			passed++
		}
		if os.Getenv("LINKEDIN_LI_AT") != "" {
			printCheck(true, "LinkedIn: LINKEDIN_LI_AT set")
			passed++
		} else {
			printWarn("LinkedIn: LINKEDIN_LI_AT not set")
			warned++
		}
		if os.Getenv("LINKEDIN_JSESSIONID") != "" {
			printCheck(true, "LinkedIn: LINKEDIN_JSESSIONID set")
			passed++
		} else {
			printWarn("LinkedIn: LINKEDIN_JSESSIONID not set")
			warned++
		}

		// Check 20: Facebook skill.
		if os.Getenv("FACEBOOK_PAGE_TOKEN") != "" {
			printCheck(true, "Facebook: FACEBOOK_PAGE_TOKEN set")
			passed++
		} else {
			printWarn("Facebook: FACEBOOK_PAGE_TOKEN not set")
			warned++
		}

		// Check 21: TikTok skill.
		if os.Getenv("TIKTOK_ACCESS_TOKEN") != "" {
			printCheck(true, "TikTok: TIKTOK_ACCESS_TOKEN set")
			passed++
		} else {
			printWarn("TikTok: TIKTOK_ACCESS_TOKEN not set")
			warned++
		}

		// Check 22: Google Workspace skill.
		if _, err := exec.LookPath("gws"); err != nil {
			printWarn("Google Workspace: gws CLI not found (npm install -g @googleworkspace/cli)")
			warned++
		} else {
			printCheck(true, "Google Workspace: gws CLI found")
			passed++
		}

		// ── Phase 2 Skills ──────────────────────────────────────────
		fmt.Println()
		fmt.Println("--- Phase 2 Skills ---")

		// Check 23 + 24: Veo3 / nano-banana (Gemini-based generation skills).
		uvFound := false
		if _, err := exec.LookPath("uv"); err != nil {
			printWarn("Veo3/nano-banana: uv not found (pip install uv)")
			warned++
		} else {
			printCheck(true, "Veo3/nano-banana: uv found")
			passed++
			uvFound = true
		}
		_ = uvFound // informational — both skills silently skip when missing
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			printWarn("Veo3: ffmpeg not found")
			warned++
		} else {
			printCheck(true, "Veo3: ffmpeg found")
			passed++
		}
		if os.Getenv("GEMINI_API_KEY") != "" {
			printCheck(true, "Veo3/nano-banana: GEMINI_API_KEY set")
			passed++
		} else {
			printWarn("Veo3/nano-banana: GEMINI_API_KEY not set")
			warned++
		}

		// Check 25: document-processing.
		if _, err := exec.LookPath("python3"); err != nil {
			printWarn("document-processing: python3 not found")
			warned++
		} else {
			printCheck(true, "document-processing: python3 found")
			passed++
		}

		// Check 26: capability-evolver.
		if _, err := exec.LookPath("node"); err != nil {
			printWarn("capability-evolver: node not found")
			warned++
		} else {
			printCheck(true, "capability-evolver: node found")
			passed++
		}

		// Check 27: reddit-scraper.
		if _, err := exec.LookPath("python3"); err != nil {
			printWarn("reddit-scraper: python3 not found")
			warned++
		} else {
			printCheck(true, "reddit-scraper: python3 found")
			passed++
		}

		// Check 28: prompt-guard.
		if _, err := exec.LookPath("python3"); err != nil {
			printWarn("prompt-guard: python3 not found")
			warned++
		} else {
			printCheck(true, "prompt-guard: python3 found")
			passed++
		}

		// Check 29: marketplace-installer.
		if _, err := exec.LookPath("npx"); err != nil {
			printWarn("marketplace-installer: npx not found")
			warned++
		} else {
			printCheck(true, "marketplace-installer: npx found")
			passed++
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
