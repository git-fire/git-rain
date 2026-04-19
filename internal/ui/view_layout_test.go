package ui

import (
	"testing"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
)

// assertMeasuredCapacityInvariants checks binary-search layout: chosen capacity
// is at least 1, full panel fits windowHeight, and capacity+1 would overflow
// (when list is long enough to test).
func assertMeasuredCapacityInvariants(t *testing.T, windowHeight, capacity, maxList int, panelOuterHeight func(int) int) {
	t.Helper()
	if capacity < 1 {
		t.Fatalf("capacity = %d, want >= 1", capacity)
	}
	h := panelOuterHeight(capacity)
	if h > windowHeight {
		t.Fatalf("panel outer height %d > window %d for capacity %d", h, windowHeight, capacity)
	}
	if capacity+1 <= maxList {
		hNext := panelOuterHeight(capacity + 1)
		if hNext <= windowHeight {
			t.Fatalf("binary search not maximal: capacity=%d height=%d but capacity+1 height=%d also fits", capacity, h, hNext)
		}
	}
}

func TestMeasuredListCapacityFitsWindow_table(t *testing.T) {
	repos := make([]git.Repository, 50)
	for i := range repos {
		repos[i] = git.Repository{
			Name:    "repo",
			Path:    "/tmp/r" + string(rune('a'+i%26)),
			Remotes: []git.Remote{{Name: "origin"}},
			Mode:    git.ModeSyncDefault,
		}
	}
	ignored := make([]registry.RegistryEntry, 35)
	for i := range ignored {
		ignored[i] = registry.RegistryEntry{
			Path: "/tmp/ignored-" + string(rune('0'+i%10)),
		}
	}

	cases := []struct {
		name    string
		model   RepoSelectorModel
		measure func(RepoSelectorModel) int
		height  func(RepoSelectorModel, int) int
		maxList int
	}{
		{
			name: "main view",
			model: RepoSelectorModel{
				repos:        repos,
				windowWidth:  100,
				windowHeight: 28,
				showRain:     false,
				scanDone:     true,
			},
			measure: func(m RepoSelectorModel) int { return m.mainViewMeasuredRepoListCapacity() },
			height:  func(m RepoSelectorModel, capacity int) int { return m.mainViewPanelOuterHeight(capacity) },
			maxList: len(repos),
		},
		{
			name: "ignored view",
			model: RepoSelectorModel{
				ignoredEntries: ignored,
				windowWidth:    90,
				windowHeight:   24,
				showRain:       false,
			},
			measure: func(m RepoSelectorModel) int { return m.ignoredMeasuredListCapacity() },
			height:  func(m RepoSelectorModel, capacity int) int { return m.ignoredViewPanelOuterHeight(capacity) },
			maxList: len(ignored),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capacity := tc.measure(tc.model)
			panelH := func(c int) int { return tc.height(tc.model, c) }
			assertMeasuredCapacityInvariants(t, tc.model.windowHeight, capacity, tc.maxList, panelH)
		})
	}
}
