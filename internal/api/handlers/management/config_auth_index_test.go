package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestGetGeminiKeysIncludesAuthIndexWithStableMapping(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		GeminiKey: []config.GeminiKey{
			{APIKey: "shared-key", BaseURL: "https://a.example.com"},
			{APIKey: "shared-key", BaseURL: "https://b.example.com"},
		},
	}

	idGen := synthesizer.NewStableIDGenerator()
	idA, _ := idGen.Next("gemini:apikey", "shared-key", "https://a.example.com")
	idB, _ := idGen.Next("gemini:apikey", "shared-key", "https://b.example.com")

	manager := coreauth.NewManager(nil, nil, nil)
	authB, errRegisterB := manager.Register(context.Background(), &coreauth.Auth{ID: idB, Provider: "gemini", Status: coreauth.StatusActive})
	if errRegisterB != nil {
		t.Fatalf("register auth B: %v", errRegisterB)
	}
	authA, errRegisterA := manager.Register(context.Background(), &coreauth.Auth{ID: idA, Provider: "gemini", Status: coreauth.StatusActive})
	if errRegisterA != nil {
		t.Fatalf("register auth A: %v", errRegisterA)
	}

	h := NewHandlerWithoutConfigFilePath(cfg, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/gemini-api-key", nil)

	h.GetGeminiKeys(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"gemini-api-key"`
	}
	if errUnmarshal := json.Unmarshal(recorder.Body.Bytes(), &payload); errUnmarshal != nil {
		t.Fatalf("unmarshal response: %v", errUnmarshal)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(payload.Items))
	}

	byBaseURL := make(map[string]string, len(payload.Items))
	for _, item := range payload.Items {
		byBaseURL[jsonStringValue(item["base-url"])] = jsonStringValue(item["auth-index"])
	}

	if got := byBaseURL["https://a.example.com"]; got != authA.Index {
		t.Fatalf("auth-index for base-url A = %q, want %q", got, authA.Index)
	}
	if got := byBaseURL["https://b.example.com"]; got != authB.Index {
		t.Fatalf("auth-index for base-url B = %q, want %q", got, authB.Index)
	}
}

func TestGetCodexKeysOmitsAuthIndexWhenLiveAuthMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandlerWithoutConfigFilePath(&config.Config{
		CodexKey: []config.CodexKey{
			{APIKey: "codex-key", BaseURL: "https://codex.example.com"},
		},
	}, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/codex-api-key", nil)

	h.GetCodexKeys(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"codex-api-key"`
	}
	if errUnmarshal := json.Unmarshal(recorder.Body.Bytes(), &payload); errUnmarshal != nil {
		t.Fatalf("unmarshal response: %v", errUnmarshal)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(payload.Items))
	}
	if _, exists := payload.Items[0]["auth-index"]; exists {
		t.Fatalf("auth-index should be omitted when live auth mapping is unavailable: %#v", payload.Items[0])
	}
}

func TestGetOpenAICompatAssignsNestedAndTopLevelAuthIndex(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name:    "RouterX",
				BaseURL: "https://routerx.example.com/v1",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "key-a", ProxyURL: "socks5://proxy-a"},
					{APIKey: "key-b"},
				},
			},
			{
				Name:    "NoKeyProvider",
				BaseURL: "https://nokey.example.com/v1",
			},
		},
	}

	idGen := synthesizer.NewStableIDGenerator()
	routerKind := fmt.Sprintf("openai-compatibility:%s", "routerx")
	noKeyKind := fmt.Sprintf("openai-compatibility:%s", "nokeyprovider")

	routerAID, _ := idGen.Next(routerKind, "key-a", "https://routerx.example.com/v1", "socks5://proxy-a")
	routerBID, _ := idGen.Next(routerKind, "key-b", "https://routerx.example.com/v1", "")
	noKeyID, _ := idGen.Next(noKeyKind, "https://nokey.example.com/v1")

	manager := coreauth.NewManager(nil, nil, nil)
	noKeyAuth, errRegisterNoKey := manager.Register(context.Background(), &coreauth.Auth{ID: noKeyID, Provider: "nokeyprovider", Status: coreauth.StatusActive})
	if errRegisterNoKey != nil {
		t.Fatalf("register no-key auth: %v", errRegisterNoKey)
	}
	routerBAuth, errRegisterRouterB := manager.Register(context.Background(), &coreauth.Auth{ID: routerBID, Provider: "routerx", Status: coreauth.StatusActive})
	if errRegisterRouterB != nil {
		t.Fatalf("register router B auth: %v", errRegisterRouterB)
	}
	routerAAuth, errRegisterRouterA := manager.Register(context.Background(), &coreauth.Auth{ID: routerAID, Provider: "routerx", Status: coreauth.StatusActive})
	if errRegisterRouterA != nil {
		t.Fatalf("register router A auth: %v", errRegisterRouterA)
	}

	h := NewHandlerWithoutConfigFilePath(cfg, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/openai-compatibility", nil)

	h.GetOpenAICompat(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"openai-compatibility"`
	}
	if errUnmarshal := json.Unmarshal(recorder.Body.Bytes(), &payload); errUnmarshal != nil {
		t.Fatalf("unmarshal response: %v", errUnmarshal)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(payload.Items))
	}

	var routerItem map[string]any
	var noKeyItem map[string]any
	for _, item := range payload.Items {
		switch jsonStringValue(item["name"]) {
		case "RouterX":
			routerItem = item
		case "NoKeyProvider":
			noKeyItem = item
		}
	}

	if routerItem == nil {
		t.Fatalf("RouterX item missing from response: %#v", payload.Items)
	}
	if noKeyItem == nil {
		t.Fatalf("NoKeyProvider item missing from response: %#v", payload.Items)
	}

	if _, exists := routerItem["auth-index"]; exists {
		t.Fatalf("provider-level auth-index should be omitted when nested api-key entries exist: %#v", routerItem)
	}

	nestedEntries, ok := routerItem["api-key-entries"].([]any)
	if !ok || len(nestedEntries) != 2 {
		t.Fatalf("nested api-key entries invalid: %#v", routerItem["api-key-entries"])
	}

	byAPIKey := make(map[string]string, len(nestedEntries))
	for _, rawEntry := range nestedEntries {
		entry, okEntry := rawEntry.(map[string]any)
		if !okEntry {
			t.Fatalf("nested entry has unexpected shape: %#v", rawEntry)
		}
		byAPIKey[jsonStringValue(entry["api-key"])] = jsonStringValue(entry["auth-index"])
	}

	if got := byAPIKey["key-a"]; got != routerAAuth.Index {
		t.Fatalf("RouterX key-a auth-index = %q, want %q", got, routerAAuth.Index)
	}
	if got := byAPIKey["key-b"]; got != routerBAuth.Index {
		t.Fatalf("RouterX key-b auth-index = %q, want %q", got, routerBAuth.Index)
	}
	if got := jsonStringValue(noKeyItem["auth-index"]); got != noKeyAuth.Index {
		t.Fatalf("NoKeyProvider auth-index = %q, want %q", got, noKeyAuth.Index)
	}
}

func jsonStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
