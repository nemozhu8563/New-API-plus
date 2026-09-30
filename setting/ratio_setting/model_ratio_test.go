package ratio_setting

import "testing"

func restoreCompletionRatioMap(t *testing.T) func() {
	t.Helper()

	original := CompletionRatio2JSONString()
	return func() {
		if err := UpdateCompletionRatioByJSONString(original); err != nil {
			t.Fatalf("restore completion ratio map: %v", err)
		}
	}
}

func TestGetCompletionRatioIgnoresOverrideForLockedHardcodedModel(t *testing.T) {
	defer restoreCompletionRatioMap(t)()

	if err := UpdateCompletionRatioByJSONString(`{"gpt-5.4":3.5}`); err != nil {
		t.Fatalf("set completion ratio override: %v", err)
	}

	if got := GetCompletionRatio("gpt-5.4"); got != 6 {
		t.Fatalf("expected locked hardcoded completion ratio 6, got %v", got)
	}

	info := GetCompletionRatioInfo("gpt-5.4")
	if info.Ratio != 6 {
		t.Fatalf("expected completion ratio info to expose hardcoded 6, got %v", info.Ratio)
	}
	if !info.Locked {
		t.Fatalf("expected locked hardcoded completion ratio to stay locked")
	}
}

func TestGetCompletionRatioAppliesOverrideForUnlockedModel(t *testing.T) {
	defer restoreCompletionRatioMap(t)()

	if err := UpdateCompletionRatioByJSONString(`{"gpt-5.5":7}`); err != nil {
		t.Fatalf("set completion ratio override: %v", err)
	}

	if got := GetCompletionRatio("gpt-5.5"); got != 7 {
		t.Fatalf("expected explicit completion ratio override 7, got %v", got)
	}

	info := GetCompletionRatioInfo("gpt-5.5")
	if info.Ratio != 7 {
		t.Fatalf("expected completion ratio info to expose override 7, got %v", info.Ratio)
	}
	if info.Locked {
		t.Fatalf("expected unlocked model override to stay editable")
	}
}

func TestGetCompletionRatioKeepsVendorNameConfigured(t *testing.T) {
	defer restoreCompletionRatioMap(t)()

	if err := UpdateCompletionRatioByJSONString(`{"openai/gpt-5.5":9}`); err != nil {
		t.Fatalf("set vendor-prefixed completion ratio override: %v", err)
	}

	info := GetCompletionRatioInfo("openai/gpt-5.5")
	if info.Ratio != 9 {
		t.Fatalf("expected vendor-prefixed configured ratio 9, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoKeepsNonGPTHardcodedRatiosLocked(t *testing.T) {
	defer restoreCompletionRatioMap(t)()

	if err := UpdateCompletionRatioByJSONString(`{}`); err != nil {
		t.Fatalf("clear completion ratio overrides: %v", err)
	}

	info := GetCompletionRatioInfo("claude-3-5-sonnet")
	if info.Ratio != 5 {
		t.Fatalf("expected claude hardcoded completion ratio 5, got %v", info.Ratio)
	}
	if !info.Locked {
		t.Fatalf("expected non-GPT hardcoded completion ratio to remain locked")
	}
}
