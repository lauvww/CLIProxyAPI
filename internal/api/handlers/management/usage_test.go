package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestGetUsageStatisticsWithPoolFilter(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	stats.MergeSnapshot(usage.StatisticsSnapshot{
		APIs: map[string]usage.APISnapshot{
			"POST /v1/responses": {
				Models: map[string]usage.ModelSnapshot{
					"gpt-5.4": {
						Details: []usage.RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								PoolType:  usage.PoolTypePlus,
								Tokens:    usage.TokenStats{TotalTokens: 10},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								PoolType:  usage.PoolTypeFree,
								Tokens:    usage.TokenStats{TotalTokens: 20},
							},
						},
					},
				},
			},
		},
	})

	handler := &Handler{usageStats: stats}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/usage?pool=plus", nil)

	handler.GetUsageStatistics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if applied, ok := payload["pool_filter_applied"].(bool); !ok || !applied {
		t.Fatalf("pool_filter_applied = %v, want true", payload["pool_filter_applied"])
	}
	if pool, ok := payload["pool_filter"].(string); !ok || pool != usage.PoolTypePlus {
		t.Fatalf("pool_filter = %v, want %q", payload["pool_filter"], usage.PoolTypePlus)
	}

	usagePayload, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage payload missing or invalid: %v", payload["usage"])
	}
	if got := intFromAny(usagePayload["total_requests"]); got != 1 {
		t.Fatalf("usage.total_requests = %d, want 1", got)
	}
}

func TestGetUsageStatisticsByPoolAggregation(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	stats.MergeSnapshot(usage.StatisticsSnapshot{
		APIs: map[string]usage.APISnapshot{
			"POST /v1/responses": {
				Models: map[string]usage.ModelSnapshot{
					"gpt-5.4": {
						Details: []usage.RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								PoolType:  usage.PoolTypePlus,
								Tokens:    usage.TokenStats{TotalTokens: 10},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								PoolType:  usage.PoolTypeCustom,
								Tokens:    usage.TokenStats{TotalTokens: 20},
							},
						},
					},
				},
			},
		},
	})

	handler := &Handler{usageStats: stats}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/usage?by_pool=1", nil)

	handler.GetUsageStatistics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	byPool, ok := payload["by_pool"].(map[string]any)
	if !ok {
		t.Fatalf("by_pool missing or invalid: %v", payload["by_pool"])
	}
	if _, ok := byPool[usage.PoolTypePlus]; !ok {
		t.Fatalf("by_pool missing %q", usage.PoolTypePlus)
	}
	if _, ok := byPool[usage.PoolTypeFree]; !ok {
		t.Fatalf("by_pool missing %q", usage.PoolTypeFree)
	}
	if _, ok := byPool[usage.PoolTypeCustom]; !ok {
		t.Fatalf("by_pool missing %q", usage.PoolTypeCustom)
	}
}

func TestGetUsageStatisticsRejectsInvalidPoolFilter(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	handler := &Handler{usageStats: usage.NewRequestStatistics()}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/usage?pool=invalid", nil)

	handler.GetUsageStatistics(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetUsageStatisticsDefaultsToCurrentAuthPool(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	stats.MergeSnapshot(usage.StatisticsSnapshot{
		APIs: map[string]usage.APISnapshot{
			"POST /v1/responses": {
				Models: map[string]usage.ModelSnapshot{
					"gpt-5.4": {
						Details: []usage.RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								AuthPool:  "D:/auth/pool-a",
								PoolType:  usage.PoolTypePlus,
								Tokens:    usage.TokenStats{TotalTokens: 10},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								AuthPool:  "D:/auth/pool-b",
								PoolType:  usage.PoolTypePlus,
								Tokens:    usage.TokenStats{TotalTokens: 20},
							},
						},
					},
				},
			},
		},
	})

	handler := &Handler{
		cfg: &config.Config{
			AuthDir: "D:/auth/pool-a",
			AuthPool: config.AuthPoolConfig{
				Enabled:    true,
				Paths:      []string{"D:/auth/pool-a", "D:/auth/pool-b"},
				ActivePath: "D:/auth/pool-a",
			},
		},
		usageStats: stats,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/usage", nil)

	handler.GetUsageStatistics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if applied, ok := payload["auth_pool_filter_applied"].(bool); !ok || !applied {
		t.Fatalf("auth_pool_filter_applied = %v, want true", payload["auth_pool_filter_applied"])
	}
	if defaulted, ok := payload["auth_pool_filter_defaulted"].(bool); !ok || !defaulted {
		t.Fatalf("auth_pool_filter_defaulted = %v, want true", payload["auth_pool_filter_defaulted"])
	}
	if hint, ok := payload["usage_scope_hint"].(string); !ok || hint == "" {
		t.Fatalf("usage_scope_hint = %v, want non-empty hint", payload["usage_scope_hint"])
	}

	usagePayload, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage payload missing or invalid: %v", payload["usage"])
	}
	if got := intFromAny(usagePayload["total_requests"]); got != 1 {
		t.Fatalf("usage.total_requests = %d, want 1", got)
	}
}

func TestGetUsageStatisticsDoesNotDefaultToAuthPoolWhenDisabled(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	stats.MergeSnapshot(usage.StatisticsSnapshot{
		APIs: map[string]usage.APISnapshot{
			"POST /v1/responses": {
				Models: map[string]usage.ModelSnapshot{
					"gpt-5.4": {
						Details: []usage.RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								AuthPool:  "D:/auth/pool-a",
								PoolType:  usage.PoolTypePlus,
								Tokens:    usage.TokenStats{TotalTokens: 10},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								AuthPool:  "D:/auth/pool-b",
								PoolType:  usage.PoolTypePlus,
								Tokens:    usage.TokenStats{TotalTokens: 20},
							},
						},
					},
				},
			},
		},
	})

	handler := &Handler{
		cfg: &config.Config{
			AuthDir: "D:/auth/pool-a",
			AuthPool: config.AuthPoolConfig{
				Enabled:    false,
				Paths:      []string{"D:/auth/pool-a", "D:/auth/pool-b"},
				ActivePath: "D:/auth/pool-a",
			},
		},
		usageStats: stats,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/usage", nil)

	handler.GetUsageStatistics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if applied, ok := payload["auth_pool_filter_applied"].(bool); !ok || applied {
		t.Fatalf("auth_pool_filter_applied = %v, want false", payload["auth_pool_filter_applied"])
	}
	if defaulted, ok := payload["auth_pool_filter_defaulted"].(bool); !ok || defaulted {
		t.Fatalf("auth_pool_filter_defaulted = %v, want false", payload["auth_pool_filter_defaulted"])
	}

	usagePayload, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage payload missing or invalid: %v", payload["usage"])
	}
	if got := intFromAny(usagePayload["total_requests"]); got != 2 {
		t.Fatalf("usage.total_requests = %d, want 2", got)
	}
}

func TestGetUsageStatisticsAppliesAPIKeyAliases(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	usage.SetAPIKeyAliases(map[string]string{
		"client-key-1": "Desktop",
	})
	t.Cleanup(func() {
		usage.SetAPIKeyAliases(nil)
	})

	stats := usage.NewRequestStatistics()
	stats.Record(nil, coreusage.Record{
		APIKey:      "client-key-1",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	})

	handler := &Handler{usageStats: stats}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/usage", nil)

	handler.GetUsageStatistics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	usagePayload, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage payload missing or invalid: %v", payload["usage"])
	}
	apis, ok := usagePayload["apis"].(map[string]any)
	if !ok {
		t.Fatalf("usage.apis missing or invalid: %v", usagePayload["apis"])
	}
	if _, ok := apis["Desktop"]; !ok {
		t.Fatalf("expected aliased bucket Desktop, got %#v", apis)
	}
	if _, ok := apis["client-key-1"]; ok {
		t.Fatalf("did not expect raw bucket after alias mapping, got %#v", apis)
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}
