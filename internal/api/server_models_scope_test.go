package api

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestCurrentModelsViewHeadersUsesActiveAuthPoolViewWhenEnabled(t *testing.T) {
	cfg := &config.Config{
		AuthDir: `C:\Users\ww\.cli-proxy-api\Pro`,
		AuthPool: config.AuthPoolConfig{
			Enabled:    true,
			ActivePath: `C:\Users\ww\.cli-proxy-api\Pro`,
		},
	}

	scope, mode, authDir, authPool := currentModelsViewHeaders(cfg)
	if scope != "runtime" {
		t.Fatalf("unexpected scope: %s", scope)
	}
	if mode != "active-auth-pool-view" {
		t.Fatalf("unexpected mode: %s", mode)
	}
	if authDir != `C:\Users\ww\.cli-proxy-api\Pro` {
		t.Fatalf("unexpected auth dir: %s", authDir)
	}
	if authPool != `C:\Users\ww\.cli-proxy-api\Pro` {
		t.Fatalf("unexpected auth pool: %s", authPool)
	}
}

func TestCurrentModelsViewHeadersFallsBackToAuthDirView(t *testing.T) {
	cfg := &config.Config{
		AuthDir: `C:\Users\ww\.cli-proxy-api\Free`,
	}

	scope, mode, authDir, authPool := currentModelsViewHeaders(cfg)
	if scope != "runtime" {
		t.Fatalf("unexpected scope: %s", scope)
	}
	if mode != "auth-dir-view" {
		t.Fatalf("unexpected mode: %s", mode)
	}
	if authDir != `C:\Users\ww\.cli-proxy-api\Free` {
		t.Fatalf("unexpected auth dir: %s", authDir)
	}
	if authPool != "" {
		t.Fatalf("expected empty auth pool, got: %s", authPool)
	}
}
