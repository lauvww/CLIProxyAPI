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
