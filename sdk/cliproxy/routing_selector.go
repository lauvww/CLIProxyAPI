package cliproxy

import (
	"sort"
	"strings"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/pathutil"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type resolvedRoutingSelectorConfig struct {
	strategy         string
	sessionAffinity  bool
	sessionTTLString string
	sessionTTL       time.Duration
	authPoolEnabled  bool
	authPoolMode     string
	authPoolRouting  string
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

	resolved.authPoolEnabled = cfg.AuthPool.Enabled
	resolved.authPoolMode = cfg.AuthPoolModeValue()
	resolved.authPoolRouting = authPoolRoutingSignature(cfg)

	return resolved
}

func buildRoutingSelector(cfg *config.Config) coreauth.Selector {
	resolved := resolveRoutingSelectorConfig(cfg)

	if cfg != nil && cfg.AuthPool.Enabled && cfg.AuthPoolModeValue() == "multi" {
		return coreauth.NewAuthPoolStrategySelector((*internalconfig.Config)(cfg))
	}

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

func authPoolRoutingSignature(cfg *config.Config) string {
	if cfg == nil || len(cfg.AuthPool.RoutingStrategyByPath) == 0 {
		return ""
	}

	pairs := make([]string, 0, len(cfg.AuthPool.RoutingStrategyByPath))
	for path, strategy := range cfg.AuthPool.RoutingStrategyByPath {
		if key := pathutil.NormalizeCompareKey(path); key != "" {
			pairs = append(pairs, key+"="+strings.ToLower(strings.TrimSpace(strategy)))
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "|")
}
