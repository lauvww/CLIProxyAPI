package watcher

import (
	"os"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/pathutil"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestSwitchAuthDirUpdatesValueWithoutWatcher(t *testing.T) {
	w := &Watcher{authDir: "D:/auth/pool-a"}

	w.switchAuthDir("D:/auth/pool-b")

	if got := w.authDir; got != pathutil.NormalizePath("D:/auth/pool-b") {
		t.Fatalf("authDir = %q, want %q", got, pathutil.NormalizePath("D:/auth/pool-b"))
	}
}

func TestSwitchAuthDirSkipsEmptyValue(t *testing.T) {
	w := &Watcher{authDir: "D:/auth/pool-a"}

	w.switchAuthDir("   ")

	if got := w.authDir; got != "D:/auth/pool-a" {
		t.Fatalf("authDir = %q, want unchanged", got)
	}
}

func TestApplyConfigRefreshesRuntimeAuthStateWhenAuthDirChanges(t *testing.T) {
	queue := make(chan AuthUpdate, 8)
	root := t.TempDir()
	oldDir := pathutil.NormalizePath(root + "\\pool-a")
	newDir := pathutil.NormalizePath(root + "\\pool-b")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("failed to create next auth dir: %v", err)
	}

	oldCfg := &config.Config{AuthDir: oldDir}
	newCfg := &config.Config{AuthDir: newDir}

	w := &Watcher{
		authDir:        oldCfg.AuthDir,
		authDirs:       []string{pathutil.NormalizePath(oldCfg.AuthDir)},
		lastAuthHashes: make(map[string]string),
	}
	w.SetConfig(oldCfg)
	w.currentAuths = map[string]*coreauth.Auth{
		"pool-a-auth": {ID: "pool-a-auth", Provider: "codex"},
	}
	w.SetAuthUpdateQueue(queue)
	defer w.stopDispatch()

	origSnapshot := snapshotCoreAuthsFunc
	snapshotCoreAuthsFunc = func(cfg *config.Config, authDirs []string) []*coreauth.Auth {
		if len(authDirs) == 1 && pathutil.PathsEqual(authDirs[0], newDir) {
			return []*coreauth.Auth{{ID: "pool-b-auth", Provider: "codex"}}
		}
		return []*coreauth.Auth{{ID: "pool-a-auth", Provider: "codex"}}
	}
	defer func() { snapshotCoreAuthsFunc = origSnapshot }()

	w.ApplyConfig(newCfg)

	deadline := time.After(2 * time.Second)
	seenAdd := false
	for !seenAdd {
		select {
		case update := <-queue:
			if update.Action == AuthUpdateActionAdd && update.ID == "pool-b-auth" {
				seenAdd = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for pool-b auth refresh after ApplyConfig")
		}
	}
}
