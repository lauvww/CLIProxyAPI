package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestSessionAffinitySelectorPick_BindsSameSession(t *testing.T) {
	t.Parallel()

	selector := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{
		Fallback: &RoundRobinSelector{},
		TTL:      time.Minute,
	})
	t.Cleanup(selector.Stop)

	auths := []*Auth{{ID: "b"}, {ID: "a"}}
	headers := make(http.Header)
	headers.Set("X-Session-ID", "session-a")
	opts := cliproxyexecutor.Options{
		Headers: headers,
	}

	first, errFirst := selector.Pick(context.Background(), "gemini", "gemini-2.5-pro", opts, auths)
	if errFirst != nil {
		t.Fatalf("first Pick() error = %v", errFirst)
	}
	second, errSecond := selector.Pick(context.Background(), "gemini", "gemini-2.5-pro", opts, auths)
	if errSecond != nil {
		t.Fatalf("second Pick() error = %v", errSecond)
	}
	if first == nil || second == nil {
		t.Fatalf("session-affinity returned nil auth")
	}
	if first.ID != "a" {
		t.Fatalf("first Pick() auth.ID = %q, want %q", first.ID, "a")
	}
	if second.ID != first.ID {
		t.Fatalf("second Pick() auth.ID = %q, want same session binding %q", second.ID, first.ID)
	}

	otherHeaders := make(http.Header)
	otherHeaders.Set("X-Session-ID", "session-b")
	other, errOther := selector.Pick(context.Background(), "gemini", "gemini-2.5-pro", cliproxyexecutor.Options{
		Headers: otherHeaders,
	}, auths)
	if errOther != nil {
		t.Fatalf("other-session Pick() error = %v", errOther)
	}
	if other == nil {
		t.Fatalf("other-session Pick() auth = nil")
	}
	if other.ID != "b" {
		t.Fatalf("other-session Pick() auth.ID = %q, want %q", other.ID, "b")
	}
}

func TestSessionAffinitySelectorPick_RebindsWhenCachedAuthUnavailable(t *testing.T) {
	t.Parallel()

	selector := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{
		Fallback: &RoundRobinSelector{},
		TTL:      time.Minute,
	})
	t.Cleanup(selector.Stop)

	headers := make(http.Header)
	headers.Set("X-Session-ID", "session-rebind")
	opts := cliproxyexecutor.Options{
		Headers: headers,
	}
	initialAuths := []*Auth{{ID: "b"}, {ID: "a"}}

	first, errFirst := selector.Pick(context.Background(), "claude", "claude-sonnet", opts, initialAuths)
	if errFirst != nil {
		t.Fatalf("first Pick() error = %v", errFirst)
	}
	if first == nil || first.ID != "a" {
		t.Fatalf("first Pick() auth.ID = %v, want %q", authID(first), "a")
	}

	reboundAuths := []*Auth{
		{
			ID:       "a",
			Disabled: true,
		},
		{ID: "b"},
	}
	second, errSecond := selector.Pick(context.Background(), "claude", "claude-sonnet", opts, reboundAuths)
	if errSecond != nil {
		t.Fatalf("second Pick() error = %v", errSecond)
	}
	if second == nil {
		t.Fatalf("second Pick() auth = nil")
	}
	if second.ID != "b" {
		t.Fatalf("second Pick() auth.ID = %q, want rebound auth %q", second.ID, "b")
	}
}

func TestSessionAffinitySelectorPick_TTLExpiryAllowsRebind(t *testing.T) {
	t.Parallel()

	selector := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{
		Fallback: &RoundRobinSelector{},
		TTL:      25 * time.Millisecond,
	})
	t.Cleanup(selector.Stop)

	auths := []*Auth{{ID: "b"}, {ID: "a"}}
	headers := make(http.Header)
	headers.Set("X-Session-ID", "session-ttl")
	opts := cliproxyexecutor.Options{
		Headers: headers,
	}

	first, errFirst := selector.Pick(context.Background(), "codex", "gpt-5", opts, auths)
	if errFirst != nil {
		t.Fatalf("first Pick() error = %v", errFirst)
	}
	if first == nil || first.ID != "a" {
		t.Fatalf("first Pick() auth.ID = %v, want %q", authID(first), "a")
	}

	time.Sleep(40 * time.Millisecond)

	second, errSecond := selector.Pick(context.Background(), "codex", "gpt-5", opts, auths)
	if errSecond != nil {
		t.Fatalf("second Pick() error = %v", errSecond)
	}
	if second == nil {
		t.Fatalf("second Pick() auth = nil")
	}
	if second.ID != "b" {
		t.Fatalf("second Pick() auth.ID = %q, want %q after TTL expiry", second.ID, "b")
	}
}

func TestExtractSessionIDPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers http.Header
		payload []byte
		wantID  string
		wantAlt string
	}{
		{
			name: "claude metadata session has highest priority",
			headers: func() http.Header {
				h := make(http.Header)
				h.Set("X-Session-ID", "header-session")
				return h
			}(),
			payload: []byte(`{"metadata":{"user_id":"user_123_account__session_aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},"conversation_id":"conv-1"}`),
			wantID:  "claude:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
		{
			name: "header session used before generic metadata user id",
			headers: func() http.Header {
				h := make(http.Header)
				h.Set("X-Session-ID", "header-session")
				return h
			}(),
			payload: []byte(`{"metadata":{"user_id":"plain-user"},"conversation_id":"conv-1"}`),
			wantID:  "header:header-session",
		},
		{
			name:    "conversation id fallback",
			payload: []byte(`{"conversation_id":"conv-42"}`),
			wantID:  "conv:conv-42",
		},
		{
			name:    "message hash fallback",
			payload: []byte(`{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]}`),
			wantID:  "msg:708e3b2be6fac997",
		},
		{
			name:    "message hash with assistant yields primary and fallback",
			payload: []byte(`{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"},{"role":"assistant","content":"world"}]}`),
			wantID:  "msg:4b1a144af0802483",
			wantAlt: "msg:708e3b2be6fac997",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotAlt := extractSessionIDs(tt.headers, tt.payload, nil)
			if gotID != tt.wantID {
				t.Fatalf("primary session ID = %q, want %q", gotID, tt.wantID)
			}
			if gotAlt != tt.wantAlt {
				t.Fatalf("fallback session ID = %q, want %q", gotAlt, tt.wantAlt)
			}
		})
	}
}

func authID(auth *Auth) string {
	if auth == nil {
		return "<nil>"
	}
	return auth.ID
}
