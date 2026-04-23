package usage

import "strings"

const (
	canonicalWWBookKey = "sk-UJ3VkVkY5VEFinWkR"
	canonicalWWPcKey   = "sk-H3vtFmzXhwO7D8eGP"
)

var legacyUsageBucketCleanupRules = map[string]string{
	"local":  canonicalWWBookKey,
	"remote": canonicalWWPcKey,
	"杩滅▼":    canonicalWWBookKey,
	"codex":  canonicalWWBookKey,
}

func canonicalizeLegacyAPIBucketName(apiName string) string {
	trimmed := strings.TrimSpace(apiName)
	if trimmed == "" {
		return ""
	}

	if mapped := legacyUsageBucketCleanupRules[strings.ToLower(trimmed)]; mapped != "" {
		return mapped
	}

	return trimmed
}

// CanonicalizeAPIUsageSnapshot rewrites legacy API usage buckets into canonical
// client API key buckets. It is intended only for persistence/import cleanup,
// not for long-term display logic.
func CanonicalizeAPIUsageSnapshot(snapshot StatisticsSnapshot) (StatisticsSnapshot, bool) {
	if len(snapshot.APIs) == 0 {
		return snapshot, false
	}

	result := snapshot
	result.APIs = make(map[string]APISnapshot, len(snapshot.APIs))
	changed := false

	for apiName, apiSnapshot := range snapshot.APIs {
		targetKey := canonicalizeLegacyAPIBucketName(apiName)
		if targetKey == "" {
			targetKey = strings.TrimSpace(apiName)
		}
		if targetKey != strings.TrimSpace(apiName) {
			changed = true
		}

		existing := result.APIs[targetKey]
		if existing.Models == nil {
			existing.Models = make(map[string]ModelSnapshot, len(apiSnapshot.Models))
		}
		existing.TotalRequests += apiSnapshot.TotalRequests
		existing.TotalTokens += apiSnapshot.TotalTokens

		for modelName, modelSnapshot := range apiSnapshot.Models {
			existingModel := existing.Models[modelName]
			existingModel.TotalRequests += modelSnapshot.TotalRequests
			existingModel.TotalTokens += modelSnapshot.TotalTokens
			if len(modelSnapshot.Details) > 0 {
				details := make([]RequestDetail, len(modelSnapshot.Details))
				for idx, detail := range modelSnapshot.Details {
					detail = normalizeRequestDetailDimensions(detail)
					if detail.ClientAPIKey == "" && targetKey != strings.TrimSpace(apiName) {
						detail.ClientAPIKey = targetKey
						changed = true
					}
					details[idx] = detail
				}
				existingModel.Details = append(existingModel.Details, details...)
			}
			existing.Models[modelName] = existingModel
		}

		result.APIs[targetKey] = existing
	}

	return result, changed
}
