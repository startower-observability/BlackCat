package version

import (
	"strings"
	"testing"
)

func TestInfoDefaults(t *testing.T) {
	// Reset to defaults for testing
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
	}()

	Version = "dev"
	Commit = "unknown"
	BuildDate = ""

	info := Info()
	if !strings.Contains(info, "dev") {
		t.Errorf("Info() should contain version 'dev', got: %s", info)
	}
	if !strings.Contains(info, "unknown") {
		t.Errorf("Info() should contain commit 'unknown', got: %s", info)
	}
	if !strings.Contains(info, "built") {
		t.Errorf("Info() should contain 'built', got: %s", info)
	}
}

func TestInfoWithLdflags(t *testing.T) {
	// Simulate ldflags override
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
	}()

	Version = "v1.2.3"
	Commit = "abc123def456"
	BuildDate = "2026-03-08T10:00:00Z"

	info := Info()
	if !strings.Contains(info, "v1.2.3") {
		t.Errorf("Info() should contain version 'v1.2.3', got: %s", info)
	}
	if !strings.Contains(info, "abc123def456") {
		t.Errorf("Info() should contain commit 'abc123def456', got: %s", info)
	}
	if !strings.Contains(info, "2026-03-08T10:00:00Z") {
		t.Errorf("Info() should contain build date '2026-03-08T10:00:00Z', got: %s", info)
	}
}

func TestInfoFormat(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
	}()

	Version = "test-v2"
	Commit = "testsha"
	BuildDate = "2026-03-08T00:00:00Z"

	info := Info()
	expected := "test-v2 (testsha) built 2026-03-08T00:00:00Z"
	if info != expected {
		t.Errorf("Info() format mismatch.\nExpected: %s\nGot:      %s", expected, info)
	}
}
