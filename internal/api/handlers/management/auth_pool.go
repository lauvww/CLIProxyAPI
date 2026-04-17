package management

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/managementasset"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/pathutil"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

func (h *Handler) authPoolStatePayload() gin.H {
	if h == nil || h.cfg == nil {
		return gin.H{
			"enabled":                  false,
			"paths":                    []string{},
			"active-path":              "",
			"active_path":              "",
			"auth-dir":                 "",
			"auth_dir":                 "",
			"routing-strategy-by-path": map[string]string{},
			"routing_strategy_by_path": map[string]string{},
			"current-strategy":         "",
			"current_strategy":         "",
		}
	}

	h.cfg.NormalizeAuthPool()

	strategyByPath := make(map[string]string, len(h.cfg.AuthPool.RoutingStrategyByPath))
	for path, strategy := range h.cfg.AuthPool.RoutingStrategyByPath {
		strategyByPath[path] = strategy
	}

	currentStrategy := strings.TrimSpace(h.cfg.Routing.Strategy)
	return gin.H{
		"enabled":                  h.cfg.AuthPool.Enabled,
		"paths":                    append([]string(nil), h.cfg.AuthPool.Paths...),
		"active-path":              h.cfg.AuthPool.ActivePath,
		"active_path":              h.cfg.AuthPool.ActivePath,
		"auth-dir":                 h.cfg.AuthDir,
		"auth_dir":                 h.cfg.AuthDir,
		"routing-strategy-by-path": strategyByPath,
		"routing_strategy_by_path": strategyByPath,
		"current-strategy":         currentStrategy,
		"current_strategy":         currentStrategy,
	}
}

func (h *Handler) respondWithAuthPoolState(c *gin.Context) {
	c.JSON(http.StatusOK, h.authPoolStatePayload())
}

func (h *Handler) persistAuthPoolState(c *gin.Context) bool {
	if h != nil && h.cfg != nil {
		h.cfg.NormalizeAuthPool()
		managementasset.SetCurrentConfig(h.cfg)
	}
	if err := h.saveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config", "message": err.Error()})
		return false
	}
	h.respondWithAuthPoolState(c)
	return true
}

// GetAuthPool returns configured auth pool settings.
func (h *Handler) GetAuthPool(c *gin.Context) {
	h.respondWithAuthPoolState(c)
}

// PutAuthPoolEnabled toggles auth pool mode.
func (h *Handler) PutAuthPoolEnabled(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config unavailable"})
		return
	}
	enabled, ok := readAuthPoolEnabledFromBody(c)
	if !ok {
		return
	}
	if enabled {
		active := strings.TrimSpace(h.cfg.CurrentAuthPoolPath())
		if active != "" {
			if errMkdir := os.MkdirAll(active, 0o755); errMkdir != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create auth pool directory"})
				return
			}
		}
	}
	h.cfg.SetAuthPoolEnabled(enabled)
	h.persistAuthPoolState(c)
}

// AddAuthPoolPath appends an auth pool path to config.auth-pool.paths.
func (h *Handler) AddAuthPoolPath(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config unavailable"})
		return
	}
	path, ok := readAuthPoolPathFromBody(c)
	if !ok {
		return
	}
	resolved, err := resolveAndNormalizeAuthPoolPath(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errMkdir := os.MkdirAll(resolved, 0o755); errMkdir != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create auth pool directory"})
		return
	}

	if !containsAuthPoolPath(h.cfg.AuthPool.Paths, resolved) {
		h.cfg.AuthPool.Paths = append(h.cfg.AuthPool.Paths, resolved)
	}
	if strings.TrimSpace(h.cfg.AuthPool.ActivePath) == "" {
		h.cfg.SetCurrentAuthPoolPath(resolved)
	} else {
		h.cfg.NormalizeAuthPool()
	}
	h.persistAuthPoolState(c)
}

// PutCurrentAuthPoolPath switches the active auth pool and syncs auth-dir.
func (h *Handler) PutCurrentAuthPoolPath(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config unavailable"})
		return
	}
	path, ok := readAuthPoolPathFromBody(c)
	if !ok {
		return
	}
	resolved, err := resolveAndNormalizeAuthPoolPath(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errMkdir := os.MkdirAll(resolved, 0o755); errMkdir != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create auth pool directory"})
		return
	}

	h.cfg.SetCurrentAuthPoolPath(resolved)
	h.persistAuthPoolState(c)
}

// PutAuthPoolStrategy updates the routing strategy of the selected auth pool path.
func (h *Handler) PutAuthPoolStrategy(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config unavailable"})
		return
	}
	path, strategy, ok := readAuthPoolStrategyFromBody(c)
	if !ok {
		return
	}
	if strings.TrimSpace(path) == "" {
		path = h.cfg.CurrentAuthPoolPath()
	}
	resolved, err := resolveAndNormalizeAuthPoolPath(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !containsAuthPoolPath(h.cfg.AuthPool.Paths, resolved) {
		h.cfg.AuthPool.Paths = append(h.cfg.AuthPool.Paths, resolved)
	}
	if !h.cfg.SetAuthPoolRoutingStrategy(resolved, strategy) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy"})
		return
	}
	h.cfg.NormalizeAuthPool()
	h.persistAuthPoolState(c)
}

// DeleteAuthPoolPath removes one auth pool path from config.auth-pool.paths.
func (h *Handler) DeleteAuthPoolPath(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config unavailable"})
		return
	}

	target := strings.TrimSpace(c.Query("path"))
	if target == "" {
		value, ok := readAuthPoolPathFromBody(c)
		if !ok {
			return
		}
		target = value
	}
	resolved, err := resolveAndNormalizeAuthPoolPath(target)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	active := strings.TrimSpace(h.cfg.CurrentAuthPoolPath())
	if authPoolPathKey(active) == authPoolPathKey(resolved) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove active auth pool path; switch to another pool first"})
		return
	}

	updated := make([]string, 0, len(h.cfg.AuthPool.Paths))
	for _, item := range h.cfg.AuthPool.Paths {
		if pathutil.PathsEqual(item, resolved) {
			continue
		}
		updated = append(updated, item)
	}
	if len(updated) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one auth pool path must remain"})
		return
	}
	h.cfg.AuthPool.Paths = updated
	if h.cfg.AuthPool.RoutingStrategyByPath != nil {
		for existingPath := range h.cfg.AuthPool.RoutingStrategyByPath {
			if pathutil.PathsEqual(existingPath, resolved) {
				delete(h.cfg.AuthPool.RoutingStrategyByPath, existingPath)
			}
		}
	}
	h.cfg.NormalizeAuthPool()
	h.persistAuthPoolState(c)
}

func readAuthPoolEnabledFromBody(c *gin.Context) (bool, bool) {
	var body struct {
		Enabled *bool `json:"enabled"`
		Value   *bool `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return false, false
	}
	if body.Enabled != nil {
		return *body.Enabled, true
	}
	if body.Value != nil {
		return *body.Value, true
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "enabled is required"})
	return false, false
}

func readAuthPoolPathFromBody(c *gin.Context) (string, bool) {
	var body struct {
		Path  *string `json:"path"`
		Value *string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return "", false
	}
	var raw string
	if body.Path != nil {
		raw = *body.Path
	} else if body.Value != nil {
		raw = *body.Value
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return "", false
	}
	return raw, true
}

func readAuthPoolStrategyFromBody(c *gin.Context) (string, string, bool) {
	var body struct {
		Path     *string `json:"path"`
		Strategy *string `json:"strategy"`
		Value    *string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return "", "", false
	}
	path := ""
	if body.Path != nil {
		path = strings.TrimSpace(*body.Path)
	}
	strategy := ""
	if body.Strategy != nil {
		strategy = strings.TrimSpace(*body.Strategy)
	} else if body.Value != nil {
		strategy = strings.TrimSpace(*body.Value)
	}
	if strategy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy is required"})
		return "", "", false
	}
	return path, strategy, true
}

func resolveAndNormalizeAuthPoolPath(path string) (string, error) {
	resolved, err := util.ResolveAuthDir(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	if resolved == "" {
		return "", nil
	}
	if !filepath.IsAbs(resolved) {
		if abs, errAbs := filepath.Abs(resolved); errAbs == nil {
			resolved = abs
		}
	}
	return pathutil.NormalizePath(resolved), nil
}

func containsAuthPoolPath(paths []string, target string) bool {
	for _, item := range paths {
		if pathutil.PathsEqual(item, target) {
			return true
		}
	}
	return false
}

func authPoolPathKey(path string) string {
	return pathutil.NormalizeCompareKey(path)
}

func authPoolPathMatches(path string, target string) bool {
	return pathutil.PathsEqual(path, target)
}
