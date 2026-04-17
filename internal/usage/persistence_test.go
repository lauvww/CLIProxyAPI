package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolvePersistenceFilePathUsesPoolSpecificName(t *testing.T) {
	t.Setenv(usagePersistenceDirEnv, t.TempDir())
	poolAPath := ResolvePersistenceFilePath("", `D:\CLIProxyAPI\auth\pool-a`)
	poolBPath := ResolvePersistenceFilePath("", `D:\CLIProxyAPI\auth\pool-b`)
	defaultPath := ResolvePersistenceFilePath("", "")

	if poolAPath == poolBPath {
		t.Fatalf("expected distinct persistence paths for different pools, got %q", poolAPath)
	}
	if got := filepath.Base(poolAPath); got != "pool-a"+usagePersistenceFileExt {
		t.Fatalf("expected pool-specific file name, got %q", got)
	}
	if filepath.Base(defaultPath) != usagePersistenceDefaultTag+usagePersistenceFileExt {
		t.Fatalf("expected default file name, got %q", filepath.Base(defaultPath))
	}
}

func TestResolvePersistenceFilePathUsesSanitizedPoolName(t *testing.T) {
	t.Setenv(usagePersistenceDirEnv, t.TempDir())
	path := ResolvePersistenceFilePath("", `C:\Users\ww\.cli-proxy-api\Pro`)
	if filepath.Base(path) != "Pro"+usagePersistenceFileExt {
		t.Fatalf("expected Pro.json, got %q", filepath.Base(path))
	}
}

func TestSaveAndLoadSnapshotFiltersCurrentAuthPool(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(usagePersistenceDirEnv, dir)
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	authPool := `D:\CLIProxyAPI\auth\pool-a`
	snapshot := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"POST /v1/responses": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{
							{
								Timestamp: now.Add(-time.Hour),
								AuthPool:  authPool,
								Tokens:    TokenStats{TotalTokens: 10},
							},
							{
								Timestamp: now.Add(-2 * time.Hour),
								AuthPool:  `D:\CLIProxyAPI\auth\pool-b`,
								Tokens:    TokenStats{TotalTokens: 20},
							},
						},
					},
				},
			},
		},
	}

	path, errSave := SaveSnapshot("", authPool, snapshot, now)
	if errSave != nil {
		t.Fatalf("SaveSnapshot returned error: %v", errSave)
	}
	if _, errStat := os.Stat(path); errStat != nil {
		t.Fatalf("expected persisted file %q: %v", path, errStat)
	}

	loaded, loadedPath, found, errLoad := LoadRecentSnapshot("", authPool, now, 7)
	if errLoad != nil {
		t.Fatalf("LoadRecentSnapshot returned error: %v", errLoad)
	}
	if !found {
		t.Fatal("expected persisted snapshot to be found")
	}
	if loadedPath != path {
		t.Fatalf("loadedPath = %q, want %q", loadedPath, path)
	}
	if loaded.TotalRequests != 1 {
		t.Fatalf("loaded.TotalRequests = %d, want 1", loaded.TotalRequests)
	}
	model := loaded.APIs["POST /v1/responses"].Models["gpt-5.4"]
	if len(model.Details) != 1 {
		t.Fatalf("len(model.Details) = %d, want 1", len(model.Details))
	}
	if normalizeAuthPool(model.Details[0].AuthPool) != normalizeAuthPool(authPool) {
		t.Fatalf("loaded auth pool = %q, want %q", model.Details[0].AuthPool, authPool)
	}
}

func TestListAuthPoolsReturnsDistinctNormalizedPools(t *testing.T) {
	snapshot := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"POST /v1/responses": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{
							{AuthPool: `D:\CLIProxyAPI\Auth\Pool-A`},
							{AuthPool: `d:/cliproxyapi/auth/pool-a/`},
							{AuthPool: `D:\CLIProxyAPI\Auth\Pool-B`},
						},
					},
				},
			},
		},
	}

	pools := ListAuthPools(snapshot)
	if len(pools) != 2 {
		t.Fatalf("len(pools) = %d, want 2 (%v)", len(pools), pools)
	}
	if pools[0] != normalizeAuthPool(`D:\CLIProxyAPI\Auth\Pool-A`) {
		t.Fatalf("pools[0] = %q, want %q", pools[0], normalizeAuthPool(`D:\CLIProxyAPI\Auth\Pool-A`))
	}
	if pools[1] != normalizeAuthPool(`D:\CLIProxyAPI\Auth\Pool-B`) {
		t.Fatalf("pools[1] = %q, want %q", pools[1], normalizeAuthPool(`D:\CLIProxyAPI\Auth\Pool-B`))
	}
}

func TestLoadRecentSnapshotFallsBackToLegacyFileName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(usagePersistenceDirEnv, dir)
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	authPool := `D:\CLIProxyAPI\auth\Pro`
	snapshot := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"POST /v1/responses": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{
							{
								Timestamp: now.Add(-time.Hour),
								AuthPool:  authPool,
								Tokens:    TokenStats{TotalTokens: 10},
							},
						},
					},
				},
			},
		},
	}

	legacyPath := resolveLegacyPersistenceFilePath("", authPool)
	payload := persistedUsagePayload{
		Version:    usagePersistencePayloadVer,
		ExportedAt: now,
		AuthPool:   normalizeAuthPool(authPool),
		Usage:      FilterSnapshotByAuthPool(snapshot, authPool),
	}
	data, errMarshal := json.MarshalIndent(payload, "", "  ")
	if errMarshal != nil {
		t.Fatalf("json.MarshalIndent returned error: %v", errMarshal)
	}
	if errWrite := os.WriteFile(legacyPath, data, 0o644); errWrite != nil {
		t.Fatalf("os.WriteFile returned error: %v", errWrite)
	}

	loaded, loadedPath, found, errLoad := LoadRecentSnapshot("", authPool, now, 7)
	if errLoad != nil {
		t.Fatalf("LoadRecentSnapshot returned error: %v", errLoad)
	}
	if !found {
		t.Fatal("expected persisted snapshot to be found")
	}
	if loadedPath != legacyPath {
		t.Fatalf("loadedPath = %q, want %q", loadedPath, legacyPath)
	}
	if loaded.TotalRequests != 1 {
		t.Fatalf("loaded.TotalRequests = %d, want 1", loaded.TotalRequests)
	}
}
