package usage

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/pathutil"
)

const (
	PoolTypePlus   = "plus"
	PoolTypeFree   = "free"
	PoolTypeCustom = "custom"
)

var canonicalPoolTypes = [...]string{PoolTypePlus, PoolTypeFree, PoolTypeCustom}

func normalizePoolType(poolType string) string {
	switch strings.ToLower(strings.TrimSpace(poolType)) {
	case PoolTypePlus:
		return PoolTypePlus
	case PoolTypeFree:
		return PoolTypeFree
	default:
		return PoolTypeCustom
	}
}

func normalizePlanType(planType string) string {
	return strings.ToLower(strings.TrimSpace(planType))
}

func normalizeRequestDetailDimensions(detail RequestDetail) RequestDetail {
	detail.AuthID = strings.TrimSpace(detail.AuthID)
	detail.AuthIndex = strings.TrimSpace(detail.AuthIndex)
	detail.ClientAPIKey = strings.TrimSpace(detail.ClientAPIKey)
	detail.AuthPool = normalizeAuthPool(detail.AuthPool)
	detail.PoolType = normalizePoolType(detail.PoolType)
	detail.PlanType = normalizePlanType(detail.PlanType)
	return detail
}

func normalizeAuthPool(authPool string) string {
	return pathutil.NormalizeCompareKey(authPool)
}

func newEmptySnapshot() StatisticsSnapshot {
	return StatisticsSnapshot{
		APIs:           make(map[string]APISnapshot),
		RequestsByDay:  make(map[string]int64),
		RequestsByHour: make(map[string]int64),
		TokensByDay:    make(map[string]int64),
		TokensByHour:   make(map[string]int64),
	}
}

func appendDetailToSnapshot(snapshot *StatisticsSnapshot, apiName, modelName string, detail RequestDetail) {
	if snapshot == nil {
		return
	}
	if snapshot.APIs == nil {
		snapshot.APIs = make(map[string]APISnapshot)
	}
	if snapshot.RequestsByDay == nil {
		snapshot.RequestsByDay = make(map[string]int64)
	}
	if snapshot.RequestsByHour == nil {
		snapshot.RequestsByHour = make(map[string]int64)
	}
	if snapshot.TokensByDay == nil {
		snapshot.TokensByDay = make(map[string]int64)
	}
	if snapshot.TokensByHour == nil {
		snapshot.TokensByHour = make(map[string]int64)
	}

	detail = normalizeRequestDetailDimensions(detail)
	detail.Tokens = normaliseTokenStats(detail.Tokens)
	if detail.Timestamp.IsZero() {
		return
	}
	detail.Timestamp = detail.Timestamp.UTC()

	modelSnapshot := ModelSnapshot{
		TotalRequests: 1,
		TotalTokens:   detail.Tokens.TotalTokens,
		Details:       []RequestDetail{detail},
	}

	apiSnapshot := snapshot.APIs[apiName]
	if apiSnapshot.Models == nil {
		apiSnapshot.Models = make(map[string]ModelSnapshot)
	}
	existingModel := apiSnapshot.Models[modelName]
	existingModel.TotalRequests += modelSnapshot.TotalRequests
	existingModel.TotalTokens += modelSnapshot.TotalTokens
	existingModel.Details = append(existingModel.Details, modelSnapshot.Details...)
	apiSnapshot.Models[modelName] = existingModel
	apiSnapshot.TotalRequests++
	apiSnapshot.TotalTokens += detail.Tokens.TotalTokens
	snapshot.APIs[apiName] = apiSnapshot

	snapshot.TotalRequests++
	if detail.Failed {
		snapshot.FailureCount++
	} else {
		snapshot.SuccessCount++
	}
	snapshot.TotalTokens += detail.Tokens.TotalTokens

	dayKey := detail.Timestamp.Format("2006-01-02")
	hourKey := formatHour(detail.Timestamp.Hour())
	snapshot.RequestsByDay[dayKey]++
	snapshot.RequestsByHour[hourKey]++
	snapshot.TokensByDay[dayKey] += detail.Tokens.TotalTokens
	snapshot.TokensByHour[hourKey] += detail.Tokens.TotalTokens
}

// FilterSnapshotByPool returns a filtered snapshot containing only records from the pool type.
func FilterSnapshotByPool(snapshot StatisticsSnapshot, poolType string) StatisticsSnapshot {
	targetPoolType := normalizePoolType(poolType)
	filtered := newEmptySnapshot()

	for apiName, apiSnapshot := range snapshot.APIs {
		for modelName, modelSnapshot := range apiSnapshot.Models {
			for _, detail := range modelSnapshot.Details {
				if normalizePoolType(detail.PoolType) != targetPoolType {
					continue
				}
				appendDetailToSnapshot(&filtered, apiName, modelName, detail)
			}
		}
	}

	return filtered
}

// FilterSnapshotByAuthPool returns a filtered snapshot containing only records from the auth pool path.
func FilterSnapshotByAuthPool(snapshot StatisticsSnapshot, authPool string) StatisticsSnapshot {
	targetAuthPool := normalizeAuthPool(authPool)
	if targetAuthPool == "" {
		return snapshot
	}
	filtered := newEmptySnapshot()

	for apiName, apiSnapshot := range snapshot.APIs {
		for modelName, modelSnapshot := range apiSnapshot.Models {
			for _, detail := range modelSnapshot.Details {
				if normalizeAuthPool(detail.AuthPool) != targetAuthPool {
					continue
				}
				appendDetailToSnapshot(&filtered, apiName, modelName, detail)
			}
		}
	}

	return filtered
}

// AggregateSnapshotByPool returns snapshot aggregates split by pool_type.
func AggregateSnapshotByPool(snapshot StatisticsSnapshot) map[string]StatisticsSnapshot {
	poolBuckets := map[string]*StatisticsSnapshot{
		PoolTypePlus:   ptrSnapshot(newEmptySnapshot()),
		PoolTypeFree:   ptrSnapshot(newEmptySnapshot()),
		PoolTypeCustom: ptrSnapshot(newEmptySnapshot()),
	}

	for apiName, apiSnapshot := range snapshot.APIs {
		for modelName, modelSnapshot := range apiSnapshot.Models {
			for _, detail := range modelSnapshot.Details {
				poolType := normalizePoolType(detail.PoolType)
				appendDetailToSnapshot(poolBuckets[poolType], apiName, modelName, detail)
			}
		}
	}

	aggregated := make(map[string]StatisticsSnapshot, len(canonicalPoolTypes))
	for _, poolType := range canonicalPoolTypes {
		aggregated[poolType] = *poolBuckets[poolType]
	}
	return aggregated
}

func ptrSnapshot(snapshot StatisticsSnapshot) *StatisticsSnapshot {
	value := snapshot
	return &value
}
