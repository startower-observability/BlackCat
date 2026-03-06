//go:build cgo

package whatsapp

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	codeBlockRe      = regexp.MustCompile("(?s)```.*?```")
	boldDoubleStarRe = regexp.MustCompile(`\*\*(.+?)\*\*`)
	boldDoubleUndRe  = regexp.MustCompile(`__(.+?)__`)
	italicSingleRe   = regexp.MustCompile(`\*([^*\n]+?)\*`)
	italicFullLineRe = regexp.MustCompile(`^\*([^*\n]+?)\*$`)
	strikeRe         = regexp.MustCompile(`~~(.+?)~~`)
	headingRe        = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	horizontalRuleRe = regexp.MustCompile(`(?m)^\s*---+\s*$\r?\n?`)
	bulletListRe     = regexp.MustCompile(`(?m)^(\s*)[-*+]\s+`)
	numberedListRe   = regexp.MustCompile(`(?m)^(\s*)\d+\.\s+`)
)

// FormatForWhatsApp converts standard Markdown to WhatsApp-compatible formatting.
// Conversion rules applied in strict order to avoid collisions:
//  1. Triple-backtick code blocks: left as-is (WhatsApp renders them)
//  2. **bold** or __bold__ -> *bold*
//  3. *italic* (single star, not part of **) -> _italic_
//  4. ~~strikethrough~~ -> ~strike~
//  5. # Heading lines -> *Heading* (bold)
//  6. --- horizontal rules -> removed
func FormatForWhatsApp(text string) string {
	if text == "" {
		return text
	}
	original := text

	codeBlocks := make([]string, 0)
	text = codeBlockRe.ReplaceAllStringFunc(text, func(m string) string {
		idx := len(codeBlocks)
		codeBlocks = append(codeBlocks, m)
		return fmt.Sprintf("@@WA_CODE_BLOCK_%d@@", idx)
	})

	boldPlaceholders := make([]string, 0)
	replaceBold := func(content string) string {
		idx := len(boldPlaceholders)
		boldPlaceholders = append(boldPlaceholders, "*"+content+"*")
		return fmt.Sprintf("@@WA_BOLD_%d@@", idx)
	}

	text = boldDoubleStarRe.ReplaceAllStringFunc(text, func(m string) string {
		parts := boldDoubleStarRe.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return replaceBold(parts[1])
	})
	text = boldDoubleUndRe.ReplaceAllStringFunc(text, func(m string) string {
		parts := boldDoubleUndRe.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return replaceBold(parts[1])
	})

	hasNonItalicMarkdown :=
		boldDoubleStarRe.MatchString(original) ||
			boldDoubleUndRe.MatchString(original) ||
			strikeRe.MatchString(original) ||
			headingRe.MatchString(original) ||
			horizontalRuleRe.MatchString(original)

	if hasNonItalicMarkdown {
		text = italicSingleRe.ReplaceAllString(text, `_$1_`)
	} else if italicFullLineRe.MatchString(strings.TrimSpace(text)) {
		text = italicSingleRe.ReplaceAllString(text, `_$1_`)
	}

	text = strikeRe.ReplaceAllString(text, `~$1~`)
	text = headingRe.ReplaceAllString(text, `*$1*`)
	text = horizontalRuleRe.ReplaceAllString(text, "")
	text = bulletListRe.ReplaceAllString(text, "${1}• ")
	text = numberedListRe.ReplaceAllString(text, "${1}")

	for i, value := range boldPlaceholders {
		placeholder := fmt.Sprintf("@@WA_BOLD_%d@@", i)
		text = strings.ReplaceAll(text, placeholder, value)
	}
	for i, value := range codeBlocks {
		placeholder := fmt.Sprintf("@@WA_CODE_BLOCK_%d@@", i)
		text = strings.ReplaceAll(text, placeholder, value)
	}

	return text
}
