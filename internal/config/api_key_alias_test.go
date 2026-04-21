package config

import "testing"

func TestNormalizeAPIKeyAliasesKeepsNonEmptyEntries(t *testing.T) {
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

	if len(cfg.APIKeyAliases) != 2 {
		t.Fatalf("len(APIKeyAliases) = %d, want 2 (%#v)", len(cfg.APIKeyAliases), cfg.APIKeyAliases)
	}
	if got := cfg.APIKeyAliases["key-a"]; got != "Desktop" {
		t.Fatalf("APIKeyAliases[key-a] = %q, want %q", got, "Desktop")
	}
	if got := cfg.APIKeyAliases["key-c"]; got != "Unknown" {
		t.Fatalf("APIKeyAliases[key-c] = %q, want %q", got, "Unknown")
	}
}
