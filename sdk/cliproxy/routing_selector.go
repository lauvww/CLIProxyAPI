package cliproxy

import (
	"strings"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type resolvedRoutingSelectorConfig struct {
	strategy         string
	sessionAffinity  bool
	sessionTTLString string
	sessionTTL       time.Duration
}

func resolveRoutingSelectorConfig(cfg *config.Config) resolvedRoutingSelectorConfig {
	resolved := resolvedRoutingSelectorConfig{
		strategy:         "round-robin",
		sessionAffinity:  false,
		sessionTTLString: "1h",
		sessionTTL:       time.Hour,
	}
	if cfg == nil {
		return resolved
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Routing.Strategy)) {
	case "fill-first", "fillfirst", "ff":
		resolved.strategy = "fill-first"
	default:
		resolved.strategy = "round-robin"
	}

	resolved.sessionAffinity = cfg.Routing.ClaudeCodeSessionAffinity || cfg.Routing.SessionAffinity
	if ttl := strings.TrimSpace(cfg.Routing.SessionAffinityTTL); ttl != "" {
		resolved.sessionTTLString = ttl
		if parsed, err := time.ParseDuration(ttl); err == nil && parsed > 0 {
			resolved.sessionTTL = parsed
		}
	}

	return resolved
}

func buildRoutingSelector(cfg *config.Config) coreauth.Selector {
	resolved := resolveRoutingSelectorConfig(cfg)

	var selector coreauth.Selector
	switch resolved.strategy {
	case "fill-first":
		selector = &coreauth.FillFirstSelector{}
	default:
		selector = &coreauth.RoundRobinSelector{}
	}

	if resolved.sessionAffinity {
		selector = coreauth.NewSessionAffinitySelectorWithConfig(coreauth.SessionAffinityConfig{
			Fallback: selector,
			TTL:      resolved.sessionTTL,
		})
	}

	return selector
}
