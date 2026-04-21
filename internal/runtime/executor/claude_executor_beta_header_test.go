package executor

import (
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestApplyClaudeHeaders_DoesNotInjectObsoleteClaude1MBeta(t *testing.T) {
	req := newClaudeHeaderTestRequest(t, http.Header{
		"Anthropic-Beta":  []string{"structured-outputs-2025-12-15"},
		"X-CPA-CLAUDE-1M": []string{"1"},
	})
	auth := &cliproxyauth.Auth{
		ID: "auth-legacy-1m-header",
		Attributes: map[string]string{
			"api_key": "key-legacy-1m-header",
		},
	}

	applyClaudeHeaders(req, auth, "key-legacy-1m-header", false, nil, &config.Config{})

	got := req.Header.Get("Anthropic-Beta")
	if strings.Contains(got, "context-1m-2025-08-07") {
		t.Fatalf("Anthropic-Beta = %q, obsolete 1M beta should not be injected", got)
	}
	if !strings.Contains(got, "structured-outputs-2025-12-15") {
		t.Fatalf("Anthropic-Beta = %q, want existing custom beta to be preserved", got)
	}
	if !strings.Contains(got, "oauth-2025-04-20") {
		t.Fatalf("Anthropic-Beta = %q, want oauth beta to remain enabled", got)
	}
	if !strings.Contains(got, "interleaved-thinking-2025-05-14") {
		t.Fatalf("Anthropic-Beta = %q, want interleaved-thinking beta to remain enabled", got)
	}
}
