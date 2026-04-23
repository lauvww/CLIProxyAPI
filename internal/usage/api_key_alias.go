package usage

import (
	"strings"
	"sync/atomic"
)

var apiKeyAliasesValue atomic.Value

func init() {
	apiKeyAliasesValue.Store(map[string]string{})
}

// SetAPIKeyAliases updates the display aliases used for configured client API keys.
func SetAPIKeyAliases(aliases map[string]string) {
	if len(aliases) == 0 {
		apiKeyAliasesValue.Store(map[string]string{})
		return
	}

	normalized := make(map[string]string, len(aliases))
	for rawKey, rawAlias := range aliases {
		key := strings.TrimSpace(rawKey)
		alias := strings.TrimSpace(rawAlias)
		if key == "" || alias == "" {
			continue
		}
		normalized[key] = alias
	}

	apiKeyAliasesValue.Store(normalized)
}

func lookupAPIKeyAlias(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ""
	}

	aliases, _ := apiKeyAliasesValue.Load().(map[string]string)
	if len(aliases) == 0 {
		return ""
	}

	return strings.TrimSpace(aliases[apiKey])
}

func displayAPIBucketName(apiName string) string {
	apiName = strings.TrimSpace(apiName)
	if apiName == "" {
		return ""
	}

	if alias := lookupAPIKeyAlias(apiName); alias != "" {
		return alias
	}

	return apiName
}

func resolveSnapshotBucketAPIName(apiName string, apiSnapshot APISnapshot) string {
	resolved := strings.TrimSpace(apiName)
	if resolved == "" {
		return ""
	}

	inferredClientKey := ""
	for _, modelSnapshot := range apiSnapshot.Models {
		for _, detail := range modelSnapshot.Details {
			clientAPIKey := strings.TrimSpace(detail.ClientAPIKey)
			if clientAPIKey == "" {
				continue
			}
			if inferredClientKey == "" {
				inferredClientKey = clientAPIKey
				continue
			}
			if !strings.EqualFold(inferredClientKey, clientAPIKey) {
				return resolved
			}
		}
	}

	if inferredClientKey != "" {
		return inferredClientKey
	}

	return resolved
}

// ApplyAPIKeyAliasesToSnapshot remaps API buckets using the current alias
// table at read time so alias edits immediately reflect in usage statistics.
func ApplyAPIKeyAliasesToSnapshot(snapshot StatisticsSnapshot) StatisticsSnapshot {
	if len(snapshot.APIs) == 0 {
		return snapshot
	}

	result := snapshot
	result.APIs = make(map[string]APISnapshot, len(snapshot.APIs))
	for apiName, apiSnapshot := range snapshot.APIs {
		mappedName := displayAPIBucketName(resolveSnapshotBucketAPIName(apiName, apiSnapshot))
		if mappedName == "" {
			mappedName = apiName
		}

		existing := result.APIs[mappedName]
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
				existingModel.Details = append(existingModel.Details, modelSnapshot.Details...)
			}
			existing.Models[modelName] = existingModel
		}

		result.APIs[mappedName] = existing
	}

	return result
}
