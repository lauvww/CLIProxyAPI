package config

import "testing"

func TestNormalizeAPIKeyAliasesDropsUnknownOrEmptyEntries(t *testing.T) {
	cfg := &Config{
		SDKConfig: SDKConfig{
			APIKeys: []string{" key-a ", "key-b"},
			APIKeyAliases: map[string]string{
				"key-a": "Desktop",
				"key-b": "  ",
				"key-c": "Unknown",
				"":      "Blank",
			},
		},
	}

	cfg.NormalizeAPIKeyAliases()

	if len(cfg.APIKeyAliases) != 1 {
		t.Fatalf("len(APIKeyAliases) = %d, want 1 (%#v)", len(cfg.APIKeyAliases), cfg.APIKeyAliases)
	}
	if got := cfg.APIKeyAliases["key-a"]; got != "Desktop" {
		t.Fatalf("APIKeyAliases[key-a] = %q, want %q", got, "Desktop")
	}
}
