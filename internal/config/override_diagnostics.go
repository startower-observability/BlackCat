package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// OverrideDiagnostic reports whether a config field is controlled by YAML, env, or a mix.
type OverrideDiagnostic struct {
	Field     string // Dot-notation field path, e.g. "providers.openai.model"
	YAMLValue string // Value from YAML (or default if no YAML)
	EnvVar    string // Environment variable name that can override this field
	EnvValue  string // Current value of the environment variable (empty if unset)
	Source    string // One of: "yaml", "env", "default", "mixed-env-wins", "mixed-yaml-wins"
}

// fieldEnvMap maps dot-notation config field paths to their environment variable names.
var fieldEnvMap = map[string]string{
	// Server
	"server.addr": "BLACKCAT_SERVER_ADDR",
	"server.port": "BLACKCAT_SERVER_PORT",

	// OpenCode
	"opencode.addr":     "BLACKCAT_OPENCODE_ADDR",
	"opencode.password": "BLACKCAT_OPENCODE_PASSWORD",
	"opencode.timeout":  "BLACKCAT_OPENCODE_TIMEOUT",

	// LLM (LEGACY: read-only compatibility, use providers.*.model for new code)
	"llm.provider":    "BLACKCAT_LLM_PROVIDER",
	"llm.model":       "BLACKCAT_LLM_MODEL",
	"llm.apiKey":      "BLACKCAT_LLM_APIKEY",
	"llm.baseURL":     "BLACKCAT_LLM_BASEURL",
	"llm.temperature": "BLACKCAT_LLM_TEMPERATURE",
	"llm.maxTokens":   "BLACKCAT_LLM_MAXTOKENS",

	// Providers — OpenAI
	"providers.openai.enabled": "BLACKCAT_PROVIDERS_OPENAI_ENABLED",
	"providers.openai.model":   "BLACKCAT_PROVIDERS_OPENAI_MODEL",
	"providers.openai.apiKey":  "BLACKCAT_PROVIDERS_OPENAI_API_KEY",
	"providers.openai.baseURL": "BLACKCAT_PROVIDERS_OPENAI_BASE_URL",

	// Providers — Copilot
	"providers.copilot.enabled": "BLACKCAT_PROVIDERS_COPILOT_ENABLED",
	"providers.copilot.model":   "BLACKCAT_PROVIDERS_COPILOT_MODEL",

	// Providers — Antigravity
	"providers.antigravity.enabled": "BLACKCAT_PROVIDERS_ANTIGRAVITY_ENABLED",
	"providers.antigravity.model":   "BLACKCAT_PROVIDERS_ANTIGRAVITY_MODEL",

	// Providers — Gemini
	"providers.gemini.enabled": "BLACKCAT_PROVIDERS_GEMINI_ENABLED",
	"providers.gemini.model":   "BLACKCAT_PROVIDERS_GEMINI_MODEL",
	"providers.gemini.apiKey":  "BLACKCAT_PROVIDERS_GEMINI_APIKEY",

	// Providers — Zen
	"providers.zen.enabled": "BLACKCAT_PROVIDERS_ZEN_ENABLED",
	"providers.zen.model":   "BLACKCAT_PROVIDERS_ZEN_MODEL",

	// Channels
	"channels.telegram.enabled": "BLACKCAT_CHANNELS_TELEGRAM_ENABLED",
	"channels.telegram.token":   "BLACKCAT_CHANNELS_TELEGRAM_TOKEN",
	"channels.discord.enabled":  "BLACKCAT_CHANNELS_DISCORD_ENABLED",
	"channels.discord.token":    "BLACKCAT_CHANNELS_DISCORD_TOKEN",
	"channels.whatsapp.enabled": "BLACKCAT_CHANNELS_WHATSAPP_ENABLED",
	"channels.whatsapp.token":   "BLACKCAT_CHANNELS_WHATSAPP_TOKEN",

	// Logging
	"logging.level":  "BLACKCAT_LOGGING_LEVEL",
	"logging.format": "BLACKCAT_LOGGING_FORMAT",

	// Zen (top-level)
	"zen.enabled": "BLACKCAT_ZEN_ENABLED",
	"zen.apiKey":  "BLACKCAT_ZEN_APIKEY",
	"zen.baseURL": "BLACKCAT_ZEN_BASEURL",
}

// GetFieldSourceDiagnostics returns an OverrideDiagnostic for the given config field.
// The cfg parameter should be the loaded (post-merge) config, and field is a
// dot-notation path like "providers.openai.model" or "llm.provider".
func GetFieldSourceDiagnostics(cfg *Config, field string) OverrideDiagnostic {
	diag := OverrideDiagnostic{
		Field: field,
	}

	// Look up the env var for this field
	envVar, known := fieldEnvMap[field]
	if known {
		diag.EnvVar = envVar
		diag.EnvValue = os.Getenv(envVar)
	}

	// Get the YAML value from the defaults to compare
	defaults := Defaults()
	defaultVal := resolveFieldValue(defaults, field)
	yamlVal := resolveFieldValue(cfg, field)
	diag.YAMLValue = yamlVal

	envSet := known && diag.EnvValue != ""

	switch {
	case !envSet && yamlVal == defaultVal:
		diag.Source = "default"
	case !envSet && yamlVal != defaultVal:
		diag.Source = "yaml"
	case envSet && (yamlVal == defaultVal || yamlVal == ""):
		diag.Source = "env"
	case envSet && yamlVal != defaultVal && yamlVal != diag.EnvValue:
		// Both YAML and env are set to different values; env wins in our merge
		diag.Source = "mixed-env-wins"
	case envSet && yamlVal == diag.EnvValue:
		// Both agree (or env overwrote yaml already in merge)
		diag.Source = "env"
	default:
		diag.Source = "default"
	}

	return diag
}

// resolveFieldValue navigates the Config struct using a dot-notation field path
// and returns the string representation of the value.
func resolveFieldValue(cfg *Config, field string) string {
	parts := strings.Split(field, ".")
	v := reflect.ValueOf(cfg).Elem()

	for _, part := range parts {
		if v.Kind() == reflect.Struct {
			v = findFieldByYAMLTag(v, part)
			if !v.IsValid() {
				return ""
			}
		} else {
			return ""
		}
	}

	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case reflect.Int, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	default:
		return ""
	}
}

// findFieldByYAMLTag finds a struct field by its yaml tag name.
func findFieldByYAMLTag(v reflect.Value, tag string) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		yamlTag := t.Field(i).Tag.Get("yaml")
		// Handle tags like `yaml:"model,omitempty"`
		if idx := strings.Index(yamlTag, ","); idx != -1 {
			yamlTag = yamlTag[:idx]
		}
		if yamlTag == tag {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}
