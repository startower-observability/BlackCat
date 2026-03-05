package agent

import (
	"strings"
	"unicode/utf8"
)

// ambiguousKeywords are phrases that indicate a vague or underspecified request.
var ambiguousKeywords = []string{
	"something",
	"stuff",
	"things",
	"whatever",
	"somehow",
	"some kind of",
	"anything",
	"you know what i mean",
	"the usual",
	"do the thing",
	"make it work",
	"fix everything",
	"help me with",
	"i need help",
	"do it",
}

// pronounsWithoutContext are standalone pronoun-heavy phrases that lack specificity.
var pronounsWithoutContext = []string{
	"do that",
	"change it",
	"update it",
	"fix it",
	"run it",
	"delete it",
	"move it",
	"send it",
	"check it",
	"make that",
}

// IsAmbiguous returns true if the user message appears vague or underspecified,
// suggesting the agent should ask clarifying questions before acting.
// It uses lightweight heuristics — no LLM call required.
func IsAmbiguous(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return true
	}

	lower := strings.ToLower(trimmed)

	// Very short messages (under 10 characters, excluding simple greetings) are ambiguous
	charCount := utf8.RuneCountInString(trimmed)
	if charCount < 10 {
		// Allow simple greetings and common short commands
		shortAllowed := []string{
			"hello", "hi", "hey", "halo", "/status", "status", "/help", "help",
			"/ping", "ping", "yes", "no", "ok", "stop", "restart",
		}
		for _, allowed := range shortAllowed {
			if lower == allowed {
				return false
			}
		}
		// Very short non-greeting is ambiguous
		return true
	}

	// Check for vague keyword phrases
	for _, kw := range ambiguousKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	// Check for standalone pronoun phrases (exact match or at start/end of message)
	for _, phr := range pronounsWithoutContext {
		if lower == phr {
			return true
		}
		if strings.HasPrefix(lower, phr+" ") || strings.HasSuffix(lower, " "+phr) {
			return true
		}
	}

	return false
}

// ClarificationPromptSection returns a system prompt section instructing the agent
// to ask clarifying questions when the request is ambiguous. Returns "" if the
// message is not ambiguous.
func ClarificationPromptSection(userMessage string) string {
	if !IsAmbiguous(userMessage) {
		return ""
	}

	return `# Clarification Required
The user's request appears vague or incomplete. Before taking any action:
1. Identify what specific information is missing (e.g., which file, what kind of change, target environment).
2. Ask 1-3 focused clarifying questions.
3. Do NOT execute tools or make assumptions until the user clarifies.
4. Keep your questions concise and specific.`
}
