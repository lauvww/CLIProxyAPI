package config

import "testing"

func TestNormalizeAuthPoolMultiBindings(t *testing.T) {
	cfg := &Config{
		SDKConfig: SDKConfig{
			APIKeys: []string{"sk-a", "sk-b"},
		},
		AuthDir: `C:\Auth\Default`,
		Routing: RoutingConfig{Strategy: "round-robin"},
		AuthPool: AuthPoolConfig{
			Enabled: true,
			Mode:    "multi",
			Paths: []string{
				`C:\Auth\Default\`,
				`C:\Auth\Plus`,
			},
			ActivePath: `C:\Auth\Default`,
			APIKeyBindings: map[string]string{
				"sk-a": `C:\Auth\Plus\`,
				"sk-b": `C:\Auth\Missing`,
			},
		},
	}

	cfg.NormalizeAuthPool()

	if got := cfg.AuthPool.Mode; got != "multi" {
		t.Fatalf("AuthPool.Mode = %q, want multi", got)
	}
	if len(cfg.AuthPool.Paths) != 2 {
		t.Fatalf("AuthPool.Paths length = %d, want 2", len(cfg.AuthPool.Paths))
	}
	if got := cfg.AuthPool.APIKeyBindings["sk-a"]; got != `C:\Auth\Plus` {
		t.Fatalf("APIKeyBindings[sk-a] = %q, want %q", got, `C:\Auth\Plus`)
	}
	if _, ok := cfg.AuthPool.APIKeyBindings["sk-b"]; ok {
		t.Fatal("expected binding pointing to an unknown pool path to be removed")
	}

	path, explicit, fallback := cfg.ResolveAuthPoolForAPIKey("sk-a")
	if path != `C:\Auth\Plus` || !explicit || fallback {
		t.Fatalf("ResolveAuthPoolForAPIKey(sk-a) = (%q, %t, %t)", path, explicit, fallback)
	}

	path, explicit, fallback = cfg.ResolveAuthPoolForAPIKey("sk-b")
	if path != `C:\Auth\Default` || explicit || !fallback {
		t.Fatalf("ResolveAuthPoolForAPIKey(sk-b) = (%q, %t, %t)", path, explicit, fallback)
	}
}
