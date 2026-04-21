package cliproxy

import (
	"context"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestApplyModelCatalogRuntimeConfig_UsesConfigWithoutCLIOverride(t *testing.T) {
	registry.StopModelsUpdater()
	t.Cleanup(func() {
		registry.StopModelsUpdater()
	})

	disabledCfg := &config.Config{}
	if effective := applyModelCatalogRuntimeConfig(context.Background(), nil, disabledCfg); effective {
		t.Fatal("expected remote refresh to stay disabled by default")
	}
	if registry.ModelsUpdaterEnabled() {
		t.Fatal("expected updater to remain stopped when config leaves remote refresh disabled")
	}

	nextCfg := &config.Config{}
	nextCfg.ModelCatalog.RemoteRefreshEnabled = true

	effective := applyModelCatalogRuntimeConfig(context.Background(), nil, nextCfg)
	if !effective {
		t.Fatal("expected remote refresh to be enabled by config")
	}
	if !nextCfg.ModelCatalog.RemoteRefreshEffective {
		t.Fatal("expected runtime metadata to mark remote refresh effective")
	}
	if nextCfg.ModelCatalog.RemoteRefreshForcedByCLI != "" {
		t.Fatalf("expected no CLI override, got %q", nextCfg.ModelCatalog.RemoteRefreshForcedByCLI)
	}
	if !registry.ModelsUpdaterEnabled() {
		t.Fatal("expected updater to start when config enables remote refresh")
	}
}

func TestApplyModelCatalogRuntimeConfig_CLIOverrideWins(t *testing.T) {
	registry.StopModelsUpdater()
	t.Cleanup(func() {
		registry.StopModelsUpdater()
	})

	forcedRemoteCfg := &config.Config{}
	forcedRemoteCfg.ModelCatalog.RemoteRefreshForcedByCLI = internalconfig.ModelCatalogCLIOverrideRemote
	registry.SetModelsUpdaterEnabled(context.Background(), true)

	nextCfg := &config.Config{}
	nextCfg.ModelCatalog.RemoteRefreshEnabled = false

	effective := applyModelCatalogRuntimeConfig(context.Background(), forcedRemoteCfg, nextCfg)
	if !effective {
		t.Fatal("expected --remote-model override to force remote refresh on")
	}
	if nextCfg.ModelCatalog.RemoteRefreshForcedByCLI != internalconfig.ModelCatalogCLIOverrideRemote {
		t.Fatalf("expected remote override metadata, got %q", nextCfg.ModelCatalog.RemoteRefreshForcedByCLI)
	}
	if !registry.ModelsUpdaterEnabled() {
		t.Fatal("expected updater to remain enabled under --remote-model override")
	}

	forcedLocalCfg := &config.Config{}
	forcedLocalCfg.ModelCatalog.RemoteRefreshForcedByCLI = internalconfig.ModelCatalogCLIOverrideLocal
	forcedLocalCfg.ModelCatalog.RemoteRefreshEnabled = true
	registry.StopModelsUpdater()

	nextCfg = &config.Config{}
	nextCfg.ModelCatalog.RemoteRefreshEnabled = true

	effective = applyModelCatalogRuntimeConfig(context.Background(), forcedLocalCfg, nextCfg)
	if effective {
		t.Fatal("expected --local-model override to force remote refresh off")
	}
	if nextCfg.ModelCatalog.RemoteRefreshForcedByCLI != internalconfig.ModelCatalogCLIOverrideLocal {
		t.Fatalf("expected local override metadata, got %q", nextCfg.ModelCatalog.RemoteRefreshForcedByCLI)
	}
	if registry.ModelsUpdaterEnabled() {
		t.Fatal("expected updater to stay disabled under --local-model override")
	}
}
