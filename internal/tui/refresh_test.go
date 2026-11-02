package tui

import (
	"context"
	"testing"
)

func TestRefreshRequestsCancelPrevious(t *testing.T) {
	r := &refreshCoordinator{}
	first := r.request(context.Background())
	second := r.request(context.Background())
	if first.Err() != context.Canceled {
		t.Fatal("previous request remains active")
	}
	if second.Err() != nil {
		t.Fatal("new request already cancelled")
	}
	r.stop()
	if second.Err() != context.Canceled {
		t.Fatal("stop did not cancel current request")
	}
}

func TestStaleInventoryCannotReplaceCurrent(t *testing.T) {
	m := Model{generation: 3}
	updated, _ := m.Update(inventoryMsg{generation: 2})
	if updated.(Model).generation != 3 {
		t.Fatal("stale inventory changed generation")
	}
}
