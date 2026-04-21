package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

type usageExportPayload struct {
	Version    int                      `json:"version"`
	ExportedAt time.Time                `json:"exported_at"`
	Usage      usage.StatisticsSnapshot `json:"usage"`
}

type usageImportPayload struct {
	Version int                      `json:"version"`
	Usage   usage.StatisticsSnapshot `json:"usage"`
}

// GetUsageStatistics returns the in-memory request statistics snapshot.
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	poolFilter := strings.ToLower(strings.TrimSpace(c.Query("pool")))
	if poolFilter == "" {
		poolFilter = "all"
	}
	if poolFilter != "all" &&
		poolFilter != usage.PoolTypePlus &&
		poolFilter != usage.PoolTypeFree &&
		poolFilter != usage.PoolTypeCustom {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool filter"})
		return
	}

	includeByPool := parseUsageBooleanQuery(c.Query("by_pool"))
	requestedAuthPoolFilter := strings.TrimSpace(c.Query("auth_pool"))
	currentAuthPool := ""
	authPoolEnabled := false
	if h != nil && h.cfg != nil {
		currentAuthPool = strings.TrimSpace(h.cfg.CurrentAuthPoolPath())
		authPoolEnabled = h.cfg.AuthPool.Enabled
	}

	authPoolFilter := requestedAuthPoolFilter
	authPoolFilterDefaulted := false
	if authPoolFilter == "" && authPoolEnabled && currentAuthPool != "" {
		authPoolFilter = currentAuthPool
		authPoolFilterDefaulted = true
	}
	authPoolFilterApplied := authPoolFilter != "" && !strings.EqualFold(authPoolFilter, "all")

	var fullSnapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		fullSnapshot = h.usageStats.Snapshot()
	}

	snapshot := fullSnapshot
	if authPoolFilterApplied {
		snapshot = usage.FilterSnapshotByAuthPool(snapshot, authPoolFilter)
	}
	poolFilterApplied := false
	if poolFilter != "all" {
		snapshot = usage.FilterSnapshotByPool(snapshot, poolFilter)
		poolFilterApplied = true
	}
	displaySnapshot := usage.ApplyAPIKeyAliasesToSnapshot(snapshot)

	response := gin.H{
		"usage":                      displaySnapshot,
		"failed_requests":            displaySnapshot.FailureCount,
		"pool_filter":                poolFilter,
		"pool_filter_applied":        poolFilterApplied,
		"auth_pool_filter":           authPoolFilter,
		"auth_pool_filter_applied":   authPoolFilterApplied,
		"auth_pool_filter_defaulted": authPoolFilterDefaulted,
		"current_auth_pool":          currentAuthPool,
		"auth_pool_enabled":          authPoolEnabled,
	}
	if authPoolFilterDefaulted {
		response["usage_scope_hint"] = fmt.Sprintf(
			"Showing usage for the current auth pool by default (%s). Set auth_pool=all to view all auth pools.",
			currentAuthPool,
		)
	} else if authPoolEnabled && authPoolFilterApplied {
		response["usage_scope_hint"] = fmt.Sprintf(
			"Showing usage for auth pool %s. Set auth_pool=all to view all auth pools.",
			authPoolFilter,
		)
	} else if strings.EqualFold(authPoolFilter, "all") {
		response["usage_scope_hint"] = "Showing usage for all auth pools."
	}
	if includeByPool {
		aggregated := usage.AggregateSnapshotByPool(fullSnapshot)
		displayAggregated := make(map[string]usage.StatisticsSnapshot, len(aggregated))
		for poolName, poolSnapshot := range aggregated {
			displayAggregated[poolName] = usage.ApplyAPIKeyAliasesToSnapshot(poolSnapshot)
		}
		response["by_pool"] = displayAggregated
	}

	c.JSON(http.StatusOK, response)
}

func parseUsageBooleanQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// ExportUsageStatistics returns a complete usage snapshot for backup/migration.
func (h *Handler) ExportUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, usageExportPayload{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Usage:      snapshot,
	})
}

// ImportUsageStatistics merges a previously exported usage snapshot into memory.
func (h *Handler) ImportUsageStatistics(c *gin.Context) {
	if h == nil || h.usageStats == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usage statistics unavailable"})
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var payload usageImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.Version != 0 && payload.Version != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported version"})
		return
	}

	result := h.usageStats.MergeSnapshot(payload.Usage)
	snapshot := h.usageStats.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"added":           result.Added,
		"skipped":         result.Skipped,
		"total_requests":  snapshot.TotalRequests,
		"failed_requests": snapshot.FailureCount,
	})
}
