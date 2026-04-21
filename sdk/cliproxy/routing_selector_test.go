package cliproxy

import (
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestResolveRoutingSelectorConfig_UsesLegacyAliasAndParsesTTL(t *testing.T) {
	t.Parallel()

	resolved := resolveRoutingSelectorConfig(&config.Config{
		Routing: internalconfig.RoutingConfig{
			Strategy:                  "fillfirst",
			ClaudeCodeSessionAffinity: true,
			SessionAffinityTTL:        "45m",
			SessionAffinity:           false,
		},
	})

	if resolved.strategy != "fill-first" {
		t.Fatalf("strategy = %q, want %q", resolved.strategy, "fill-first")
	}
	if !resolved.sessionAffinity {
		t.Fatalf("sessionAffinity = false, want true")
	}
	if resolved.sessionTTL != 45*time.Minute {
		t.Fatalf("sessionTTL = %v, want %v", resolved.sessionTTL, 45*time.Minute)
	}
}

func TestResolveRoutingSelectorConfig_InvalidTTLDefaultsToOneHour(t *testing.T) {
	t.Parallel()

	resolved := resolveRoutingSelectorConfig(&config.Config{
		Routing: internalconfig.RoutingConfig{
			SessionAffinity:    true,
			SessionAffinityTTL: "not-a-duration",
		},
	})

	if resolved.sessionTTL != time.Hour {
		t.Fatalf("sessionTTL = %v, want %v", resolved.sessionTTL, time.Hour)
	}
	if resolved.sessionTTLString != "not-a-duration" {
		t.Fatalf("sessionTTLString = %q, want %q", resolved.sessionTTLString, "not-a-duration")
	}
}

func TestBuildRoutingSelector_ReturnsExpectedSelectorTypes(t *testing.T) {
	t.Parallel()

	fillFirstSelector := buildRoutingSelector(&config.Config{
		Routing: internalconfig.RoutingConfig{Strategy: "fill-first"},
	})
	if _, ok := fillFirstSelector.(*coreauth.FillFirstSelector); !ok {
		t.Fatalf("fill-first selector type = %T, want *coreauth.FillFirstSelector", fillFirstSelector)
	}

	sessionSelector := buildRoutingSelector(&config.Config{
		Routing: internalconfig.RoutingConfig{
			Strategy:           "round-robin",
			SessionAffinity:    true,
			SessionAffinityTTL: "30m",
		},
	})
	if _, ok := sessionSelector.(*coreauth.SessionAffinitySelector); !ok {
		t.Fatalf("session selector type = %T, want *coreauth.SessionAffinitySelector", sessionSelector)
	}
}
