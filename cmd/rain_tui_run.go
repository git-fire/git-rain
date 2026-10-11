package cmd

import (
	"github.com/git-rain/git-rain/internal/config"
	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/rainexec"
	"github.com/git-rain/git-rain/internal/registry"
	"github.com/git-rain/git-rain/internal/ui"
)

func runRainOnReposTUI(cfg *config.Config, reg *registry.Registry, repos []git.Repository, opts git.RainOptions, fullSync bool, abortRun func()) error {
	if len(repos) == 0 {
		return nil
	}

	paths := make([]string, len(repos))
	for i, r := range repos {
		paths[i] = r.Path
	}
	coord := newRainCoordinator(opts.RunCtx, paths)
	events := make(chan rainexec.Event, len(repos)*8)
	done := make(chan rainTotals, 1)

	go func() {
		totals := runRainReposParallel(reg, repos, opts, fullSync, cfg.Global.FetchWorkers, events, coord, true)
		close(events)
		done <- totals
	}()

	ctrl := ui.RainControls{
		Skip: coord.SkipRepo,
		Abort: func() {
			coord.AbortAll()
			if abortRun != nil {
				abortRun()
			}
		},
	}
	_ = ui.RunRainProgressTUI(repos, events, ctrl, opts.FetchTimeoutSec)
	// Drain so workers never block on emitRainEvent after the TUI stops consuming.
	go func() {
		for range events {
		}
	}()
	totals := <-done
	return printRainSummary(totals, opts.RunCtx)
}
