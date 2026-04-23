package auth

import (
	"context"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestResolveRequestAuthPoolFromMetadata(t *testing.T) {
	cfg := &internalconfig.Config{
		SDKConfig: internalconfig.SDKConfig{
			APIKeys: []string{"sk-a"},
		},
		AuthDir: `C:\Auth\Default`,
		AuthPool: internalconfig.AuthPoolConfig{
			Enabled: true,
			Mode:    "multi",
			Paths: []string{
				`C:\Auth\Default`,
				`C:\Auth\Plus`,
			},
			ActivePath: `C:\Auth\Default`,
			APIKeyBindings: map[string]string{
				"sk-a": `C:\Auth\Plus`,
			},
		},
	}
	cfg.NormalizeAuthPool()

	meta := map[string]any{
		cliproxyexecutor.ClientAPIKeyMetadataKey: "sk-a",
	}
	resolved := resolveRequestAuthPool(cfg, meta)
	if resolved.Path != `C:\Auth\Plus` || resolved.Mode != "multi" || resolved.Fallback {
		t.Fatalf("resolved = %+v", resolved)
	}

	meta = map[string]any{
		cliproxyexecutor.ClientAPIKeyMetadataKey: "sk-missing",
	}
	resolved = resolveRequestAuthPool(cfg, meta)
	if resolved.Path != `C:\Auth\Default` || resolved.Mode != "multi" || !resolved.Fallback {
		t.Fatalf("fallback resolved = %+v", resolved)
	}
}

func TestAuthPoolStrategySelectorUsesPerPoolStrategy(t *testing.T) {
	cfg := &internalconfig.Config{
		Routing: internalconfig.RoutingConfig{Strategy: "round-robin"},
		AuthPool: internalconfig.AuthPoolConfig{
			Enabled: true,
			Mode:    "multi",
			RoutingStrategyByPath: map[string]string{
				`C:\Auth\Plus`: "fill-first",
			},
		},
	}
	selector := NewAuthPoolStrategySelector(cfg)

	auths := []*Auth{
		{ID: "a1"},
		{ID: "a2"},
	}

	first, err := selector.Pick(context.Background(), "claude", "claude-sonnet-4", cliproxyexecutor.Options{
		Metadata: map[string]any{
			cliproxyexecutor.ResolvedAuthPoolMetadataKey:     `C:\Auth\Default`,
			cliproxyexecutor.ResolvedAuthPoolModeMetadataKey: "multi",
		},
	}, auths)
	if err != nil {
		t.Fatalf("pick default pool first: %v", err)
	}
	second, err := selector.Pick(context.Background(), "claude", "claude-sonnet-4", cliproxyexecutor.Options{
		Metadata: map[string]any{
			cliproxyexecutor.ResolvedAuthPoolMetadataKey:     `C:\Auth\Default`,
			cliproxyexecutor.ResolvedAuthPoolModeMetadataKey: "multi",
		},
	}, auths)
	if err != nil {
		t.Fatalf("pick default pool second: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected round-robin behavior for default pool, got %q twice", first.ID)
	}

	fillFirstA, err := selector.Pick(context.Background(), "claude", "claude-sonnet-4", cliproxyexecutor.Options{
		Metadata: map[string]any{
			cliproxyexecutor.ResolvedAuthPoolMetadataKey:     `C:\Auth\Plus`,
			cliproxyexecutor.ResolvedAuthPoolModeMetadataKey: "multi",
		},
	}, auths)
	if err != nil {
		t.Fatalf("pick plus pool first: %v", err)
	}
	fillFirstB, err := selector.Pick(context.Background(), "claude", "claude-sonnet-4", cliproxyexecutor.Options{
		Metadata: map[string]any{
			cliproxyexecutor.ResolvedAuthPoolMetadataKey:     `C:\Auth\Plus`,
			cliproxyexecutor.ResolvedAuthPoolModeMetadataKey: "multi",
		},
	}, auths)
	if err != nil {
		t.Fatalf("pick plus pool second: %v", err)
	}
	if fillFirstA.ID != fillFirstB.ID {
		t.Fatalf("expected fill-first behavior for plus pool, got %q then %q", fillFirstA.ID, fillFirstB.ID)
	}
}
