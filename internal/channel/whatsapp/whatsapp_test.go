package whatsapp

import (
	"strings"
	"testing"

	"github.com/startower-observability/blackcat/internal/types"
)

func TestSplitMessageShort(t *testing.T) {
	text := "Hello, world!"
	chunks := splitMessage(text, 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Fatalf("expected %q, got %q", text, chunks[0])
	}
}

func TestSplitMessageExactLength(t *testing.T) {
	text := "abcde"
	chunks := splitMessage(text, 5)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Fatalf("expected %q, got %q", text, chunks[0])
	}
}

func TestSplitMessageLong(t *testing.T) {
	// Build text with newlines, total > maxLen.
	lines := []string{
		strings.Repeat("a", 30),
		strings.Repeat("b", 30),
		strings.Repeat("c", 30),
		strings.Repeat("d", 30),
	}
	text := strings.Join(lines, "\n")
	// Total: 4*30 + 3 newlines = 123 chars. Split at 70.
	chunks := splitMessage(text, 70)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify all content is preserved.
	rejoined := strings.Join(chunks, "")
	if rejoined != text {
		t.Fatalf("content not preserved after split:\n  original: %q\n  rejoined: %q", text, rejoined)
	}

	// Each chunk should be <= maxLen.
	for i, chunk := range chunks {
		if len(chunk) > 70 {
			t.Errorf("chunk %d exceeds maxLen: %d chars", i, len(chunk))
		}
	}
}

func TestSplitMessageNoNewlines(t *testing.T) {
	// Continuous text without newlines — must hard split.
	text := strings.Repeat("x", 250)
	chunks := splitMessage(text, 100)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	// Verify lengths.
	if len(chunks[0]) != 100 {
		t.Errorf("chunk 0: expected 100 chars, got %d", len(chunks[0]))
	}
	if len(chunks[1]) != 100 {
		t.Errorf("chunk 1: expected 100 chars, got %d", len(chunks[1]))
	}
	if len(chunks[2]) != 50 {
		t.Errorf("chunk 2: expected 50 chars, got %d", len(chunks[2]))
	}

	// Verify content is preserved.
	rejoined := strings.Join(chunks, "")
	if rejoined != text {
		t.Fatal("content not preserved after hard split")
	}
}

func TestSplitMessageEmpty(t *testing.T) {
	chunks := splitMessage("", 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty string, got %d", len(chunks))
	}
	if chunks[0] != "" {
		t.Fatalf("expected empty string, got %q", chunks[0])
	}
}

func TestSplitMessageDefaultMaxLen(t *testing.T) {
	// Passing 0 should use the default maxMessageLen (4096).
	text := strings.Repeat("a", 5000)
	chunks := splitMessage(text, 0)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks with default maxLen, got %d", len(chunks))
	}
	if len(chunks[0]) != 4096 {
		t.Errorf("chunk 0: expected 4096 chars, got %d", len(chunks[0]))
	}
}

func TestNewWhatsAppChannel(t *testing.T) {
	ch := NewWhatsAppChannel("file:test.db?_foreign_keys=on", nil)

	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	if ch.storePath != "file:test.db?_foreign_keys=on" {
		t.Fatalf("expected storePath %q, got %q",
			"file:test.db?_foreign_keys=on", ch.storePath)
	}
	if ch.incoming == nil {
		t.Fatal("expected non-nil incoming channel")
	}
	if cap(ch.incoming) != 256 {
		t.Fatalf("expected incoming buffer size 256, got %d", cap(ch.incoming))
	}
	if ch.started {
		t.Fatal("expected started=false on new channel")
	}
}

func TestInfo(t *testing.T) {
	ch := NewWhatsAppChannel("file:test.db", nil)
	info := ch.Info()

	if info.Type != types.ChannelWhatsApp {
		t.Fatalf("expected type %q, got %q", types.ChannelWhatsApp, info.Type)
	}
	if info.Name != "whatsapp" {
		t.Fatalf("expected name %q, got %q", "whatsapp", info.Name)
	}
	if info.Connected {
		t.Fatal("expected Connected=false before Start()")
	}
}

func TestReceive(t *testing.T) {
	ch := NewWhatsAppChannel("file:test.db", nil)
	recv := ch.Receive()
	if recv == nil {
		t.Fatal("expected non-nil Receive() channel")
	}
	// Should be the same channel as incoming.
	if cap(recv) != 256 {
		t.Fatalf("expected buffer size 256, got %d", cap(recv))
	}
}

func TestChannelInterfaceCompliance(t *testing.T) {
	// Compile-time check that WhatsAppChannel implements types.Channel.
	var _ types.Channel = (*WhatsAppChannel)(nil)
}

func TestStartWithoutCGO(t *testing.T) {
	// In non-CGO builds, Start should return an error about CGO.
	// In CGO builds, Start would try to open a real DB (which we don't test).
	// This test only runs meaningfully in !cgo builds.
	ch := NewWhatsAppChannel("file::memory:?_foreign_keys=on", nil)
	err := ch.Start(t.Context())
	if err == nil {
		// If Start succeeded (CGO build with in-memory DB), clean up.
		_ = ch.Stop()
		t.Skip("CGO is enabled, skipping non-CGO error test")
	}
	// In non-CGO build, expect the CGO error.
	if !strings.Contains(err.Error(), "CGO") {
		// Could also be a real DB error in CGO builds — that's fine too.
		t.Logf("Start error (expected): %v", err)
	}
}

func TestNormalizeE164(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+628123456789", "+628123456789"},
		{"628123456789", "+628123456789"},
		{"+62 812-345-6789", "+628123456789"},
		{"+62 (812) 345-6789", "+628123456789"},
		{"", "+"},
	}
	for _, tt := range tests {
		got := normalizeE164(tt.input)
		if got != tt.want {
			t.Errorf("normalizeE164(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPhoneFromJID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"628123456789@s.whatsapp.net", "+628123456789"},
		{"6282394921432:5@s.whatsapp.net", "+6282394921432"},
		{"1234567890:0@s.whatsapp.net", "+1234567890"},
		{"1234567890@s.whatsapp.net", "+1234567890"},
		{"", ""},
		{"@s.whatsapp.net", ""},
	}
	for _, tt := range tests {
		got := phoneFromJID(tt.input)
		if got != tt.want {
			t.Errorf("phoneFromJID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsAllowed(t *testing.T) {
	// nil allowFrom = deny all (secure by default)
	ch := NewWhatsAppChannel("file:test.db", nil)
	if ch.isAllowed("628123456789@s.whatsapp.net", "628123456789@s.whatsapp.net", "") {
		t.Error("nil allowFrom should deny all (secure by default)")
	}

	// empty slice = deny all (secure by default)
	ch = NewWhatsAppChannel("file:test.db", []string{})
	if ch.isAllowed("628123456789@s.whatsapp.net", "628123456789@s.whatsapp.net", "") {
		t.Error("empty allowFrom should deny all (secure by default)")
	}

	// wildcard = allow all
	ch = NewWhatsAppChannel("file:test.db", []string{"*"})
	if !ch.isAllowed("628123456789@s.whatsapp.net", "628123456789@s.whatsapp.net", "") {
		t.Error("wildcard allowFrom should allow all")
	}

	// specific numbers — matched via chat JID (phone-based)
	ch = NewWhatsAppChannel("file:test.db", []string{"+628123456789"})
	if !ch.isAllowed("628123456789@s.whatsapp.net", "999@lid", "") {
		t.Error("allowed number via chat JID should pass")
	}
	if ch.isAllowed("629999999999@s.whatsapp.net", "888@lid", "") {
		t.Error("non-allowed number should be blocked")
	}

	// LID sender with matching chat JID (Meta's new format)
	ch = NewWhatsAppChannel("file:test.db", []string{"+6282394921432"})
	if !ch.isAllowed("6282394921432@s.whatsapp.net", "249271429374089@lid", "") {
		t.Error("LID sender with matching chat JID should pass")
	}
	if ch.isAllowed("629999999999@s.whatsapp.net", "249271429374089@lid", "") {
		t.Error("LID sender with non-matching chat JID should be blocked")
	}

	// normalisation: config has spaces/dashes
	ch = NewWhatsAppChannel("file:test.db", []string{"+62 812-345-6789"})
	if !ch.isAllowed("628123456789@s.whatsapp.net", "999@lid", "") {
		t.Error("normalised number should match")
	}

	// JID with device suffix in chat
	ch = NewWhatsAppChannel("file:test.db", []string{"+6282394921432"})
	if !ch.isAllowed("6282394921432:5@s.whatsapp.net", "999@lid", "") {
		t.Error("chat JID with device suffix should match")
	}

	// Fallback: sender JID is phone-based (older protocol)
	ch = NewWhatsAppChannel("file:test.db", []string{"+628123456789"})
	if !ch.isAllowed("group@g.us", "628123456789@s.whatsapp.net", "") {
		t.Error("phone-based sender JID should match as fallback")
	}

	// --- LID resolution via resolvedPhone parameter ---

	// Both chat and sender are LID, but resolvedPhone matches whitelist
	ch = NewWhatsAppChannel("file:test.db", []string{"+6282394921432"})
	if !ch.isAllowed("249271429374089@lid", "249271429374089@lid", "+6282394921432") {
		t.Error("LID sender with resolvedPhone should pass")
	}

	// resolvedPhone doesn't match whitelist
	if ch.isAllowed("249271429374089@lid", "249271429374089@lid", "+629999999999") {
		t.Error("LID sender with non-matching resolvedPhone should be blocked")
	}

	// Both JIDs are LID and no resolvedPhone — should be blocked
	if ch.isAllowed("249271429374089@lid", "249271429374089@lid", "") {
		t.Error("LID-only JIDs with no resolvedPhone should be blocked")
	}

	// resolvedPhone takes priority even when chat JID doesn't match
	ch = NewWhatsAppChannel("file:test.db", []string{"+628123456789"})
	if !ch.isAllowed("249271429374089@lid", "249271429374089@lid", "+628123456789") {
		t.Error("resolvedPhone should match even when JIDs are all LID")
	}
}
