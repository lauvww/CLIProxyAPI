package registry

import "testing"

func TestStaticModelDefinitions_AddedEntriesExist(t *testing.T) {
	t.Helper()

	claudeOpus47 := LookupStaticModelInfo("claude-opus-4-7")
	if claudeOpus47 == nil {
		t.Fatalf("expected claude-opus-4-7 to exist in static model catalog")
	}
	if claudeOpus47.DisplayName != "Claude Opus 4.7" {
		t.Fatalf("claude-opus-4-7 display name = %q, want %q", claudeOpus47.DisplayName, "Claude Opus 4.7")
	}

	var foundAntigravityLite bool
	for _, model := range GetAntigravityModels() {
		if model != nil && model.ID == "gemini-3.1-flash-lite" {
			foundAntigravityLite = true
			if model.DisplayName != "Gemini 3.1 Flash Lite" {
				t.Fatalf("gemini-3.1-flash-lite display name = %q, want %q", model.DisplayName, "Gemini 3.1 Flash Lite")
			}
			break
		}
	}
	if !foundAntigravityLite {
		t.Fatalf("expected antigravity gemini-3.1-flash-lite to exist in static model catalog")
	}
}

func TestStaticModelDefinitions_PreserveForkCodexTierModels(t *testing.T) {
	t.Helper()

	checkCodexTierModel := func(name string, models []*ModelInfo) {
		t.Helper()
		for _, model := range models {
			if model != nil && model.ID == name {
				return
			}
		}
		t.Fatalf("expected %s to remain in fork codex tier model definitions", name)
	}

	checkCodexTierModel("gpt-5", GetCodexFreeModels())
	checkCodexTierModel("gpt-5-codex", GetCodexPlusModels())
	checkCodexTierModel("gpt-5.1", GetCodexTeamModels())
	checkCodexTierModel("gpt-5.2-codex", GetCodexProModels())
}
