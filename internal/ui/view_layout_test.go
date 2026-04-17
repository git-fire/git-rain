package ui

import (
	"testing"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
)

func TestMainViewMeasuredRepoListCapacityFitsWindow(t *testing.T) {
	repos := make([]git.Repository, 50)
	for i := range repos {
		repos[i] = git.Repository{
			Name:    "repo",
			Path:    "/tmp/r" + string(rune('a'+i%26)),
			Remotes: []git.Remote{{Name: "origin"}},
			Mode:    git.ModeSyncDefault,
		}
	}
	m := RepoSelectorModel{
		repos:        repos,
		windowWidth:  100,
		windowHeight: 28,
		showRain:     false,
		scanDone:     true,
	}
	cap := m.mainViewMeasuredRepoListCapacity()
	if cap < 1 {
		t.Fatalf("capacity = %d, want >= 1", cap)
	}
	h := m.mainViewPanelOuterHeight(cap)
	if h > m.windowHeight {
		t.Fatalf("panel outer height %d > window %d for capacity %d", h, m.windowHeight, cap)
	}
	if cap+1 <= len(repos) {
		hNext := m.mainViewPanelOuterHeight(cap + 1)
		if hNext <= m.windowHeight {
			t.Fatalf("binary search not maximal: cap=%d height=%d but cap+1 height=%d also fits", cap, h, hNext)
		}
	}
}

func TestIgnoredMeasuredListCapacityFitsWindow(t *testing.T) {
	ignored := make([]registry.RegistryEntry, 35)
	for i := range ignored {
		ignored[i] = registry.RegistryEntry{
			Path: "/tmp/ignored-" + string(rune('0'+i%10)),
		}
	}
	m := RepoSelectorModel{
		ignoredEntries: ignored,
		windowWidth:    90,
		windowHeight:   24,
		showRain:       false,
	}
	cap := m.ignoredMeasuredListCapacity()
	if cap < 1 {
		t.Fatalf("capacity = %d, want >= 1", cap)
	}
	h := m.ignoredViewPanelOuterHeight(cap)
	if h > m.windowHeight {
		t.Fatalf("panel outer height %d > window %d for capacity %d", h, m.windowHeight, cap)
	}
	if cap+1 <= len(ignored) {
		hNext := m.ignoredViewPanelOuterHeight(cap + 1)
		if hNext <= m.windowHeight {
			t.Fatalf("binary search not maximal: cap=%d height=%d but cap+1 height=%d also fits", cap, h, hNext)
		}
	}
}
