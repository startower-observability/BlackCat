//go:build nospa

package agent

import (
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
)

func TestRoleRegistryFromConfig(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: "wizard", Priority: 30, Keywords: []string{"code", "debug", "test"}},
		{Name: "phantom", Priority: 10, Keywords: []string{"deploy", "docker"}},
		{Name: "oracle", Priority: 100, Keywords: nil},
	}

	reg := NewRoleRegistry(roles)
	views := reg.Views()

	if len(views) != 3 {
		t.Fatalf("expected 3 views, got %d", len(views))
	}

	// Assert sorted ascending by priority
	if views[0].Name != "phantom" || views[0].Priority != 10 {
		t.Errorf("expected first role phantom(10), got %s(%d)", views[0].Name, views[0].Priority)
	}
	if views[1].Name != "wizard" || views[1].Priority != 30 {
		t.Errorf("expected second role wizard(30), got %s(%d)", views[1].Name, views[1].Priority)
	}
	if views[2].Name != "oracle" || views[2].Priority != 100 {
		t.Errorf("expected third role oracle(100), got %s(%d)", views[2].Name, views[2].Priority)
	}

	// Assert KeywordCount matches
	if views[0].KeywordCount != 2 {
		t.Errorf("phantom keyword count: expected 2, got %d", views[0].KeywordCount)
	}
	if views[1].KeywordCount != 3 {
		t.Errorf("wizard keyword count: expected 3, got %d", views[1].KeywordCount)
	}
	if views[2].KeywordCount != 0 {
		t.Errorf("oracle keyword count: expected 0, got %d", views[2].KeywordCount)
	}
}

func TestRoleRegistryFallbackRole(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: "wizard", Priority: 30, Keywords: []string{"code"}},
		{Name: "oracle", Priority: 100, Keywords: nil},
	}

	reg := NewRoleRegistry(roles)
	fb := reg.FallbackRole()

	if fb == nil {
		t.Fatal("expected fallback role, got nil")
	}
	if fb.Name != "oracle" {
		t.Errorf("expected fallback name oracle, got %s", fb.Name)
	}
	if fb.Priority != 100 {
		t.Errorf("expected fallback priority 100, got %d", fb.Priority)
	}
	if fb.KeywordCount != 0 {
		t.Errorf("expected fallback keyword count 0, got %d", fb.KeywordCount)
	}
}

func TestRoleRegistryEmptyConfig(t *testing.T) {
	reg := NewRoleRegistry(nil)

	views := reg.Views()
	if len(views) != 0 {
		t.Errorf("expected 0 views for nil config, got %d", len(views))
	}

	fb := reg.FallbackRole()
	if fb != nil {
		t.Errorf("expected nil fallback for nil config, got %+v", fb)
	}
}
