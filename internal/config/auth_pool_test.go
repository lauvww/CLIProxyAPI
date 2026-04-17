package config

import "testing"

func TestNormalizeAuthPoolSeedsMissingRoutingStrategies(t *testing.T) {
	poolA := `D:\CLIProxyAPI\auth\pool-a`
	poolB := `D:\CLIProxyAPI\auth\pool-b`

	cfg := &Config{
		AuthDir: poolA,
		Routing: RoutingConfig{Strategy: "fill-first"},
		AuthPool: AuthPoolConfig{
			Enabled:    true,
			Paths:      []string{poolA, poolB},
			ActivePath: poolA,
			RoutingStrategyByPath: map[string]string{
				poolA: "fill-first",
			},
		},
	}

	cfg.NormalizeAuthPool()

	if got := cfg.AuthPool.RoutingStrategyByPath[poolA]; got != "fill-first" {
		t.Fatalf("routing strategy for %q = %q, want %q", poolA, got, "fill-first")
	}
	if got := cfg.AuthPool.RoutingStrategyByPath[poolB]; got != "fill-first" {
		t.Fatalf("routing strategy for %q = %q, want %q", poolB, got, "fill-first")
	}
}

func TestSetCurrentAuthPoolPathAppliesPoolSpecificStrategy(t *testing.T) {
	poolA := `D:\CLIProxyAPI\auth\pool-a`
	poolB := `D:\CLIProxyAPI\auth\pool-b`

	cfg := &Config{
		AuthDir: poolA,
		Routing: RoutingConfig{Strategy: "round-robin"},
		AuthPool: AuthPoolConfig{
			Enabled:    true,
			Paths:      []string{poolA, poolB},
			ActivePath: poolA,
			RoutingStrategyByPath: map[string]string{
				poolA: "round-robin",
				poolB: "fill-first",
			},
		},
	}

	cfg.NormalizeAuthPool()
	cfg.SetCurrentAuthPoolPath(poolB)

	if got := cfg.CurrentAuthPoolPath(); got != poolB {
		t.Fatalf("CurrentAuthPoolPath() = %q, want %q", got, poolB)
	}
	if got := cfg.AuthDir; got != poolB {
		t.Fatalf("AuthDir = %q, want %q", got, poolB)
	}
	if got := cfg.Routing.Strategy; got != "fill-first" {
		t.Fatalf("Routing.Strategy = %q, want %q", got, "fill-first")
	}
}
