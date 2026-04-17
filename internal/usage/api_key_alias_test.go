package usage

import (
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestRequestStatisticsUsesAPIKeyAliasForAPIBucket(t *testing.T) {
	stats := NewRequestStatistics()
	SetAPIKeyAliases(map[string]string{
		"client-key-1": "Primary Desktop",
	})
	t.Cleanup(func() {
		SetAPIKeyAliases(nil)
	})

	stats.Record(nil, coreusage.Record{
		APIKey:      "client-key-1",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	})

	snapshot := stats.Snapshot()
	if _, ok := snapshot.APIs["Primary Desktop"]; !ok {
		t.Fatalf("expected aliased API bucket to exist, got %#v", snapshot.APIs)
	}
	if _, ok := snapshot.APIs["client-key-1"]; ok {
		t.Fatalf("expected raw API key bucket to be hidden when alias exists, got %#v", snapshot.APIs)
	}
}
