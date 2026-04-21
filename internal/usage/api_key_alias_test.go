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
	if _, ok := snapshot.APIs["client-key-1"]; !ok {
		t.Fatalf("expected raw API bucket to exist before alias mapping, got %#v", snapshot.APIs)
	}

	displaySnapshot := ApplyAPIKeyAliasesToSnapshot(snapshot)
	if _, ok := displaySnapshot.APIs["Primary Desktop"]; !ok {
		t.Fatalf("expected aliased API bucket to exist after alias mapping, got %#v", displaySnapshot.APIs)
	}
	if _, ok := displaySnapshot.APIs["client-key-1"]; ok {
		t.Fatalf("expected raw API key bucket to be hidden after alias mapping, got %#v", displaySnapshot.APIs)
	}
}

func TestApplyAPIKeyAliasesToSnapshotMergesBuckets(t *testing.T) {
	SetAPIKeyAliases(map[string]string{
		"client-key-1": "Desktop",
		"client-key-2": "Desktop",
	})
	t.Cleanup(func() {
		SetAPIKeyAliases(nil)
	})

	snapshot := StatisticsSnapshot{
		TotalRequests: 3,
		SuccessCount:  3,
		TotalTokens:   30,
		APIs: map[string]APISnapshot{
			"client-key-1": {
				TotalRequests: 1,
				TotalTokens:   10,
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						TotalRequests: 1,
						TotalTokens:   10,
						Details: []RequestDetail{{
							Timestamp: time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC),
							Tokens:    TokenStats{TotalTokens: 10},
						}},
					},
				},
			},
			"client-key-2": {
				TotalRequests: 2,
				TotalTokens:   20,
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						TotalRequests: 2,
						TotalTokens:   20,
						Details: []RequestDetail{{
							Timestamp: time.Date(2026, 4, 20, 2, 0, 0, 0, time.UTC),
							Tokens:    TokenStats{TotalTokens: 20},
						}},
					},
				},
			},
		},
	}

	displaySnapshot := ApplyAPIKeyAliasesToSnapshot(snapshot)
	if len(displaySnapshot.APIs) != 1 {
		t.Fatalf("expected merged alias bucket count = 1, got %d", len(displaySnapshot.APIs))
	}

	apiBucket, ok := displaySnapshot.APIs["Desktop"]
	if !ok {
		t.Fatalf("expected merged alias bucket to exist, got %#v", displaySnapshot.APIs)
	}
	if apiBucket.TotalRequests != 3 {
		t.Fatalf("merged total_requests = %d, want 3", apiBucket.TotalRequests)
	}
	if apiBucket.TotalTokens != 30 {
		t.Fatalf("merged total_tokens = %d, want 30", apiBucket.TotalTokens)
	}

	modelBucket := apiBucket.Models["gpt-5.4"]
	if modelBucket.TotalRequests != 3 {
		t.Fatalf("merged model total_requests = %d, want 3", modelBucket.TotalRequests)
	}
	if len(modelBucket.Details) != 2 {
		t.Fatalf("merged details len = %d, want 2", len(modelBucket.Details))
	}
}
