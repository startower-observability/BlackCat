package llm

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestCountTokens(t *testing.T) {
	tests := []struct {
		name  string
		model string
		text  string
		want  int
	}{
		{
			name:  "hello_world_gpt4",
			model: "gpt-4",
			text:  "hello world",
			want:  2,
		},
		{
			name:  "empty_string",
			model: "gpt-4",
			text:  "",
			want:  0,
		},
		{
			name:  "gpt4o_hello_world",
			model: "gpt-4o",
			text:  "hello world",
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountTokens(tt.model, tt.text)
			if got != tt.want {
				t.Errorf("CountTokens(%q, %q) = %d, want %d", tt.model, tt.text, got, tt.want)
			}
		})
	}
}

func TestCountTokens_UnknownModel(t *testing.T) {
	// Should not panic; falls back to cl100k_base.
	got := CountTokens("unknown-model-xyz", "test")
	if got <= 0 {
		t.Errorf("CountTokens with unknown model returned %d, expected > 0", got)
	}
}

func TestCountTokens_EncodingReused(t *testing.T) {
	// Call twice — second call should reuse the cached encoding without error.
	model := "gpt-4"
	first := CountTokens(model, "hello")
	second := CountTokens(model, "hello")
	if first != second {
		t.Errorf("encoding reuse: first=%d, second=%d — expected identical", first, second)
	}
}

func TestCountMessages(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello!"},
	}

	tokens := CountMessages("gpt-4", msgs)
	if tokens <= 0 {
		t.Errorf("CountMessages returned %d, expected > 0", tokens)
	}

	// Known token count for this exact conversation with gpt-4 (cl100k_base):
	// system msg: 3 (overhead) + 4 (role "system") + 5 (content) = ~12
	// user msg: 3 (overhead) + 1 (role "user") + 2 (content "Hello!") = ~6
	// reply priming: 3
	// Total should be around 21
	// We just verify it's reasonable (between 15 and 30).
	if tokens < 15 || tokens > 30 {
		t.Errorf("CountMessages returned %d, expected between 15 and 30", tokens)
	}
}

func TestCountMessages_EmptySlice(t *testing.T) {
	tokens := CountMessages("gpt-4", nil)
	// Empty messages: just the reply priming (3 tokens).
	if tokens != 3 {
		t.Errorf("CountMessages(nil) = %d, want 3", tokens)
	}
}

func TestEstimateTokens_BackwardCompat(t *testing.T) {
	// EstimateTokens should delegate to CountTokens("gpt-4o", s).
	got := EstimateTokens("hello world")
	want := CountTokens("gpt-4o", "hello world")
	if got != want {
		t.Errorf("EstimateTokens(\"hello world\") = %d, want %d (from CountTokens)", got, want)
	}
}
