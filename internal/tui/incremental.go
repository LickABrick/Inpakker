package tui

import "sort"

// merge applies operation-local observations without rescanning unrelated apps.
// Manual refresh remains responsible for discovering external moves/removals.
func (p *applicationsPage) merge(updated []ApplicationView, preferred string) {
 if len(updated) == 0 { return }
 all := append([]ApplicationView(nil), p.all...)
 positions := make(map[string]int, len(all))
 for i, app := range all { positions[app.App.Ref.Relative] = i }
 for _, app := range updated {
  if i, ok := positions[app.App.Ref.Relative]; ok { all[i] = app } else {
   positions[app.App.Ref.Relative] = len(all)
   all = append(all, app)
  }
 }
 sort.SliceStable(all, func(i,j int) bool { return all[i].App.Ref.Relative < all[j].App.Ref.Relative })
 p.refresh(all, preferred)
}
