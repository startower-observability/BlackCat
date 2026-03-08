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

func TestRuntimeModelHolderUpdateApplied(t *testing.T) {
	h := NewRuntimeModelHolder()
	initial := RuntimeModelStatus{
		ConfiguredModel: RuntimeModelRef{CanonicalID: "openai/gpt-4.1", Vendor: "openai"},
		AppliedModel:    RuntimeModelRef{CanonicalID: "openai/gpt-4.1", Vendor: "openai"},
		BackendProvider: "openai",
		LastReloadError: "previous error",
		ReloadCount:     7,
	}
	h.Set(initial)

	newApplied := RuntimeModelRef{CanonicalID: "openai-codex/gpt-5.3-codex", Vendor: "openai-codex"}
	h.UpdateApplied(newApplied, "copilot")

	got := h.Get()
	if got.AppliedModel != newApplied {
		t.Fatalf("AppliedModel = %+v; want %+v", got.AppliedModel, newApplied)
	}
	if got.BackendProvider != "copilot" {
		t.Fatalf("BackendProvider = %q; want %q", got.BackendProvider, "copilot")
	}

	if got.ConfiguredModel != initial.ConfiguredModel {
		t.Fatalf("ConfiguredModel changed: got %+v; want %+v", got.ConfiguredModel, initial.ConfiguredModel)
	}
	if got.LastReloadError != initial.LastReloadError {
		t.Fatalf("LastReloadError changed: got %q; want %q", got.LastReloadError, initial.LastReloadError)
	}
	if got.ReloadCount != initial.ReloadCount {
		t.Fatalf("ReloadCount changed: got %d; want %d", got.ReloadCount, initial.ReloadCount)
	}
}

func TestRuntimeModelHolderSetReloadError(t *testing.T) {
	h := NewRuntimeModelHolder()

	h.SetReloadError("reload failed: invalid config")
	got := h.Get()

	if got.LastReloadError != "reload failed: invalid config" {
		t.Fatalf("LastReloadError = %q; want %q", got.LastReloadError, "reload failed: invalid config")
	}
}

func TestRuntimeModelHolderHighConcurrency(t *testing.T) {
	h := NewRuntimeModelHolder()

	const workers = 200
	const iterations = 50

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				applied := RuntimeModelRef{CanonicalID: fmt.Sprintf("openai/gpt-4.%d", (worker+j)%10)}
				h.UpdateApplied(applied, "openai")
				h.SetReloadError(fmt.Sprintf("err-%d", j))
				_ = h.Get()
			}
		}(i)
	}
	wg.Wait()

	got := h.Get()
	if got.BackendProvider == "" {
		t.Fatal("expected backend provider to be set after high-concurrency updates")
	}
}
