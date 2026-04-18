//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTrayServiceConfigReloadsSecretKeyFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	authDir := filepath.Join(tmpDir, "auths")

	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	configBody := "port: 8317\n" +
		"auth-dir: " + authDir + "\n" +
		"remote-management:\n" +
		"  secret-key: new-password\n"

	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := loadTrayServiceConfig(nil, configPath)
	if err != nil {
		t.Fatalf("loadTrayServiceConfig returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config to be loaded")
	}
	if got := cfg.RemoteManagement.SecretKey; got == "" || got == "new-password" {
		t.Fatalf("expected secret-key to be hashed after reload, got %q", got)
	}
	if got := cfg.AuthDir; got != authDir {
		t.Fatalf("AuthDir = %q, want %q", got, authDir)
	}
}
