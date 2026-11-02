package tui

import "testing"

func TestTargetedMergePreservesUnrelatedRowsAndFilter(t *testing.T) {
	m := newTestModel(t)
	if len(m.apps.all) == 0 {
		t.Fatal("fixture has no apps")
	}
	first := m.apps.all[0]
	m.apps.search.SetValue(first.App.Label())
	m.apps.applyFilter(first.App.Ref.Relative)
	replacement := first
	replacement.Build.Reason = "targeted update"
	m.apps.merge([]ApplicationView{replacement}, "")
	if m.apps.search.Value() != first.App.Label() {
		t.Fatal("filter lost")
	}
	selected, ok := m.apps.selected()
	if !ok || selected.App.Ref.Relative != first.App.Ref.Relative || selected.Build.Reason != "targeted update" {
		t.Fatal("selection or update lost")
	}
}
