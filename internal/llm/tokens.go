package llm

import (
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
	openai "github.com/sashabaranov/go-openai"
)

// encodingCache caches tiktoken encoding instances per model to avoid
// expensive re-creation on every call.
var (
	encodingCache sync.Map // map[string]*tiktoken.Tiktoken
)

// getEncoding returns a cached tiktoken encoding for the given model.
// Falls back to cl100k_base if the model is not recognized.
func getEncoding(model string) *tiktoken.Tiktoken {
	if enc, ok := encodingCache.Load(model); ok {
		return enc.(*tiktoken.Tiktoken)
	}

	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fallback to cl100k_base for unknown models.
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			// cl100k_base should always be available; panic is a programming error.
			panic("tiktoken: failed to load cl100k_base encoding: " + err.Error())
		}
	}

	actual, _ := encodingCache.LoadOrStore(model, enc)
	return actual.(*tiktoken.Tiktoken)
}

// CountTokens returns the exact BPE token count for the given text using
// the tokenizer associated with the specified model.
func CountTokens(model, text string) int {
	enc := getEncoding(model)
	return len(enc.Encode(text, nil, nil))
}

// CountMessages returns the total token count for a slice of chat messages,
// following the OpenAI token-counting conventions.
//
// Reference: https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
func CountMessages(model string, msgs []openai.ChatCompletionMessage) int {
	enc := getEncoding(model)

	// Per-message overhead depends on the model.
	tokensPerMessage := 3 // <|start|>{role/name}\n{content}<|end|>\n
	tokensPerName := 1
	if model == "gpt-3.5-turbo-0301" {
		tokensPerMessage = 4
		tokensPerName = -1 // role is omitted when name is present
	}

	numTokens := 0
	for _, msg := range msgs {
		numTokens += tokensPerMessage
		numTokens += len(enc.Encode(msg.Content, nil, nil))
		numTokens += len(enc.Encode(msg.Role, nil, nil))
		if msg.Name != "" {
			numTokens += len(enc.Encode(msg.Name, nil, nil))
			numTokens += tokensPerName
		}
	}
	numTokens += 3 // every reply is primed with <|start|>assistant<|message|>

	return numTokens
}

// EstimateTokens is a backward-compatible alias that delegates to CountTokens
// using the gpt-4o tokenizer.
//
// Deprecated: Use CountTokens(model, text) for accurate per-model counting.
func EstimateTokens(s string) int {
	return CountTokens("gpt-4o", s)
}
