package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestModelsUpdaterLifecycleAndRestart(t *testing.T) {
	StopModelsUpdater()

	originalURLs := append([]string(nil), modelsURLs...)
	originalData, err := json.Marshal(getModels())
	if err != nil {
		t.Fatalf("marshal original models: %v", err)
	}

	refreshCallbackMu.Lock()
	originalCallback := refreshCallback
	originalPending := append([]string(nil), pendingRefreshChanges...)
	refreshCallback = nil
	pendingRefreshChanges = nil
	refreshCallbackMu.Unlock()

	t.Cleanup(func() {
		StopModelsUpdater()
		modelsURLs = originalURLs
		if errRestore := loadModelsFromBytes(originalData, "test-restore"); errRestore != nil {
			t.Fatalf("restore original models: %v", errRestore)
		}
		refreshCallbackMu.Lock()
		refreshCallback = originalCallback
		pendingRefreshChanges = originalPending
		refreshCallbackMu.Unlock()
	})

	if err := loadModelsFromBytes(embeddedModelsJSON, "test-reset"); err != nil {
		t.Fatalf("reset embedded models: %v", err)
	}

	var modified staticModelsJSON
	if err := json.Unmarshal(embeddedModelsJSON, &modified); err != nil {
		t.Fatalf("decode embedded models: %v", err)
	}
	if len(modified.Claude) == 0 || modified.Claude[0] == nil {
		t.Fatal("expected embedded Claude models")
	}
	modified.Claude[0] = cloneModelInfo(modified.Claude[0])
	modified.Claude[0].ID = modified.Claude[0].ID + "-remote-refresh-test"

	modifiedJSON, err := json.Marshal(&modified)
	if err != nil {
		t.Fatalf("marshal modified models: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(modifiedJSON)
	}))
	defer server.Close()

	modelsURLs = []string{server.URL}

	assertRefresh := func() {
		changesCh := make(chan []string, 1)
		SetModelRefreshCallback(func(changedProviders []string) {
			changesCh <- append([]string(nil), changedProviders...)
		})

		if started := StartModelsUpdater(context.Background()); !started {
			t.Fatal("expected updater to start")
		}
		if started := StartModelsUpdater(context.Background()); started {
			t.Fatal("expected duplicate start to be ignored")
		}
		if !ModelsUpdaterEnabled() {
			t.Fatal("expected updater to report active")
		}

		select {
		case changed := <-changesCh:
			if !slices.Contains(changed, "claude") {
				t.Fatalf("expected claude refresh callback, got %v", changed)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for model refresh callback")
		}

		if stopped := StopModelsUpdater(); !stopped {
			t.Fatal("expected updater to stop")
		}
		if stopped := StopModelsUpdater(); stopped {
			t.Fatal("expected duplicate stop to be ignored")
		}
		if ModelsUpdaterEnabled() {
			t.Fatal("expected updater to report inactive after stop")
		}
	}

	assertRefresh()

	if err := loadModelsFromBytes(embeddedModelsJSON, "test-reset-restart"); err != nil {
		t.Fatalf("reset embedded models for restart: %v", err)
	}

	assertRefresh()
}
