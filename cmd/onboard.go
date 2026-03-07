package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/startower-observability/blackcat/internal/service"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive setup wizard for BlackCat",
	Long: `onboard walks you through the complete BlackCat setup:
  1. Configure an LLM provider
  2. Set up a messaging channel
  3. Install and start the daemon
  4. Run a health check
  5. Check built-in skills prerequisites

Run this after a fresh install to get BlackCat working in minutes.`,
	RunE: runOnboard,
}

func init() {
	rootCmd.AddCommand(onboardCmd)
	onboardCmd.Flags().Bool("non-interactive", false, "Skip all prompts (for CI/scripted use)")
}

func runOnboard(cmd *cobra.Command, args []string) error {
	// Print banner
	fmt.Println()
	fmt.Println("  ▄▄▄▄    ██▓    ▄▄▄       ▄████▄   ██ ▄█▀ ▄████▄  ▄▄▄     ▄▄▄█████▓")
	fmt.Println("  ▓█████▄ ▓██▒   ▒████▄    ▒██▀ ▀█   ██▄█▒ ▒██▀ ▀█ ▒████▄   ▓  ██▒ ▓▒")
	fmt.Println("  ▒██▒ ▄██▒██░   ▒██  ▀█▄  ▒▓█    ▄ ▓███▄░ ▒▓█    ▄▒██  ▀█▄ ▒ ▓██░ ▒░")
	fmt.Println("  ▒██░█▀  ▒██░   ░██▄▄▄▄██ ▒▓▓▄ ▄██▒▓██ █▄ ▒▓▓▄ ▄██░██▄▄▄▄██░ ▓██▓ ░")
	fmt.Println("  ░▓█  ▀█▓░██████▒▓█   ▓██▒▒ ▓███▀ ░▒██▒ █▄▒ ▓███▀ ░▓█   ▓██▒ ▒██▒ ░")
	fmt.Println("  ░▒▓███▀▒░ ▒░▓  ░▒▒   ▓▒█░░ ░▒ ▒  ░▒ ▒▒ ▓▒░ ░▒ ▒  ░▒▒   ▓▒█░ ▒ ░░")
	fmt.Println("  ▒░▒   ░ ░ ░ ▒  ░ ▒   ▒▒ ░  ░  ▒   ░ ░▒ ▒░  ░  ▒    ▒   ▒▒ ░   ░")
	fmt.Println()
	fmt.Println("  Welcome to BlackCat — AI agent for your messaging channels")
	fmt.Println("  ─────────────────────────────────────────────────────────")
	fmt.Println()

	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	// Step 1: LLM Provider
	fmt.Println("Step 1/5: Configure LLM Provider")
	fmt.Println("─────────────────────────────────")
	if !nonInteractive {
		if err := configureInteractive(cmd); err != nil {
			fmt.Printf("  ⚠ Provider setup skipped: %v\n", err)
		}
	} else {
		fmt.Println("  (skipped — non-interactive mode)")
	}
	fmt.Println()

	// Step 2: Channel Setup
	fmt.Println("Step 2/5: Set Up a Messaging Channel")
	fmt.Println("─────────────────────────────────────")
	if !nonInteractive {
		if err := onboardChannel(cmd); err != nil {
			fmt.Printf("  ⚠ Channel setup skipped: %v\n", err)
		}
	} else {
		fmt.Println("  (skipped — non-interactive mode)")
	}
	fmt.Println()

	// Step 3: Daemon Install + Start
	fmt.Println("Step 3/5: Install and Start Daemon")
	fmt.Println("───────────────────────────────────")
	if err := onboardDaemon(cmd, nonInteractive); err != nil {
		fmt.Printf("  ⚠ Daemon setup: %v\n", err)
	}
	fmt.Println()

	// Step 4: Health Check
	fmt.Println("Step 4/5: Health Check")
	fmt.Println("───────────────────────")
	onboardHealthCheck()
	fmt.Println()

	// Step 5: Built-in Skills Setup
	onboardSkills(nonInteractive)
	fmt.Println()

	// Done
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println("  BlackCat setup complete!")
	fmt.Println()
	fmt.Println("  Quick reference:")
	fmt.Println("    blackcat start       — start daemon")
	fmt.Println("    blackcat stop        — stop daemon")
	fmt.Println("    blackcat status      — check status")
	fmt.Println("    blackcat channels list  — list channels")
	fmt.Println("    blackcat doctor      — run diagnostics")
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println()
	return nil
}

func onboardChannel(cmd *cobra.Command) error {
	var channelChoice string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which channel do you want to set up?").
				Options(
					huh.NewOption("Telegram (recommended — easy token setup)", "telegram"),
					huh.NewOption("Discord", "discord"),
					huh.NewOption("WhatsApp (requires CGO build)", "whatsapp"),
					huh.NewOption("Skip for now", "skip"),
				).
				Value(&channelChoice),
		),
	).Run()
	if err != nil {
		return err
	}
	switch channelChoice {
	case "skip":
		fmt.Println("  Channel setup skipped. Run: blackcat channels login --channel <name>")
		return nil
	case "telegram":
		return loginTelegram(cmd)
	case "discord":
		return loginDiscord(cmd)
	case "whatsapp":
		return loginWhatsApp(cmd, nil)
	}
	return nil
}

func onboardDaemon(cmd *cobra.Command, nonInteractive bool) error {
	svc := service.New()

	if svc.IsInstalled() {
		fmt.Println("  Daemon already installed.")
	} else {
		home, _ := os.UserHomeDir()
		binaryPath := filepath.Join(home, ".blackcat", "bin", "blackcat")
		// Fall back to current executable if not at user-space path
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			binaryPath, _ = os.Executable()
		}
		configPath := filepath.Join(home, ".blackcat", "config.yaml")
		cfg := service.DefaultConfig()
		cfg.BinaryPath = binaryPath
		cfg.ConfigPath = configPath
		if err := svc.Install(cfg); err != nil {
			return fmt.Errorf("install service: %w", err)
		}
		fmt.Println("  ✓ Daemon installed")
	}

	status, _ := svc.Status()
	if status.Running {
		fmt.Println("  Daemon already running.")
		return nil
	}

	if !nonInteractive {
		var startNow bool
		_ = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Start the BlackCat daemon now?").
					Value(&startNow),
			),
		).Run()
		if !startNow {
			fmt.Println("  Start later with: blackcat start")
			return nil
		}
	}

	if err := svc.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	fmt.Println("  ✓ Daemon started")
	return nil
}

func onboardHealthCheck() {
	fmt.Print("  Checking daemon health... ")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/health")
	if err != nil {
		fmt.Println("⚠ Daemon not reachable (may still be starting up)")
		fmt.Println("  Run: blackcat status  to check later")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		fmt.Println("✓ Healthy")
	} else {
		fmt.Printf("⚠ Status %d\n", resp.StatusCode)
	}
}

// skillCheck holds the result of a single prerequisite check.
type skillCheck struct {
	name   string
	ready  bool
	detail string
}

func checkBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func checkEnv(name string) bool {
	return os.Getenv(name) != ""
}

// skillStatus prints a summary of all built-in skill prerequisites and
// returns true if every skill is fully ready.
func skillStatus() bool {
	fmt.Println("  Checking skill prerequisites...")
	fmt.Println()

	allReady := true

	// --- Whisper ---
	whisperReady := checkEnv("BLACKCAT_WHISPER_GROQAPIKEY")
	mark := "✓"
	detail := "ready"
	if !whisperReady {
		mark = "⚠"
		detail = "set whisper.enabled: true + BLACKCAT_WHISPER_GROQAPIKEY"
		allReady = false
	}
	fmt.Printf("  Voice Transcription (Whisper):\n    %s %s\n", mark, detail)
	fmt.Println()

	// --- Social Media ---
	fmt.Println("  Social Media Skills:")
	socials := []skillCheck{
		{"Threads", checkEnv("THREADS_ACCESS_TOKEN"),
			"THREADS_ACCESS_TOKEN not set"},
		{"Twitter/X", checkBin("bird") && checkEnv("TWITTER_AUTH_TOKEN"),
			""},
		{"LinkedIn", checkEnv("LINKEDIN_LI_AT") && checkEnv("LINKEDIN_JSESSIONID"),
			"LINKEDIN_LI_AT / LINKEDIN_JSESSIONID not set"},
		{"Facebook", checkEnv("FACEBOOK_PAGE_TOKEN"),
			"FACEBOOK_PAGE_TOKEN not set"},
		{"TikTok", checkEnv("TIKTOK_ACCESS_TOKEN"),
			"TIKTOK_ACCESS_TOKEN not set"},
		{"Google Workspace", checkBin("gws"),
			"gws CLI missing"},
	}
	// Build Twitter detail dynamically
	{
		var parts []string
		if !checkBin("bird") {
			parts = append(parts, "bird CLI missing")
		}
		if !checkEnv("TWITTER_AUTH_TOKEN") {
			parts = append(parts, "TWITTER_AUTH_TOKEN not set")
		}
		if len(parts) > 0 {
			socials[1].detail = joinParts(parts)
		}
	}
	// Build Google Workspace detail
	if !checkBin("gws") {
		socials[5].detail = "gws CLI missing"
	}
	for _, s := range socials {
		if s.ready {
			fmt.Printf("    ✓ %-18s ready\n", s.name+":")
		} else {
			allReady = false
			fmt.Printf("    ⚠ %-18s %s\n", s.name+":", s.detail)
		}
	}
	fmt.Println()

	// --- Phase 2 ---
	fmt.Println("  Phase 2 Skills:")
	type p2 struct {
		name  string
		ready bool
		info  string
	}
	// Veo3 / nano-banana
	veoReady := checkBin("uv") && checkEnv("GEMINI_API_KEY")
	veoDetail := "ready"
	if !veoReady {
		var parts []string
		if !checkBin("uv") {
			parts = append(parts, "uv missing")
		}
		if !checkEnv("GEMINI_API_KEY") {
			parts = append(parts, "GEMINI_API_KEY not set")
		}
		veoDetail = joinParts(parts)
	}
	phase2 := []p2{
		{"Veo3 / nano-banana", veoReady, veoDetail},
		{"document-processing", checkBin("python3") || checkBin("python"), ""},
		{"capability-evolver", checkBin("node") || checkBin("nodejs"), ""},
		{"reddit-scraper", checkBin("python3") || checkBin("python"), ""},
		{"prompt-guard", checkBin("python3") || checkBin("python"), ""},
		{"marketplace-installer", checkBin("npx"), ""},
	}
	// Fill in details for binary checks
	for i := range phase2 {
		if phase2[i].ready && phase2[i].info == "" {
			switch phase2[i].name {
			case "document-processing", "reddit-scraper", "prompt-guard":
				phase2[i].info = "python3 found"
			case "capability-evolver":
				phase2[i].info = "node found"
			case "marketplace-installer":
				phase2[i].info = "npx found"
			}
		}
		if !phase2[i].ready && phase2[i].info == "" {
			switch phase2[i].name {
			case "document-processing", "reddit-scraper", "prompt-guard":
				phase2[i].info = "python3 not found"
			case "capability-evolver":
				phase2[i].info = "node not found"
			case "marketplace-installer":
				phase2[i].info = "npx not found"
			}
		}
	}
	for _, s := range phase2 {
		if s.ready {
			fmt.Printf("    ✓ %-22s %s\n", s.name+":", s.info)
		} else {
			allReady = false
			fmt.Printf("    ⚠ %-22s %s\n", s.name+":", s.info)
		}
	}

	return allReady
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += " + " + p
	}
	return result
}

func onboardSkills(nonInteractive bool) {
	fmt.Println("Step 5/5: Built-in Skills Setup")
	fmt.Println("─────────────────────────────────")
	allReady := skillStatus()
	fmt.Println()

	if nonInteractive {
		fmt.Println("  (skipped interactive setup — non-interactive mode)")
		fmt.Println("  Run 'blackcat doctor' to verify prerequisites.")
		return
	}

	if allReady {
		fmt.Println("  All skills are ready — nothing to configure.")
		return
	}

	for {
		var choice string
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Would you like to set up any skills now? (you can skip and do this later)").
					Options(
						huh.NewOption("Whisper (voice transcription)", "whisper"),
						huh.NewOption("Threads", "threads"),
						huh.NewOption("Twitter/X", "twitter"),
						huh.NewOption("LinkedIn", "linkedin"),
						huh.NewOption("Facebook", "facebook"),
						huh.NewOption("TikTok", "tiktok"),
						huh.NewOption("Google Workspace", "gws"),
						huh.NewOption("Veo3 / nano-banana (Gemini image/video)", "veo3"),
						huh.NewOption("Skip — I'll set these up later", "skip"),
					).
					Value(&choice),
			),
		).Run()
		if err != nil || choice == "skip" {
			break
		}
		onboardSkill(choice)

		fmt.Println()
		var cont bool
		_ = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Set up another skill?").
					Value(&cont),
			),
		).Run()
		if !cont {
			break
		}
	}

	fmt.Println()
	fmt.Println("  Skills setup complete. Run 'blackcat doctor' to verify prerequisites.")
}

func onboardSkill(choice string) {
	fmt.Println()
	switch choice {
	case "whisper":
		fmt.Println("  To enable voice transcription:")
		fmt.Println("  1. Get a free Groq API key at: https://console.groq.com")
		fmt.Println("  2. Set the env var (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export BLACKCAT_WHISPER_GROQAPIKEY=gsk_your_key_here")
		fmt.Println("  3. Enable in config — add to ~/.blackcat/config.yaml:")
		fmt.Println("       whisper:")
		fmt.Println("         enabled: true")
		fmt.Println("         groqApiKey: \"\"  # set via env var above")
		fmt.Println("  4. Restart: blackcat restart")

	case "threads":
		fmt.Println("  To enable Threads posting:")
		fmt.Println("  1. Get a Meta Graph API token at: https://developers.facebook.com")
		fmt.Println("  2. Set the env var (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export THREADS_ACCESS_TOKEN=your_token_here")
		fmt.Println("  3. Restart: blackcat restart")

	case "twitter":
		fmt.Println("  To enable Twitter/X posting:")
		fmt.Println("  1. Install bird CLI:")
		fmt.Println("     npm install -g @steipete/bird")
		fmt.Println("  2. Get your auth token from browser cookies")
		fmt.Println("     (x.com → DevTools → Application → Cookies → auth_token)")
		fmt.Println("  3. Set the env var (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export TWITTER_AUTH_TOKEN=your_token_here")
		fmt.Println("  4. Restart: blackcat restart")

	case "linkedin":
		fmt.Println("  To enable LinkedIn posting:")
		fmt.Println("  1. Install linkedin-api:")
		fmt.Println("     pip install linkedin-api")
		fmt.Println("  2. Get your cookies from browser")
		fmt.Println("     (linkedin.com → DevTools → Application → Cookies → li_at + JSESSIONID)")
		fmt.Println("  3. Set the env vars (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export LINKEDIN_LI_AT=your_li_at_cookie")
		fmt.Println("     export LINKEDIN_JSESSIONID=your_jsessionid_cookie")
		fmt.Println("  4. Restart: blackcat restart")

	case "facebook":
		fmt.Println("  To enable Facebook Pages posting:")
		fmt.Println("  1. Get a Meta Graph API Page token at: https://developers.facebook.com")
		fmt.Println("  2. Set the env var (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export FACEBOOK_PAGE_TOKEN=your_token_here")
		fmt.Println("  3. Restart: blackcat restart")

	case "tiktok":
		fmt.Println("  To enable TikTok posting:")
		fmt.Println("  1. Get a TikTok Content API token at: https://developers.tiktok.com")
		fmt.Println("  2. Set the env var (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export TIKTOK_ACCESS_TOKEN=your_token_here")
		fmt.Println("  3. Restart: blackcat restart")

	case "gws":
		fmt.Println("  To enable Google Workspace (Calendar, Drive, Gmail, Sheets, etc.):")
		fmt.Println("  1. Install Node.js 18+ if not already installed")
		fmt.Println("  2. Install gws CLI:")
		fmt.Println("     npm install -g @googleworkspace/cli")
		fmt.Println("  3. Authenticate:")
		fmt.Println("     gws auth setup")
		fmt.Println("  4. Restart: blackcat restart")

	case "veo3":
		fmt.Println("  To enable Veo3 video generation and nano-banana image generation:")
		fmt.Println("  1. Install uv (Python package manager):")
		fmt.Println("     pip install uv")
		fmt.Println("     OR: curl -LsSf https://astral.sh/uv/install.sh | sh")
		fmt.Println("  2. Install ffmpeg (for Veo3):")
		fmt.Println("     Ubuntu/Debian: sudo apt install ffmpeg")
		fmt.Println("     macOS: brew install ffmpeg")
		fmt.Println("  3. Get a Google Gemini API key at: https://aistudio.google.com/apikey")
		fmt.Println("  4. Set the env var (add to ~/.bashrc or ~/.profile for persistence):")
		fmt.Println("     export GEMINI_API_KEY=your_key_here")
		fmt.Println("  5. Restart: blackcat restart")
	}

	fmt.Println()
	fmt.Print("  Press Enter to continue...")
	fmt.Scanln()
}
