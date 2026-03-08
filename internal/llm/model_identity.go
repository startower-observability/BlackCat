package llm

import "strings"

// CanonicalModelRef is the normalized representation of a model identifier.
// It is the single source of truth for how BlackCat talks about models.
type CanonicalModelRef struct {
	// CanonicalID is the authoritative identifier in "vendor/model-name" format.
	// Examples: "anthropic/claude-opus-4-6", "openai/gpt-4.1", "openai-codex/gpt-5.3-codex",
	//           "google/gemini-2.5-pro", "xai/grok-3", "meta/llama-3.3-70b"
	// Already-slash-qualified IDs are passed through unchanged.
	CanonicalID string

	// Vendor is the model family prefix (e.g. "anthropic", "openai", "openai-codex",
	// "google", "xai", "meta", "unknown")
	Vendor string

	// RawModel is the original model ID string before normalization (e.g. "claude-opus-4-6",
	// "gpt-4.1", "gemini-2.5-pro")
	RawModel string

	// DisplayName is a human-friendly label (e.g. "Claude Opus 4.6", "GPT-4.1")
	DisplayName string

	// BackendProvider is the BlackCat internal backend key this model runs on
	// (e.g. "openai", "copilot", "gemini", "zen", "antigravity")
	BackendProvider string

	// SourceProvider is which provider returned this model during discovery
	// (e.g. "copilot", "openai", "gemini")
	SourceProvider string
}

// CanonicalizeModelID converts a raw model ID string into a CanonicalModelRef.
// Vendor-mapping rules (applied in order):
//   - Already contains "/" → pass through (BackendProvider and SourceProvider left empty)
//   - Prefix "claude-" → Vendor "anthropic", CanonicalID "anthropic/{raw}"
//   - Prefix "gpt-" or "o1" or "o3" or "o4" or "text-embedding-" → Vendor "openai", CanonicalID "openai/{raw}"
//   - Suffix "-codex" → Vendor "openai-codex", CanonicalID "openai-codex/{raw}"
//   - Prefix "gemini-" → Vendor "google", CanonicalID "google/{raw}"
//   - Prefix "grok-" → Vendor "xai", CanonicalID "xai/{raw}"
//   - Prefix "llama-" → Vendor "meta", CanonicalID "meta/{raw}"
//   - Unknown → Vendor "unknown", CanonicalID "unknown/{raw}"
//
// DisplayName is derived by title-casing vendor + formatting model version.
func CanonicalizeModelID(rawID string) CanonicalModelRef {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return CanonicalModelRef{}
	}

	if strings.Contains(rawID, "/") {
		parts := strings.SplitN(rawID, "/", 2)
		vendor := parts[0]
		rawModel := ""
		if len(parts) > 1 {
			rawModel = parts[1]
		}
		return CanonicalModelRef{
			CanonicalID: rawID,
			Vendor:      vendor,
			RawModel:    rawModel,
			DisplayName: displayNameFor(vendor, rawModel, rawID),
		}
	}

	vendor := "unknown"
	switch {
	case strings.HasSuffix(rawID, "-codex"):
		vendor = "openai-codex"
	case strings.HasPrefix(rawID, "claude-"):
		vendor = "anthropic"
	case strings.HasPrefix(rawID, "gpt-") || strings.HasPrefix(rawID, "o1") || strings.HasPrefix(rawID, "o3") || strings.HasPrefix(rawID, "o4") || strings.HasPrefix(rawID, "text-embedding-"):
		vendor = "openai"
	case strings.HasPrefix(rawID, "gemini-"):
		vendor = "google"
	case strings.HasPrefix(rawID, "grok-"):
		vendor = "xai"
	case strings.HasPrefix(rawID, "llama-"):
		vendor = "meta"
	}

	canonicalID := vendor + "/" + rawID
	return CanonicalModelRef{
		CanonicalID: canonicalID,
		Vendor:      vendor,
		RawModel:    rawID,
		DisplayName: displayNameFor(vendor, rawID, rawID),
	}
}

func displayNameFor(vendor, rawModel, fallback string) string {
	if rawModel == "" {
		return fallback
	}

	switch vendor {
	case "anthropic":
		return "Claude " + formatModelWords(strings.TrimPrefix(rawModel, "claude-"))
	case "openai", "openai-codex":
		switch {
		case strings.HasPrefix(rawModel, "gpt-"):
			v := strings.TrimPrefix(rawModel, "gpt-")
			v = strings.TrimSuffix(v, "-codex")
			if strings.HasSuffix(rawModel, "-codex") {
				return "GPT-" + v + " Codex"
			}
			return "GPT-" + v
		case strings.HasPrefix(rawModel, "text-embedding-"):
			return "Text Embedding " + formatModelWords(strings.TrimPrefix(rawModel, "text-embedding-"))
		default:
			return strings.ToUpper(rawModel)
		}
	case "google":
		return "Gemini " + formatModelWords(strings.TrimPrefix(rawModel, "gemini-"))
	case "xai":
		return "Grok " + formatModelWords(strings.TrimPrefix(rawModel, "grok-"))
	case "meta":
		return "Llama " + formatModelWords(strings.TrimPrefix(rawModel, "llama-"))
	default:
		return fallback
	}
}

func formatModelWords(raw string) string {
	if raw == "" {
		return ""
	}

	parts := strings.Split(raw, "-")
	if len(parts) == 0 {
		return raw
	}

	formatted := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		curr := parts[i]
		if curr == "" {
			continue
		}

		if i+1 < len(parts) && isDigits(curr) && isDigits(parts[i+1]) {
			formatted = append(formatted, curr+"."+parts[i+1])
			i++
			continue
		}

		if isDigits(curr) {
			formatted = append(formatted, curr)
			continue
		}

		formatted = append(formatted, strings.ToUpper(curr[:1])+curr[1:])
	}

	return strings.Join(formatted, " ")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
