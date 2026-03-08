package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/oauth"
	"github.com/startower-observability/blackcat/internal/security"
)

var (
	configureProvider string
	configureAPIKey   string
	configureModel    string
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure BlackCat LLM providers interactively",
	Long: `configure sets up BlackCat LLM provider authentication
via an interactive wizard or non-interactive flags.

Supported providers:
  openai, anthropic, copilot, antigravity, gemini, zen, openrouter, ollama

Examples:
  # Interactive wizard
  blackcat configure

  # Non-interactive: set OpenAI API key
  blackcat configure --provider openai --api-key sk-... --model gpt-4o

  # Non-interactive: set up Copilot (triggers device flow)
  blackcat configure --provider copilot`,
	RunE: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
	configureCmd.Flags().StringVar(&configureProvider, "provider", "", "Provider to configure (non-interactive mode)")
	configureCmd.Flags().StringVar(&configureAPIKey, "api-key", "", "API key for the provider")
	configureCmd.Flags().StringVar(&configureModel, "model", "", "Model to use with the provider")
}

// providerOption describes a provider for the interactive wizard.
type providerOption struct {
	Name         string
	AuthMethod   string // "api-key", "oauth-device", "oauth-pkce", "none"
	DefaultModel string
}

var allProviders = []providerOption{
	{Name: "openai", AuthMethod: "api-key", DefaultModel: "gpt-5.2"},
	{Name: "anthropic", AuthMethod: "api-key", DefaultModel: "claude-sonnet-4-6"},
	{Name: "copilot", AuthMethod: "oauth-device", DefaultModel: "gpt-4.1"},
	{Name: "antigravity", AuthMethod: "oauth-pkce", DefaultModel: "gemini-2.5-pro"},
	{Name: "gemini", AuthMethod: "api-key", DefaultModel: "gemini-2.5-flash"},
	{Name: "zen", AuthMethod: "api-key", DefaultModel: "opencode/claude-sonnet-4-6"},
	{Name: "openrouter", AuthMethod: "api-key", DefaultModel: ""},
	{Name: "ollama", AuthMethod: "none", DefaultModel: "llama3.3"},
}

func runConfigure(cmd *cobra.Command, args []string) error {
	// Non-interactive mode
	if configureProvider != "" {
		return configureNonInteractive(cmd)
	}

	// Interactive mode
	return configureInteractive(cmd)
}

func configureInteractive(cmd *cobra.Command) error {
	fmt.Println("BlackCat Configuration Wizard")
	fmt.Println("=================================")
	fmt.Println()

	// Step 1: Provider selection
	options := make([]huh.Option[string], 0, len(allProviders))
	for _, p := range allProviders {
		label := fmt.Sprintf("%s (%s)", p.Name, p.AuthMethod)
		options = append(options, huh.NewOption(label, p.Name))
	}

	var selectedProviders []string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select LLM providers to configure").
				Options(options...).
				Value(&selectedProviders),
		),
	).Run()
	if err != nil {
		return fmt.Errorf("provider selection: %w", err)
	}

	if len(selectedProviders) == 0 {
		fmt.Println("No providers selected. Exiting.")
		return nil
	}

	// Step 2: Configure each selected provider
	for _, providerName := range selectedProviders {
		provider := findProvider(providerName)
		if provider == nil {
			continue
		}

		fmt.Printf("\nConfiguring %s...\n", provider.Name)

		if err := configureProviderInteractive(cmd, provider); err != nil {
			fmt.Printf("Warning: failed to configure %s: %v\n", provider.Name, err)
			continue
		}

		fmt.Printf("%s configured successfully.\n", provider.Name)
	}

	// Step 3: Summary
	fmt.Println()
	fmt.Println("Configuration Summary")
	fmt.Println("---------------------")
	for _, name := range selectedProviders {
		fmt.Printf("  [+] %s\n", name)
	}

	// Step 4: Next steps
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review config: cat ~/.blackcat/config.yaml")
	fmt.Println("  2. Start the daemon: blackcat daemon")
	fmt.Println("  3. Send a message via Telegram or Discord")
	fmt.Println()

	return nil
}

func configureProviderInteractive(cmd *cobra.Command, provider *providerOption) error {
	switch provider.AuthMethod {
	case "api-key":
		return configureAPIKeyProvider(cmd, provider)
	case "oauth-device":
		return configureCopilotAuth(cmd, provider)
	case "oauth-pkce":
		return configureAntigravityAuth(cmd, provider)
	case "none":
		return configureNoAuthProvider(cmd, provider)
	default:
		return fmt.Errorf("unknown auth method: %s", provider.AuthMethod)
	}
}

func configureAPIKeyProvider(cmd *cobra.Command, provider *providerOption) error {
	var apiKey, model string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("%s API Key", provider.Name)).
				Placeholder("sk-...").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
			huh.NewInput().
				Title("Model").
				Placeholder(provider.DefaultModel).
				Value(&model),
		),
	).Run()
	if err != nil {
		return err
	}

	if model == "" {
		model = provider.DefaultModel
	}

	return saveProviderConfig(cmd, provider.Name, apiKey, model)
}

func configureCopilotAuth(cmd *cobra.Command, provider *providerOption) error {
	fmt.Println("Starting GitHub Copilot device flow...")
	fmt.Println("This will open a browser for GitHub authentication.")

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	clientID := cfg.OAuth.Copilot.ClientID
	if clientID == "" {
		clientID = "01ab8ac9400c4e429b23"
	}

	deviceCfg := oauth.DeviceFlowConfig{
		ClientID:      clientID,
		Scopes:        []string{"read:user"},
		DeviceCodeURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
		PollInterval:  5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	codeResp, err := oauth.RequestDeviceCode(ctx, deviceCfg)
	if err != nil {
		return fmt.Errorf("device code request: %w", err)
	}

	fmt.Printf("\nPlease visit: %s\n", codeResp.VerificationURI)
	fmt.Printf("Enter code:   %s\n\n", codeResp.UserCode)
	fmt.Println("Waiting for authentication...")

	tokenSet, err := oauth.PollForToken(ctx, deviceCfg, codeResp.DeviceCode)
	if err != nil {
		return fmt.Errorf("token poll: %w", err)
	}

	// Save token to vault
	vault, err := openVault(cmd)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}

	tokenData, _ := json.Marshal(tokenSet)
	if err := vault.Set("oauth.copilot", string(tokenData)); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	var model string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Copilot Model").
				Placeholder(provider.DefaultModel).
				Value(&model),
		),
	).Run()
	if err != nil {
		return err
	}

	if model == "" {
		model = provider.DefaultModel
	}

	return saveProviderYAML(provider.Name, "", model)
}

func configureAntigravityAuth(cmd *cobra.Command, provider *providerOption) error {
	fmt.Println("Antigravity uses Google's internal API.")
	fmt.Println("WARNING: This may violate Google's Terms of Service.")

	var accepted bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Accept ToS risk?").
				Affirmative("Yes, I accept").
				Negative("No, cancel").
				Value(&accepted),
		),
	).Run()
	if err != nil {
		return err
	}

	if !accepted {
		fmt.Println("Antigravity setup cancelled.")
		return nil
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("Starting browser PKCE flow...")
	fmt.Println("A browser window will open for Google authentication.")

	pkceCfg := oauth.PKCEConfig{
		ClientID:     cfg.OAuth.Antigravity.ClientID,
		ClientSecret: cfg.OAuth.Antigravity.ClientSecret,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		RedirectURL:  "http://127.0.0.1:0/oauth-callback",
		Scopes:       []string{"openid", "https://www.googleapis.com/auth/cloud-platform"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tokenSet, err := oauth.RunPKCEFlow(ctx, pkceCfg, func(url string) {
		fmt.Printf("\nOpen this URL in your browser:\n  %s\n\n", url)
	})
	if err != nil {
		return fmt.Errorf("PKCE flow: %w", err)
	}

	vault, err := openVault(cmd)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}

	tokenData, _ := json.Marshal(tokenSet)
	if err := vault.Set("oauth.antigravity", string(tokenData)); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	var model string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Antigravity Model").
				Placeholder(provider.DefaultModel).
				Value(&model),
		),
	).Run()
	if err != nil {
		return err
	}

	if model == "" {
		model = provider.DefaultModel
	}

	return saveProviderYAML(provider.Name, "", model)
}

func configureNoAuthProvider(cmd *cobra.Command, provider *providerOption) error {
	var endpoint, model string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Endpoint URL").
				Placeholder("http://localhost:11434/v1").
				Value(&endpoint),
			huh.NewInput().
				Title("Model").
				Placeholder(provider.DefaultModel).
				Value(&model),
		),
	).Run()
	if err != nil {
		return err
	}

	if model == "" {
		model = provider.DefaultModel
	}

	// For endpoint-only providers, save to config YAML
	return saveProviderYAML(provider.Name, endpoint, model)
}

func configureNonInteractive(cmd *cobra.Command) error {
	provider := findProvider(configureProvider)
	if provider == nil {
		return fmt.Errorf("unknown provider: %s (available: openai, anthropic, copilot, antigravity, gemini, zen, openrouter, ollama)", configureProvider)
	}

	model := configureModel
	if model == "" {
		model = provider.DefaultModel
	}

	switch provider.AuthMethod {
	case "api-key":
		if configureAPIKey == "" {
			return fmt.Errorf("--api-key is required for %s", provider.Name)
		}
		return saveProviderConfig(cmd, provider.Name, configureAPIKey, model)
	case "oauth-device":
		return configureCopilotAuth(cmd, provider)
	case "oauth-pkce":
		return configureAntigravityAuth(cmd, provider)
	case "none":
		return saveProviderYAML(provider.Name, "", model)
	default:
		return fmt.Errorf("unsupported auth method: %s", provider.AuthMethod)
	}
}

// saveProviderConfig saves API key to vault and model to config.
func saveProviderConfig(cmd *cobra.Command, name, apiKey, model string) error {
	if apiKey != "" {
		vault, err := openVault(cmd)
		if err != nil {
			// Fall back: try to create vault directory and retry
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return fmt.Errorf("open vault: %w", err)
			}
			vaultDir := filepath.Join(home, ".blackcat")
			if mkErr := os.MkdirAll(vaultDir, 0o700); mkErr != nil {
				return fmt.Errorf("open vault: %w", err)
			}

			// Retry
			vault, err = openVault(cmd)
			if err != nil {
				return fmt.Errorf("open vault: %w", err)
			}
		}

		vaultKey := fmt.Sprintf("provider.%s.apikey", name)
		if err := vault.Set(vaultKey, apiKey); err != nil {
			return fmt.Errorf("save API key to vault: %w", err)
		}
		slog.Info("API key saved to vault", "provider", name, "key", vaultKey)
	}

	return saveProviderYAML(name, "", model)
}

// saveProviderYAML updates or creates the config YAML with provider settings.
func saveProviderYAML(name, endpoint, model string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".blackcat")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	// Load existing config or create new
	cfg, _ := config.Load(configPath)
	if cfg == nil {
		cfg = config.Defaults()
	}

	// Update config based on provider
	switch strings.ToLower(name) {
	case "openai":
		// Authoritative path: providers.openai.*
		cfg.Providers.OpenAI.Enabled = true
		if model != "" {
			cfg.Providers.OpenAI.Model = model
		}
		// LEGACY: backward-compat writes for llm.provider / llm.model
		cfg.LLM.Provider = "openai"
		if model != "" {
			cfg.LLM.Model = model
		}
	case "anthropic":
		cfg.LLM.Provider = "anthropic"
		if model != "" {
			cfg.LLM.Model = model
		}
	case "copilot":
		cfg.Providers.Copilot.Enabled = true
		cfg.OAuth.Copilot.Enabled = true
		if model != "" {
			cfg.Providers.Copilot.Model = model
		}
	case "antigravity":
		cfg.Providers.Antigravity.Enabled = true
		cfg.OAuth.Antigravity.Enabled = true
		cfg.OAuth.Antigravity.AcceptedToS = true
		if model != "" {
			cfg.Providers.Antigravity.Model = model
		}
	case "gemini":
		cfg.Providers.Gemini.Enabled = true
		if model != "" {
			cfg.Providers.Gemini.Model = model
		}
	case "zen":
		cfg.Zen.Enabled = true
		cfg.Providers.Zen.Enabled = true
		if model != "" {
			cfg.Providers.Zen.Model = model
		}
	case "openrouter":
		cfg.LLM.Provider = "openrouter"
		if endpoint != "" {
			cfg.LLM.BaseURL = endpoint
		}
		if model != "" {
			cfg.LLM.Model = model
		}
	case "ollama":
		cfg.LLM.Provider = "ollama"
		if endpoint != "" {
			cfg.LLM.BaseURL = endpoint
		} else {
			cfg.LLM.BaseURL = "http://localhost:11434/v1"
		}
		if model != "" {
			cfg.LLM.Model = model
		}
	}

	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Provider '%s' configured (model: %s)\n", name, model)
	return nil
}

// openVaultForConfigure opens the vault using standard flags or defaults.
func openVaultForConfigure(passphrase string) (*security.Vault, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	vaultPath := filepath.Join(home, ".blackcat", "vault.json")
	if err := os.MkdirAll(filepath.Dir(vaultPath), 0o700); err != nil {
		return nil, fmt.Errorf("create vault directory: %w", err)
	}

	return security.NewVault(vaultPath, passphrase)
}

func findProvider(name string) *providerOption {
	for i := range allProviders {
		if strings.EqualFold(allProviders[i].Name, name) {
			return &allProviders[i]
		}
	}
	return nil
}
