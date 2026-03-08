package llm

import "testing"

func TestCanonicalModelRefFamilies(t *testing.T) {
	// Test all vendor families
	cases := []struct {
		raw       string
		vendor    string
		canonical string
	}{
		{"claude-opus-4-6", "anthropic", "anthropic/claude-opus-4-6"},
		{"gpt-4.1", "openai", "openai/gpt-4.1"},
		{"gpt-5.3-codex", "openai-codex", "openai-codex/gpt-5.3-codex"},
		{"gemini-2.5-pro", "google", "google/gemini-2.5-pro"},
		{"grok-3", "xai", "xai/grok-3"},
		{"llama-3.3-70b", "meta", "meta/llama-3.3-70b"},
		{"openai/gpt-4.1", "openai", "openai/gpt-4.1"},            // already qualified
		{"anthropic/claude-3", "anthropic", "anthropic/claude-3"}, // already qualified
		{"", "", ""}, // empty input
	}

	for _, c := range cases {
		ref := CanonicalizeModelID(c.raw)
		if ref.Vendor != c.vendor {
			t.Errorf("CanonicalizeModelID(%q) vendor = %q; want %q", c.raw, ref.Vendor, c.vendor)
		}
		if ref.CanonicalID != c.canonical {
			t.Errorf("CanonicalizeModelID(%q) canonicalID = %q; want %q", c.raw, ref.CanonicalID, c.canonical)
		}
	}
}

func TestCanonicalizeModelIDCodexPrecedence(t *testing.T) {
	ref := CanonicalizeModelID("gpt-5.3-codex")
	if ref.Vendor != "openai-codex" {
		t.Fatalf("vendor = %q; want %q", ref.Vendor, "openai-codex")
	}
	if ref.CanonicalID != "openai-codex/gpt-5.3-codex" {
		t.Fatalf("canonical = %q; want %q", ref.CanonicalID, "openai-codex/gpt-5.3-codex")
	}
}

func TestCanonicalizeModelIDDisplayNames(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"claude-opus-4-6", "Claude Opus 4.6"},
		{"gpt-4.1", "GPT-4.1"},
		{"gpt-5.3-codex", "GPT-5.3 Codex"},
		{"gemini-2.5-pro", "Gemini 2.5 Pro"},
		{"grok-3", "Grok 3"},
		{"llama-3.3-70b", "Llama 3.3 70b"},
		{"custom-model", "custom-model"},
	}

	for _, c := range cases {
		ref := CanonicalizeModelID(c.raw)
		if ref.DisplayName != c.want {
			t.Errorf("display name for %q = %q; want %q", c.raw, ref.DisplayName, c.want)
		}
	}
}
