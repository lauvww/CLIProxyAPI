package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"golang.org/x/net/context"
)

func TestRequestExecutionMetadataIncludesExplicitIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Idempotency-Key", "idem-123")
	ginCtx.Request = req

	ctx := context.Background()
	ctx = WithPinnedAuthID(ctx, "auth-1")
	ctx = WithExecutionSessionID(ctx, "session-1")
	selectedCalled := false
	ctx = WithSelectedAuthIDCallback(ctx, func(selected string) {
		selectedCalled = selected == "selected-auth"
	})
	ctx = context.WithValue(ctx, "gin", ginCtx)

	meta := requestExecutionMetadata(ctx)

	if got, ok := meta[idempotencyKeyMetadataKey].(string); !ok || got != "idem-123" {
		t.Fatalf("idempotency key = %#v, want %q", meta[idempotencyKeyMetadataKey], "idem-123")
	}
	if got, ok := meta[coreexecutor.PinnedAuthMetadataKey].(string); !ok || got != "auth-1" {
		t.Fatalf("pinned auth metadata = %#v, want %q", meta[coreexecutor.PinnedAuthMetadataKey], "auth-1")
	}
	if got, ok := meta[coreexecutor.ExecutionSessionMetadataKey].(string); !ok || got != "session-1" {
		t.Fatalf("execution session metadata = %#v, want %q", meta[coreexecutor.ExecutionSessionMetadataKey], "session-1")
	}

	callback, ok := meta[coreexecutor.SelectedAuthCallbackMetadataKey].(func(string))
	if !ok || callback == nil {
		t.Fatalf("selected auth callback missing or invalid: %#v", meta[coreexecutor.SelectedAuthCallbackMetadataKey])
	}
	callback("selected-auth")
	if !selectedCalled {
		t.Fatalf("selected auth callback was not preserved")
	}
}

func TestRequestExecutionMetadataOmitsIdempotencyKeyWhenHeaderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	ctx := context.Background()
	ctx = WithExecutionSessionID(ctx, "session-2")
	ctx = context.WithValue(ctx, "gin", ginCtx)

	meta := requestExecutionMetadata(ctx)

	if _, exists := meta[idempotencyKeyMetadataKey]; exists {
		t.Fatalf("idempotency key should be omitted when request header is absent: %#v", meta)
	}
	if got, ok := meta[coreexecutor.ExecutionSessionMetadataKey].(string); !ok || got != "session-2" {
		t.Fatalf("execution session metadata = %#v, want %q", meta[coreexecutor.ExecutionSessionMetadataKey], "session-2")
	}
}

func TestRequestHeadersFromContextClonesIncomingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("X-Session-ID", "session-header")
	ginCtx.Request = req

	ctx := context.WithValue(context.Background(), "gin", ginCtx)
	headers := requestHeadersFromContext(ctx)
	if headers == nil {
		t.Fatalf("headers = nil, want cloned request headers")
	}
	if got := headers.Get("X-Session-ID"); got != "session-header" {
		t.Fatalf("X-Session-ID = %q, want %q", got, "session-header")
	}

	headers.Set("X-Session-ID", "mutated")
	if got := ginCtx.Request.Header.Get("X-Session-ID"); got != "session-header" {
		t.Fatalf("original request headers mutated to %q", got)
	}
}
