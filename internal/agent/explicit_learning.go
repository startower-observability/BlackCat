package agent

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"unicode"
)

// UserFact represents an explicitly stated user fact or preference.
type UserFact struct {
	Key   string // e.g. "user:name", "pref:language"
	Value string // normalized value
}

// CoreMemoryWriter is the narrow interface for writing core facts.
type CoreMemoryWriter interface {
	Get(ctx context.Context, userID, key string) (string, error)
	Set(ctx context.Context, userID, key, value string) error
}

// PrefWriter is the narrow interface for writing preference keys.
type PrefWriter interface {
	UpdatePreference(ctx context.Context, userID, key, value string) error
}

const maxFactValueLen = 100

// Allowlisted keys for explicit learning.
var allowedExplicitKeys = map[string]bool{
	"user:name":            true,
	"user:nickname":        true,
	"user:timezone":        true,
	"user:locale":          true,
	"pref:language":        true,
	"pref:style":           true,
	"pref:verbosity":       true,
	"pref:technical_depth": true,
}

// Common state/adjective words that should NOT be treated as names after "i am".
var nonNameWords = map[string]bool{
	"happy": true, "sad": true, "tired": true, "confused": true, "fine": true,
	"good": true, "great": true, "okay": true, "ok": true, "well": true,
	"busy": true, "excited": true, "bored": true, "hungry": true, "sick": true,
	"sorry": true, "ready": true, "done": true, "stuck": true, "lost": true,
	"new": true, "back": true, "here": true, "interested": true, "curious": true,
	"looking": true, "trying": true, "working": true, "using": true, "running": true,
	"having": true, "getting": true, "going": true, "asking": true, "wondering": true,
	"not": true, "also": true, "just": true, "still": true, "currently": true,
	"a": true, "an": true, "the": true, "sure": true, "afraid": true,
	"able": true, "unable": true, "aware": true, "impressed": true, "pleased": true,
}

// Compiled regex patterns (all case-insensitive).
var (
	// user:name patterns — name capture stops at punctuation/conjunctions
	reMyNameIs = regexp.MustCompile(`(?i)\bmy\s+name\s+is\s+([A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+){0,2})(?:\s*[,.\-!?;]|\s+(?:and|but|or|so|then|please|can|could|would)\b|$)`)
	reIAm      = regexp.MustCompile(`(?i)\bi\s+am\s+([A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+){0,2})(?:\s*[,.\-!?;]|\s+(?:and|but|or|so|then|please|can|could|would)\b|$)`)
	reCallMe   = regexp.MustCompile(`(?i)\bcall\s+me\s+(\S+(?:\s+\S+){0,2})(?:\s*[,.\-!?;]|\s+(?:and|but|or|so|then|please|can|could|would)\b|$)`)

	// user:nickname patterns
	reMyNicknameIs = regexp.MustCompile(`(?i)\bmy\s+nickname\s+is\s+(\S+(?:\s+\S+){0,2})\b`)
	reYouCanCallMe = regexp.MustCompile(`(?i)\byou\s+can\s+call\s+me\s+(\S+(?:\s+\S+){0,2})\b`)

	// user:timezone patterns
	reMyTimezoneIs = regexp.MustCompile(`(?i)\bmy\s+timezone\s+is\s+(\S+(?:\s+\S+){0,2})\b`)
	reInTimezone   = regexp.MustCompile(`(?i)\bi(?:'m| am)\s+in\s+timezone\s+(\S+(?:\s+\S+){0,2})\b`)

	// user:locale patterns
	reIAmFrom    = regexp.MustCompile(`(?i)\bi\s+am\s+from\s+(\S+(?:\s+\S+){0,4})\b`)
	reMyLocaleIs = regexp.MustCompile(`(?i)\bmy\s+locale\s+is\s+(\S+(?:\s+\S+){0,2})\b`)

	// pref:language patterns
	reReplyIn   = regexp.MustCompile(`(?i)\breply\s+in\s+(\S+(?:\s+\S+){0,2})\b`)
	reSpeakIn   = regexp.MustCompile(`(?i)\bspeak\s+in\s+(\S+(?:\s+\S+){0,2})\b`)
	reUseLang   = regexp.MustCompile(`(?i)\buse\s+(\S+)\s+language\b`)
	reRespondIn = regexp.MustCompile(`(?i)\brespond\s+in\s+(\S+(?:\s+\S+){0,2})\b`)

	// pref:style patterns
	reUseStyle   = regexp.MustCompile(`(?i)\buse\s+(\S+)\s+style\b`)
	reWriteStyle = regexp.MustCompile(`(?i)\bwrite\s+in\s+(\S+)\s+style\b`)

	// pref:verbosity patterns
	reBeVerbosity   = regexp.MustCompile(`(?i)\bbe\s+(verbose|concise|brief|detailed)\b`)
	reKeepVerbosity = regexp.MustCompile(`(?i)\bkeep\s+it\s+(verbose|concise|brief|detailed)\b`)

	// pref:technical_depth patterns
	reExplainAtLevel = regexp.MustCompile(`(?i)\bexplain\s+at\s+(\S+)\s+level\b`)
	reLevelExplain   = regexp.MustCompile(`(?i)\b(\S+)\s+level\s+explanations?\b`)
)

// ExtractExplicitUserKnowledge extracts ONLY explicitly stated user facts from message.
// Returns empty slice if nothing explicit found.
// NO inferences, NO tone/style detection.
func ExtractExplicitUserKnowledge(msg string) []UserFact {
	if msg == "" {
		return nil
	}

	// Deduplicate by key — only first match per key wins.
	seen := make(map[string]bool)
	var facts []UserFact

	addFact := func(key, value string) {
		if seen[key] {
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if len(value) > maxFactValueLen {
			value = value[:maxFactValueLen]
		}
		// Normalize: titlecase for user: keys, lowercase for pref: keys
		if strings.HasPrefix(key, "user:") {
			value = titleCase(value)
		} else {
			value = strings.ToLower(value)
		}
		seen[key] = true
		facts = append(facts, UserFact{Key: key, Value: value})
	}

	// --- user:nickname (highest priority, before user:name) ---
	if m := reMyNicknameIs.FindStringSubmatch(msg); m != nil {
		addFact("user:nickname", m[1])
	}
	if m := reYouCanCallMe.FindStringSubmatch(msg); m != nil {
		addFact("user:nickname", m[1])
	}

	// --- user:name ---
	if m := reMyNameIs.FindStringSubmatch(msg); m != nil {
		addFact("user:name", m[1])
	}
	if m := reIAm.FindStringSubmatch(msg); m != nil {
		candidate := m[1]
		if looksLikeProperName(candidate) {
			addFact("user:name", candidate)
		}
	}
	// "call me X" → nickname first (already handled), name fallback
	if m := reCallMe.FindStringSubmatch(msg); m != nil {
		if !seen["user:nickname"] {
			addFact("user:nickname", m[1])
		}
		if !seen["user:name"] {
			addFact("user:name", titleCase(m[1]))
		}
	}

	// --- user:timezone ---
	if m := reMyTimezoneIs.FindStringSubmatch(msg); m != nil {
		addFact("user:timezone", m[1])
	}
	if m := reInTimezone.FindStringSubmatch(msg); m != nil {
		addFact("user:timezone", m[1])
	}

	// --- user:locale ---
	if m := reIAmFrom.FindStringSubmatch(msg); m != nil {
		addFact("user:locale", m[1])
	}
	if m := reMyLocaleIs.FindStringSubmatch(msg); m != nil {
		addFact("user:locale", m[1])
	}

	// --- pref:language ---
	if m := reReplyIn.FindStringSubmatch(msg); m != nil {
		addFact("pref:language", m[1])
	}
	if m := reSpeakIn.FindStringSubmatch(msg); m != nil {
		addFact("pref:language", m[1])
	}
	if m := reUseLang.FindStringSubmatch(msg); m != nil {
		addFact("pref:language", m[1])
	}
	if m := reRespondIn.FindStringSubmatch(msg); m != nil {
		addFact("pref:language", m[1])
	}

	// --- pref:style ---
	if m := reUseStyle.FindStringSubmatch(msg); m != nil {
		addFact("pref:style", m[1])
	}
	if m := reWriteStyle.FindStringSubmatch(msg); m != nil {
		addFact("pref:style", m[1])
	}

	// --- pref:verbosity ---
	if m := reBeVerbosity.FindStringSubmatch(msg); m != nil {
		addFact("pref:verbosity", m[1])
	}
	if m := reKeepVerbosity.FindStringSubmatch(msg); m != nil {
		addFact("pref:verbosity", m[1])
	}

	// --- pref:technical_depth ---
	if m := reExplainAtLevel.FindStringSubmatch(msg); m != nil {
		addFact("pref:technical_depth", m[1])
	}
	if m := reLevelExplain.FindStringSubmatch(msg); m != nil {
		addFact("pref:technical_depth", m[1])
	}

	return facts
}

// ApplyExplicitLearning persists only new/changed facts. Max 1 write per key per call.
// Silently no-ops if coreStore or prefMgr is nil.
func ApplyExplicitLearning(ctx context.Context, userID, msg string, coreStore CoreMemoryWriter, prefMgr PrefWriter) error {
	if coreStore == nil && prefMgr == nil {
		return nil
	}

	facts := ExtractExplicitUserKnowledge(msg)
	if len(facts) == 0 {
		return nil
	}

	var firstErr error
	for _, f := range facts {
		if !allowedExplicitKeys[f.Key] {
			continue
		}

		if strings.HasPrefix(f.Key, "user:") {
			if coreStore == nil {
				continue
			}
			// Read-before-write: only persist if value differs.
			existing, err := coreStore.Get(ctx, userID, f.Key)
			if err != nil {
				slog.Warn("explicit learning: read failed", "key", f.Key, "err", err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if existing == f.Value {
				continue // No change needed.
			}
			if err := coreStore.Set(ctx, userID, f.Key, f.Value); err != nil {
				slog.Warn("explicit learning: write failed", "key", f.Key, "err", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		} else if strings.HasPrefix(f.Key, "pref:") {
			if prefMgr == nil {
				continue
			}
			if err := prefMgr.UpdatePreference(ctx, userID, f.Key, f.Value); err != nil {
				slog.Warn("explicit learning: pref update failed", "key", f.Key, "err", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	return firstErr
}

// looksLikeProperName returns true if the candidate looks like a proper noun
// (capitalized first word, 1-3 words, not a common state/adjective/verb).
func looksLikeProperName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	words := strings.Fields(s)
	if len(words) == 0 || len(words) > 3 {
		return false
	}

	// First word must start with uppercase.
	firstRune := rune(words[0][0])
	if !unicode.IsUpper(firstRune) {
		return false
	}

	// Check against non-name words (lowercase comparison).
	if nonNameWords[strings.ToLower(words[0])] {
		return false
	}

	return true
}

// titleCase converts a string to title case (first letter of each word uppercase).
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}
