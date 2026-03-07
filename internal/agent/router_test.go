package agent

import (
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
)

// TestClassifyMessage_TableDriven tests keyword-based routing with the default role set.
func TestClassifyMessage_TableDriven(t *testing.T) {
	// Use the same defaults that config.Validate() produces.
	roles := []config.RoleConfig{
		{Name: "phantom", Keywords: []string{"restart", "deploy", "server", "status", "docker", "systemctl", "health", "infra", "devops", "service", "nginx", "ssl"}, Priority: 10},
		{Name: "astrology", Keywords: []string{"crypto", "bitcoin", "btc", "eth", "ethereum", "trading", "token", "defi", "nft", "wallet", "market", "portfolio", "investment", "stock", "forex", "chart", "candlestick", "pump", "whale"}, Priority: 20},
		{Name: "wizard", Keywords: []string{"code", "implement", "function", "bug", "fix", "test", "build", "compile", "git", "deploy", "opencode", "typescript", "golang", "python", "javascript", "refactor", "debug", "api", "endpoint", "database", "sql", "migration"}, Priority: 30},
		{Name: "artist", Keywords: []string{"instagram", "tiktok", "twitter", "linkedin", "facebook", "threads", "post", "caption", "hashtag", "reel", "story", "content", "social", "viral", "engagement", "schedule", "publish"}, Priority: 40},
		{Name: "scribe", Keywords: []string{"write", "draft", "article", "blog", "email", "document", "copy", "copywriting", "proofread", "translate", "summarize", "report", "newsletter", "pitch", "proposal"}, Priority: 50},
		{Name: "explorer", Keywords: []string{"search", "find", "look up", "what is", "explain", "research", "summarize", "web", "browse", "read", "compare", "analyze", "review", "investigate"}, Priority: 60},
		{Name: "oracle", Keywords: nil, Priority: 100},
	}

	tests := []struct {
		name string
		msg  string
		want RoleType
	}{
		{
			name: "fix bug routes to wizard",
			msg:  "fix the bug in auth",
			want: RoleWizard,
		},
		{
			name: "bitcoin routes to astrology",
			msg:  "bitcoin price analysis",
			want: RoleAstrology,
		},
		{
			name: "instagram routes to artist",
			msg:  "post on instagram",
			want: RoleArtist,
		},
		{
			name: "deploy to server routes to phantom not wizard (priority)",
			msg:  "deploy to production server",
			want: RolePhantom,
		},
		{
			name: "blog article routes to scribe",
			msg:  "draft a blog article",
			want: RoleScribe,
		},
		{
			name: "what is kubernetes routes to explorer",
			msg:  "what is kubernetes",
			want: RoleExplorer,
		},
		{
			name: "gibberish routes to oracle fallback",
			msg:  "random gibberish xyz",
			want: RoleOracle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMessage(tt.msg, roles)
			if got != tt.want {
				t.Errorf("ClassifyMessage(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

// TestClassifyMessage_NilRolesBackwardCompat verifies that passing nil roles
// does not panic and returns a valid RoleType from the hardcoded defaults.
func TestClassifyMessage_NilRolesBackwardCompat(t *testing.T) {
	got := ClassifyMessage("hello world", nil)
	if got == "" {
		t.Fatal("ClassifyMessage with nil roles returned empty string, want valid RoleType")
	}
	// With "hello world" and default roles, no keyword matches → oracle fallback
	if got != RoleOracle {
		t.Logf("ClassifyMessage(\"hello world\", nil) = %q (valid role, just noting)", got)
	}
}
