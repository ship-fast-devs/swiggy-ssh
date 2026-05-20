package tui

import (
	"context"
	"testing"
)

func TestHomeSplashTickAnimatesOnlyOnSplash(t *testing.T) {
	m := homeModel{ctx: context.Background(), animate: true}

	updated, cmd := m.Update(homeSplashTickMsg{})
	hm, ok := updated.(homeModel)
	if !ok {
		t.Fatalf("expected homeModel, got %T", updated)
	}
	if hm.shineStep != 1 {
		t.Fatalf("expected shine step to advance, got %d", hm.shineStep)
	}
	if cmd == nil {
		t.Fatal("expected splash tick to re-arm while splash is visible")
	}

	hm.menu = true
	updated, cmd = hm.Update(homeSplashTickMsg{})
	hm, ok = updated.(homeModel)
	if !ok {
		t.Fatalf("expected homeModel, got %T", updated)
	}
	if hm.shineStep != 1 {
		t.Fatalf("expected shine step to stop in menu, got %d", hm.shineStep)
	}
	if cmd != nil {
		t.Fatal("did not expect splash tick to re-arm after entering menu")
	}
}
