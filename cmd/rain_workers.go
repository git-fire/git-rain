package cmd

import (
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
)

// runRainReposParallel runs fetch/rain for repos using the same bounded worker
// pool as the default streaming CLI path.
func runRainReposParallel(reg *registry.Registry, repos []git.Repository, rainOpts git.RainOptions, fullSync bool, fetchWorkers int) rainTotals {
	if len(repos) == 0 {
		return rainTotals{}
	}

	workers := fetchWorkerCount(fetchWorkers)
	repoChan := make(chan git.Repository, len(repos))
	for _, repo := range repos {
		repoChan <- repo
	}
	close(repoChan)

	var printMu sync.Mutex
	var (
		updated int64
		skipped int64
		frozen  int64
		failed  int64
		seq     int64
	)
	total := int64(len(repos))

	runCtx := rainOpts.RunCtx
	var workersWG sync.WaitGroup
	for w := 0; w < workers; w++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			for repo := range repoChan {
				if runCtx != nil && runCtx.Err() != nil {
					break
				}
				current := int(atomic.AddInt64(&seq, 1))
				var buf strings.Builder
				delta := runRainOnRepo(reg, repo, rainOpts, fullSync, current, int(total), &buf)
				printMu.Lock()
				_, _ = io.WriteString(os.Stdout, buf.String())
				printMu.Unlock()
				atomic.AddInt64(&updated, int64(delta.updated))
				atomic.AddInt64(&skipped, int64(delta.skipped))
				atomic.AddInt64(&frozen, int64(delta.frozen))
				atomic.AddInt64(&failed, int64(delta.failed))
			}
		}()
	}
	workersWG.Wait()

	return rainTotals{
		updated: int(updated),
		skipped: int(skipped),
		frozen:  int(frozen),
		failed:  int(failed),
	}
}
