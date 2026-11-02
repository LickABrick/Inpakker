package tui

import (
	"fmt"
	"strings"
	"testing"
)

func TestInventoryIndexesSurviveFilteringAndRefresh(t *testing.T) {
	m := newTestModel(t)
	first := m.apps.all[0]
	first.App.Ref.Relative = "Browsers/example"
	second := first
	second.App.Ref.Relative = "Other/example"
	m.apps.resize(60, 18)
	m.apps.refresh([]ApplicationView{first, second}, "")
	label := first.App.Label()
	if m.apps.nameCounts[label] != 2 || m.apps.positions[second.App.Ref.Relative] != 2 || len(m.apps.searchCache) != 2 {
		t.Fatal("refresh did not index inventory")
	}
	m.apps.search.SetValue("Other/example")
	m.apps.applyFilter("")
	rows := m.apps.table.Rows()
	if len(rows) != 1 || !strings.HasPrefix(rows[0][0], "2 · ") {
		t.Fatalf("filtered duplicate lost stable position: %#v", rows)
	}
	m.apps.search.SetValue("")
	m.apps.refresh([]ApplicationView{second}, "")
	if m.apps.nameCounts[label] != 1 || len(m.apps.positions) != 1 || len(m.apps.searchCache) != 1 {
		t.Fatal("refresh retained stale indexes")
	}
	if strings.Contains(m.apps.table.Rows()[0][0], " · ") {
		t.Fatal("removed duplicate still disambiguated")
	}
}

func TestInventorySearchCacheUsesSnapshotMetadata(t *testing.T) {
	m := newTestModel(t)
	app := m.apps.all[0]
	app.Group = "unique-snapshot-group"
	m.apps.refresh([]ApplicationView{app}, "")
	if m.apps.searchCache[0] != searchText(app) {
		t.Fatal("cached text differs from canonical search text")
	}
	m.apps.search.SetValue("unique-snapshot-group")
	m.apps.applyFilter("")
	if len(m.apps.filtered) != 1 {
		t.Fatal("snapshot group was not searchable")
	}
	app.Group = "replacement-group"
	m.apps.refresh([]ApplicationView{app}, "")
	if len(m.apps.filtered) != 0 {
		t.Fatal("refresh did not invalidate search text")
	}
}

func BenchmarkInventoryDuplicateRows(b *testing.B) {
	p := newApplicationsPage(NewTheme(true))
	p.resize(60, 28)
	apps := make([]ApplicationView, 1000)
	for i := range apps {
		apps[i].App.Ref.Relative = fmt.Sprintf("group-%d/example", i)
		apps[i].App.Status = "valid"
	}
	p.refresh(apps, "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.rebuildTable()
	}
}
