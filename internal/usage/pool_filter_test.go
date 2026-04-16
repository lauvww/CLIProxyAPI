package usage

import (
	"testing"
	"time"
)

func TestFilterSnapshotByPool(t *testing.T) {
	snapshot := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"POST /v1/responses": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								PoolType:  PoolTypePlus,
								Tokens: TokenStats{
									TotalTokens: 10,
								},
								Failed: false,
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								PoolType:  PoolTypeFree,
								Tokens: TokenStats{
									TotalTokens: 20,
								},
								Failed: true,
							},
							{
								Timestamp: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
								Tokens: TokenStats{
									TotalTokens: 30,
								},
								Failed: false,
							},
						},
					},
				},
			},
		},
	}

	filtered := FilterSnapshotByPool(snapshot, PoolTypePlus)
	if filtered.TotalRequests != 1 {
		t.Fatalf("filtered.TotalRequests = %d, want 1", filtered.TotalRequests)
	}
	if filtered.SuccessCount != 1 {
		t.Fatalf("filtered.SuccessCount = %d, want 1", filtered.SuccessCount)
	}
	if filtered.FailureCount != 0 {
		t.Fatalf("filtered.FailureCount = %d, want 0", filtered.FailureCount)
	}
	if filtered.TotalTokens != 10 {
		t.Fatalf("filtered.TotalTokens = %d, want 10", filtered.TotalTokens)
	}

	model := filtered.APIs["POST /v1/responses"].Models["gpt-5.4"]
	if len(model.Details) != 1 {
		t.Fatalf("len(model.Details) = %d, want 1", len(model.Details))
	}
	if model.Details[0].PoolType != PoolTypePlus {
		t.Fatalf("model.Details[0].PoolType = %q, want %q", model.Details[0].PoolType, PoolTypePlus)
	}
}

func TestAggregateSnapshotByPool(t *testing.T) {
	snapshot := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"POST /v1/responses": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								PoolType:  PoolTypePlus,
								Tokens: TokenStats{
									TotalTokens: 10,
								},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								PoolType:  PoolTypeFree,
								Tokens: TokenStats{
									TotalTokens: 20,
								},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
								Tokens: TokenStats{
									TotalTokens: 30,
								},
							},
						},
					},
				},
			},
		},
	}

	aggregated := AggregateSnapshotByPool(snapshot)
	if len(aggregated) != 3 {
		t.Fatalf("len(aggregated) = %d, want 3", len(aggregated))
	}

	if aggregated[PoolTypePlus].TotalRequests != 1 || aggregated[PoolTypePlus].TotalTokens != 10 {
		t.Fatalf("plus aggregate = %+v, want requests=1 tokens=10", aggregated[PoolTypePlus])
	}
	if aggregated[PoolTypeFree].TotalRequests != 1 || aggregated[PoolTypeFree].TotalTokens != 20 {
		t.Fatalf("free aggregate = %+v, want requests=1 tokens=20", aggregated[PoolTypeFree])
	}
	if aggregated[PoolTypeCustom].TotalRequests != 1 || aggregated[PoolTypeCustom].TotalTokens != 30 {
		t.Fatalf("custom aggregate = %+v, want requests=1 tokens=30", aggregated[PoolTypeCustom])
	}
}

func TestFilterSnapshotByAuthPool(t *testing.T) {
	snapshot := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"POST /v1/responses": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{
							{
								Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
								AuthPool:  "/opt/pool-a",
								PoolType:  PoolTypePlus,
								Tokens: TokenStats{
									TotalTokens: 10,
								},
							},
							{
								Timestamp: time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
								AuthPool:  "/opt/pool-b",
								PoolType:  PoolTypeFree,
								Tokens: TokenStats{
									TotalTokens: 20,
								},
							},
						},
					},
				},
			},
		},
	}

	filtered := FilterSnapshotByAuthPool(snapshot, "/opt/pool-a")
	if filtered.TotalRequests != 1 {
		t.Fatalf("filtered.TotalRequests = %d, want 1", filtered.TotalRequests)
	}
	if filtered.TotalTokens != 10 {
		t.Fatalf("filtered.TotalTokens = %d, want 10", filtered.TotalTokens)
	}

	model := filtered.APIs["POST /v1/responses"].Models["gpt-5.4"]
	if len(model.Details) != 1 {
		t.Fatalf("len(model.Details) = %d, want 1", len(model.Details))
	}
	if model.Details[0].AuthPool != "/opt/pool-a" {
		t.Fatalf("model.Details[0].AuthPool = %q, want %q", model.Details[0].AuthPool, "/opt/pool-a")
	}
}
