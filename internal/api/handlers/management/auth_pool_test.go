package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestPutCurrentAuthPoolPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	poolA := filepath.Join(tmpDir, "pool-a")
	poolB := filepath.Join(tmpDir, "pool-b")
	if err := os.MkdirAll(poolA, 0o755); err != nil {
		t.Fatalf("failed to create poolA: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auth-dir: "+poolA+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := &config.Config{AuthDir: poolA}
	cfg.SyncAuthPoolFromAuthDir()
	h := NewHandler(cfg, configPath, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body, errMarshal := json.Marshal(map[string]string{"path": poolB})
	if errMarshal != nil {
		t.Fatalf("failed to marshal body: %v", errMarshal)
	}
	ctx.Request = httptest.NewRequest(http.MethodPut, "/v0/management/auth-pool/current", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.PutCurrentAuthPoolPath(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if cfg.AuthDir != poolB {
		t.Fatalf("cfg.AuthDir = %q, want %q", cfg.AuthDir, poolB)
	}
	if cfg.AuthPool.ActivePath != poolB {
		t.Fatalf("cfg.AuthPool.ActivePath = %q, want %q", cfg.AuthPool.ActivePath, poolB)
	}
	found := false
	for _, path := range cfg.AuthPool.Paths {
		if path == poolB {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pool path %q to exist in auth pool paths: %#v", poolB, cfg.AuthPool.Paths)
	}
}

func TestDeleteAuthPoolPathRejectsActivePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	poolA := filepath.Join(tmpDir, "pool-a")
	poolB := filepath.Join(tmpDir, "pool-b")
	cfg := &config.Config{
		AuthDir: poolA,
		AuthPool: config.AuthPoolConfig{
			Paths:      []string{poolA, poolB},
			ActivePath: poolA,
		},
	}
	cfg.NormalizeAuthPool()
	h := NewHandlerWithoutConfigFilePath(cfg, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/v0/management/auth-pool/paths?path="+url.QueryEscape(poolA), nil)

	h.DeleteAuthPoolPath(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetAuthPool(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		AuthDir: "D:/auth/pool-a",
		AuthPool: config.AuthPoolConfig{
			Paths:      []string{"D:/auth/pool-a", "D:/auth/pool-b"},
			ActivePath: "D:/auth/pool-a",
		},
	}
	h := NewHandlerWithoutConfigFilePath(cfg, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-pool", nil)

	h.GetAuthPool(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got, ok := payload["active-path"].(string); !ok || got != `D:\auth\pool-a` {
		t.Fatalf("active-path = %v, want %q", payload["active-path"], `D:\auth\pool-a`)
	}
	if got, ok := payload["active_path"].(string); !ok || got != `D:\auth\pool-a` {
		t.Fatalf("active_path = %v, want %q", payload["active_path"], `D:\auth\pool-a`)
	}
	if got, ok := payload["auth_dir"].(string); !ok || got != `D:\auth\pool-a` {
		t.Fatalf("auth_dir = %v, want %q", payload["auth_dir"], `D:\auth\pool-a`)
	}
}

func TestPutAuthPoolEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	poolA := filepath.Join(tmpDir, "pool-a")
	if err := os.MkdirAll(poolA, 0o755); err != nil {
		t.Fatalf("failed to create poolA: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auth-dir: "+poolA+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := &config.Config{AuthDir: poolA}
	cfg.SyncAuthPoolFromAuthDir()
	h := NewHandler(cfg, configPath, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-pool/enabled", strings.NewReader(`{"enabled":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.PutAuthPoolEnabled(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !cfg.AuthPool.Enabled {
		t.Fatal("expected auth pool mode to be enabled")
	}
	if cfg.AuthDir != poolA {
		t.Fatalf("cfg.AuthDir = %q, want %q", cfg.AuthDir, poolA)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if enabled, ok := payload["enabled"].(bool); !ok || !enabled {
		t.Fatalf("enabled = %v, want true", payload["enabled"])
	}
}

func TestPutAuthPoolStrategy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	poolA := filepath.Join(tmpDir, "pool-a")
	if err := os.MkdirAll(poolA, 0o755); err != nil {
		t.Fatalf("failed to create poolA: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auth-dir: "+poolA+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := &config.Config{AuthDir: poolA, Routing: config.RoutingConfig{Strategy: "round-robin"}}
	cfg.SetAuthPoolEnabled(true)
	h := NewHandler(cfg, configPath, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-pool/strategy", strings.NewReader(`{"strategy":"fill-first"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.PutAuthPoolStrategy(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if cfg.Routing.Strategy != "fill-first" {
		t.Fatalf("cfg.Routing.Strategy = %q, want %q", cfg.Routing.Strategy, "fill-first")
	}
	if got := cfg.AuthPool.RoutingStrategyByPath[poolA]; got != "fill-first" {
		t.Fatalf("routing strategy for %q = %q, want %q", poolA, got, "fill-first")
	}
}
