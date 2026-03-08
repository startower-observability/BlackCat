package agentapi

import (
	"fmt"
	"sync"
	"testing"
)

func TestRuntimeModelHolderInitialState(t *testing.T) {
	h := NewRuntimeModelHolder()
	got := h.Get()
	if got != (RuntimeModelStatus{}) {
		t.Fatalf("expected zero-value status, got %+v", got)
	}
}

func TestRuntimeModelHolderConcurrentAccess(t *testing.T) {
	h := NewRuntimeModelHolder()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status := RuntimeModelStatus{
				ConfiguredModel: RuntimeModelRef{CanonicalID: fmt.Sprintf("openai/gpt-4.%d", i)},
				AppliedModel:    RuntimeModelRef{CanonicalID: fmt.Sprintf("openai/gpt-4.%d", i)},
				BackendProvider: "openai",
				ReloadCount:     i,
			}
			h.Set(status)
			_ = h.Get()
		}(i)
	}
	wg.Wait()

	got := h.Get()
	if got.BackendProvider == "" {
		t.Fatal("expected backend provider to be set after concurrent writes")
	}
}
