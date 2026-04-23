// Package watcher watches config/auth files and triggers hot reloads.
// It supports cross-platform fsnotify event handling.
package watcher

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/pathutil"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher/diff"
	"gopkg.in/yaml.v3"

	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// storePersister captures persistence-capable token store methods used by the watcher.
type storePersister interface {
	PersistConfig(ctx context.Context) error
	PersistAuthFiles(ctx context.Context, message string, paths ...string) error
}

type authDirProvider interface {
	AuthDir() string
}

// Watcher manages file watching for configuration and authentication files
type Watcher struct {
	configPath        string
	authDir           string
	authDirs          []string
	config            *config.Config
	clientsMutex      sync.RWMutex
	configReloadMu    sync.Mutex
	configReloadTimer *time.Timer
	serverUpdateMu    sync.Mutex
	serverUpdateTimer *time.Timer
	serverUpdateLast  time.Time
	serverUpdatePend  bool
	stopped           atomic.Bool
	reloadCallback    func(*config.Config)
	watcher           *fsnotify.Watcher
	lastAuthHashes    map[string]string
	lastAuthContents  map[string]*coreauth.Auth
	fileAuthsByPath   map[string]map[string]*coreauth.Auth
	lastRemoveTimes   map[string]time.Time
	lastConfigHash    string
	authQueue         chan<- AuthUpdate
	currentAuths      map[string]*coreauth.Auth
	runtimeAuths      map[string]*coreauth.Auth
	dispatchMu        sync.Mutex
	dispatchCond      *sync.Cond
	pendingUpdates    map[string]AuthUpdate
	pendingOrder      []string
	dispatchCancel    context.CancelFunc
	storePersister    storePersister
	mirroredAuthDir   string
	oldConfigYaml     []byte
}

// AuthUpdateAction represents the type of change detected in auth sources.
type AuthUpdateAction string

const (
	AuthUpdateActionAdd    AuthUpdateAction = "add"
	AuthUpdateActionModify AuthUpdateAction = "modify"
	AuthUpdateActionDelete AuthUpdateAction = "delete"
)

// AuthUpdate describes an incremental change to auth configuration.
type AuthUpdate struct {
	Action AuthUpdateAction
	ID     string
	Auth   *coreauth.Auth
}

const (
	// replaceCheckDelay is a short delay to allow atomic replace (rename) to settle
	// before deciding whether a Remove event indicates a real deletion.
	replaceCheckDelay        = 50 * time.Millisecond
	configReloadDebounce     = 150 * time.Millisecond
	authRemoveDebounceWindow = 1 * time.Second
	serverUpdateDebounce     = 1 * time.Second
)

// NewWatcher creates a new file watcher instance
func NewWatcher(configPath, authDir string, reloadCallback func(*config.Config)) (*Watcher, error) {
	watcher, errNewWatcher := fsnotify.NewWatcher()
	if errNewWatcher != nil {
		return nil, errNewWatcher
	}
	w := &Watcher{
		configPath:      configPath,
		authDir:         authDir,
		authDirs:        uniqueAuthDirs([]string{authDir}),
		reloadCallback:  reloadCallback,
		watcher:         watcher,
		lastAuthHashes:  make(map[string]string),
		fileAuthsByPath: make(map[string]map[string]*coreauth.Auth),
	}
	w.dispatchCond = sync.NewCond(&w.dispatchMu)
	if store := sdkAuth.GetTokenStore(); store != nil {
		if persister, ok := store.(storePersister); ok {
			w.storePersister = persister
			log.Debug("persistence-capable token store detected; watcher will propagate persisted changes")
		}
		if provider, ok := store.(authDirProvider); ok {
			if fixed := strings.TrimSpace(provider.AuthDir()); fixed != "" {
				w.mirroredAuthDir = fixed
				log.Debugf("mirrored auth directory locked to %s", fixed)
			}
		}
	}
	return w, nil
}

// Start begins watching the configuration file and authentication directory
func (w *Watcher) Start(ctx context.Context) error {
	return w.start(ctx)
}

// Stop stops the file watcher
func (w *Watcher) Stop() error {
	w.stopped.Store(true)
	w.stopDispatch()
	w.stopConfigReloadTimer()
	w.stopServerUpdateTimer()
	return w.watcher.Close()
}

// SetConfig updates the current configuration
func (w *Watcher) SetConfig(cfg *config.Config) {
	w.clientsMutex.Lock()
	defer w.clientsMutex.Unlock()
	w.config = cfg
	w.authDirs = configAuthDirs(cfg, w.authDir)
	if len(w.authDirs) > 0 {
		w.authDir = w.authDirs[0]
	}
	w.oldConfigYaml, _ = yaml.Marshal(cfg)
}

// ApplyConfig updates the watcher config cache and switches the watched auth
// directory immediately when auth-dir changes at runtime.
func (w *Watcher) ApplyConfig(cfg *config.Config) {
	if w == nil || cfg == nil {
		return
	}

	w.clientsMutex.RLock()
	previousCfg := w.config
	previousAuthDirs := append([]string(nil), w.authDirs...)
	currentAuthDir := w.authDir
	w.clientsMutex.RUnlock()

	w.SetConfig(cfg)
	w.syncConfigHashFromDisk()

	nextAuthDirs := configAuthDirs(cfg, w.authDir)
	if len(nextAuthDirs) == 0 {
		return
	}

	authDirChanged := !authDirSetsEqual(previousAuthDirs, nextAuthDirs)
	w.switchAuthDirs(nextAuthDirs)

	var affectedOAuthProviders []string
	if previousCfg != nil {
		_, affectedOAuthProviders = diff.DiffOAuthExcludedModelChanges(
			previousCfg.OAuthExcludedModels,
			cfg.OAuthExcludedModels,
		)
	}
	retryConfigChanged := previousCfg != nil &&
		(previousCfg.RequestRetry != cfg.RequestRetry ||
			previousCfg.MaxRetryInterval != cfg.MaxRetryInterval ||
			previousCfg.MaxRetryCredentials != cfg.MaxRetryCredentials)
	forceAuthRefresh := previousCfg != nil &&
		(previousCfg.ForceModelPrefix != cfg.ForceModelPrefix ||
			!reflect.DeepEqual(previousCfg.OAuthModelAlias, cfg.OAuthModelAlias) ||
			retryConfigChanged)

	if shouldReloadAuthClients(previousCfg, cfg, authDirChanged, affectedOAuthProviders, forceAuthRefresh) {
		if authDirChanged {
			w.rebuildFileAuthCache(cfg)
		}
		w.refreshAuthState(forceAuthRefresh || authDirChanged || len(affectedOAuthProviders) > 0)
		log.Infof(
			"runtime auth state refreshed after ApplyConfig (authDirsChanged=%t previous=%s next=%s)",
			authDirChanged,
			strings.Join(previousAuthDirs, ", "),
			strings.Join(nextAuthDirs, ", "),
		)
		return
	}

	log.Debugf(
		"ApplyConfig updated watcher config without auth reload (authDirsChanged=%t current=%s)",
		authDirChanged,
		currentAuthDir,
	)
}

func (w *Watcher) switchAuthDirs(nextAuthDirs []string) {
	if w == nil {
		return
	}
	nextAuthDirs = uniqueAuthDirs(nextAuthDirs)
	if len(nextAuthDirs) == 0 {
		return
	}

	w.clientsMutex.Lock()
	prevAuthDir := w.authDir
	prevAuthDirs := append([]string(nil), w.authDirs...)
	if authDirSetsEqual(prevAuthDirs, nextAuthDirs) {
		w.authDirs = nextAuthDirs
		w.authDir = nextAuthDirs[0]
		w.clientsMutex.Unlock()
		return
	}
	w.authDirs = nextAuthDirs
	w.authDir = nextAuthDirs[0]
	watcher := w.watcher
	w.clientsMutex.Unlock()

	if watcher == nil {
		return
	}
	for _, oldDir := range prevAuthDirs {
		if strings.TrimSpace(oldDir) == "" || containsAuthDir(nextAuthDirs, oldDir) {
			continue
		}
		if err := watcher.Remove(oldDir); err != nil {
			log.WithError(err).Debugf("failed to remove previous auth directory watcher: %s", oldDir)
		}
	}
	for _, nextDir := range nextAuthDirs {
		if containsAuthDir(prevAuthDirs, nextDir) {
			continue
		}
		if err := watcher.Add(nextDir); err != nil {
			log.Errorf("failed to watch switched auth directory %s: %v", nextDir, err)
			return
		}
	}
	log.Infof("auth directory watcher switched: %s -> %s", prevAuthDir, strings.Join(nextAuthDirs, ", "))
}

func (w *Watcher) switchAuthDir(nextAuthDir string) {
	w.switchAuthDirs([]string{nextAuthDir})
}

// SetAuthUpdateQueue sets the queue used to emit auth updates.
func (w *Watcher) SetAuthUpdateQueue(queue chan<- AuthUpdate) {
	w.setAuthUpdateQueue(queue)
}

// DispatchRuntimeAuthUpdate allows external runtime providers (e.g., websocket-driven auths)
// to push auth updates through the same queue used by file/config watchers.
// Returns true if the update was enqueued; false if no queue is configured.
func (w *Watcher) DispatchRuntimeAuthUpdate(update AuthUpdate) bool {
	return w.dispatchRuntimeAuthUpdate(update)
}

// SnapshotCoreAuths converts current clients snapshot into core auth entries.
func (w *Watcher) SnapshotCoreAuths() []*coreauth.Auth {
	w.clientsMutex.RLock()
	cfg := w.config
	authDirs := append([]string(nil), w.authDirs...)
	w.clientsMutex.RUnlock()
	return snapshotCoreAuths(cfg, authDirs)
}

func configAuthDirs(cfg *config.Config, fallbackAuthDir string) []string {
	if cfg == nil {
		return uniqueAuthDirs([]string{fallbackAuthDir})
	}

	authDirs := cfg.RuntimeAuthPoolPaths()
	if len(authDirs) == 0 {
		authDirs = []string{cfg.AuthDir}
	}
	if len(authDirs) == 0 {
		authDirs = []string{fallbackAuthDir}
	}
	return uniqueAuthDirs(authDirs)
}

func uniqueAuthDirs(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized := pathutil.NormalizePath(path)
		if normalized == "" {
			continue
		}
		key := pathutil.NormalizeCompareKey(normalized)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func containsAuthDir(paths []string, target string) bool {
	targetKey := pathutil.NormalizeCompareKey(target)
	if targetKey == "" {
		return false
	}
	for _, path := range paths {
		if pathutil.NormalizeCompareKey(path) == targetKey {
			return true
		}
	}
	return false
}

func authDirSetsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if !pathutil.PathsEqual(left[idx], right[idx]) {
			return false
		}
	}
	return true
}
