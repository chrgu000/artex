package db

import (
	"testing"
	"unicode"
)

// TestManualTaskScopeSummaryLocalized pins the manual task-scope provenance label.
// It is stored on task_asset_links.source_summary (AddTaskScope / SetTaskAssetSource)
// and rendered verbatim on the task detail sessions/assets tabs, so reverting it to
// Chinese would surface mixed-language provenance labels in the same panel. The
// existing task_assets_test.go already asserts the stored value equals this constant
// via the symbol, so pinning the constant here protects the user-facing text too.
func TestManualTaskScopeSummaryLocalized(t *testing.T) {
	if manualTaskScopeSummary == "" {
		t.Fatal("manualTaskScopeSummary: 빈 문자열")
	}
	hasHan := false
	for _, r := range manualTaskScopeSummary {
		if unicode.Is(unicode.Hangul, r) {
			t.Fatalf("manualTaskScopeSummary: 중국어 한자가 남아 있습니다: %q", manualTaskScopeSummary)
		}
		if unicode.Is(unicode.Han, r) {
			hasHan = true
		}
	}
	if !hasHan {
		t.Fatalf("manualTaskScopeSummary: 한글이 없습니다: %q", manualTaskScopeSummary)
	}
}
