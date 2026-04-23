package auth

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/pathutil"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type resolvedRequestAuthPool struct {
	Path     string
	Mode     string
	Fallback bool
}

// AuthPoolStrategySelector delegates to per-pool selectors so each auth pool
// keeps its own routing strategy and session-affinity cache.
type AuthPoolStrategySelector struct {
	defaultStrategy string
	sessionAffinity bool
	sessionTTL      time.Duration
	strategyByPath  map[string]string

	mu        sync.Mutex
	selectors map[string]Selector
}

func NewAuthPoolStrategySelector(cfg *internalconfig.Config) *AuthPoolStrategySelector {
	resolvedTTL := time.Hour
	if cfg != nil {
		if ttl := strings.TrimSpace(cfg.Routing.SessionAffinityTTL); ttl != "" {
			if parsed, err := time.ParseDuration(ttl); err == nil && parsed > 0 {
				resolvedTTL = parsed
			}
		}
	}

	strategyByPath := make(map[string]string)
	if cfg != nil {
		for rawPath, rawStrategy := range cfg.AuthPool.RoutingStrategyByPath {
			key := pathutil.NormalizeCompareKey(rawPath)
			if key == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(rawStrategy)) {
			case "fill-first", "fillfirst", "ff":
				strategyByPath[key] = "fill-first"
			default:
				strategyByPath[key] = "round-robin"
			}
		}
	}

	defaultStrategy := "round-robin"
	if cfg != nil {
		switch strings.ToLower(strings.TrimSpace(cfg.Routing.Strategy)) {
		case "fill-first", "fillfirst", "ff":
			defaultStrategy = "fill-first"
		}
	}

	return &AuthPoolStrategySelector{
		defaultStrategy: defaultStrategy,
		sessionAffinity: cfg != nil && (cfg.Routing.SessionAffinity || cfg.Routing.ClaudeCodeSessionAffinity),
		sessionTTL:      resolvedTTL,
		strategyByPath:  strategyByPath,
		selectors:       make(map[string]Selector),
	}
}

func (s *AuthPoolStrategySelector) Pick(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error) {
	selector := s.selectorForPool(authPoolPathFromMetadata(opts.Metadata))
	return selector.Pick(ctx, provider, model, opts, auths)
}

func (s *AuthPoolStrategySelector) Stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, selector := range s.selectors {
		if stoppable, ok := selector.(StoppableSelector); ok && selector != nil {
			stoppable.Stop()
		}
	}
	s.selectors = make(map[string]Selector)
}

func (s *AuthPoolStrategySelector) selectorForPool(poolPath string) Selector {
	if s == nil {
		return &RoundRobinSelector{}
	}

	key := pathutil.NormalizeCompareKey(poolPath)
	if key == "" {
		key = "__default__"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if selector, ok := s.selectors[key]; ok && selector != nil {
		return selector
	}

	strategy := s.defaultStrategy
	if poolPathKey := pathutil.NormalizeCompareKey(poolPath); poolPathKey != "" {
		if byPath, ok := s.strategyByPath[poolPathKey]; ok && strings.TrimSpace(byPath) != "" {
			strategy = byPath
		}
	}

	selector := buildRoutingSelectorForStrategy(strategy, s.sessionAffinity, s.sessionTTL)
	s.selectors[key] = selector
	return selector
}

func buildRoutingSelectorForStrategy(strategy string, sessionAffinity bool, sessionTTL time.Duration) Selector {
	var selector Selector
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "fill-first", "fillfirst", "ff":
		selector = &FillFirstSelector{}
	default:
		selector = &RoundRobinSelector{}
	}

	if sessionAffinity {
		selector = NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{
			Fallback: selector,
			TTL:      sessionTTL,
		})
	}

	return selector
}

func resolveRequestAuthPool(cfg *internalconfig.Config, meta map[string]any) resolvedRequestAuthPool {
	if existing := authPoolPathFromMetadata(meta); existing != "" {
		return resolvedRequestAuthPool{
			Path:     existing,
			Mode:     authPoolModeFromMetadata(meta),
			Fallback: authPoolFallbackFromMetadata(meta),
		}
	}

	if cfg == nil {
		return resolvedRequestAuthPool{}
	}

	path, explicit, fallback := cfg.ResolveAuthPoolForAPIKey(clientAPIKeyFromMetadata(meta))
	resolved := resolvedRequestAuthPool{
		Path:     path,
		Mode:     cfg.AuthPoolModeValue(),
		Fallback: !explicit && fallback,
	}
	publishResolvedAuthPoolMetadata(meta, resolved)
	return resolved
}

func publishResolvedAuthPoolMetadata(meta map[string]any, resolved resolvedRequestAuthPool) {
	if len(meta) == 0 {
		return
	}
	if resolved.Path != "" {
		meta[cliproxyexecutor.ResolvedAuthPoolMetadataKey] = resolved.Path
	}
	if resolved.Mode != "" {
		meta[cliproxyexecutor.ResolvedAuthPoolModeMetadataKey] = resolved.Mode
	}
	meta[cliproxyexecutor.ResolvedAuthPoolFallbackMetadataKey] = resolved.Fallback
}

func clientAPIKeyFromMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[cliproxyexecutor.ClientAPIKeyMetadataKey]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case []byte:
		return strings.TrimSpace(string(value))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func authPoolPathFromMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[cliproxyexecutor.ResolvedAuthPoolMetadataKey]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return pathutil.NormalizePath(value)
	case []byte:
		return pathutil.NormalizePath(string(value))
	default:
		return pathutil.NormalizePath(fmt.Sprint(value))
	}
}

func authPoolModeFromMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return "single"
	}
	raw, ok := meta[cliproxyexecutor.ResolvedAuthPoolModeMetadataKey]
	if !ok || raw == nil {
		return "single"
	}
	switch value := raw.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(value))
	case []byte:
		return strings.ToLower(strings.TrimSpace(string(value)))
	default:
		return "single"
	}
}

func authPoolFallbackFromMetadata(meta map[string]any) bool {
	if len(meta) == 0 {
		return false
	}
	raw, ok := meta[cliproxyexecutor.ResolvedAuthPoolFallbackMetadataKey]
	if !ok || raw == nil {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		return err == nil && parsed
	default:
		return false
	}
}

func authBelongsToResolvedPool(auth *Auth, resolved resolvedRequestAuthPool) bool {
	if auth == nil || resolved.Mode != "multi" {
		return true
	}
	if resolved.Path == "" {
		return true
	}
	if auth.Attributes == nil {
		return false
	}
	authPool := pathutil.NormalizePath(strings.TrimSpace(auth.Attributes["auth_pool"]))
	if authPool == "" {
		return false
	}
	return pathutil.PathsEqual(authPool, resolved.Path)
}

func filterAuthsByResolvedPool(auths []*Auth, resolved resolvedRequestAuthPool) []*Auth {
	if resolved.Mode != "multi" || resolved.Path == "" {
		return auths
	}
	filtered := make([]*Auth, 0, len(auths))
	for _, auth := range auths {
		if authBelongsToResolvedPool(auth, resolved) {
			filtered = append(filtered, auth)
		}
	}
	return filtered
}

func inferPoolTypeFromPath(path string) string {
	normalized := strings.ReplaceAll(pathutil.NormalizePath(path), "/", "\\")
	if normalized == "" {
		return "custom"
	}
	parts := strings.Split(normalized, "\\")
	for idx := len(parts) - 1; idx >= 0; idx-- {
		switch strings.ToLower(strings.TrimSpace(parts[idx])) {
		case "plus":
			return "plus"
		case "free":
			return "free"
		}
	}
	return "custom"
}
