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
func runRainReposParallel(reg *registry.Registry, repos []git.Repository, rainOpts git.RainOptions, fullSync bool, fetchWorkers int) (rainTotals, []jsonRepoResult) {
	if len(repos) == 0 {
		return rainTotals{}, nil
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
	var jsonReports []jsonRepoResult
	var jsonReportsMu sync.Mutex

	runCtx := rainOpts.RunCtx
	var workersWG sync.WaitGroup
	for w := 0; w < workers; w++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			for repo := range repoChan {
				if runCtx != nil && runCtx.Err() != nil {
					continue
				}
				current := int(atomic.AddInt64(&seq, 1))
				var buf strings.Builder
				delta, report := runRainOnRepo(reg, repo, rainOpts, fullSync, current, int(total), &buf, !rainJSON)

				printMu.Lock()
				if !rainJSON {
					_, _ = io.WriteString(os.Stdout, buf.String())
				}
				printMu.Unlock()

				if rainJSON {
					jsonReportsMu.Lock()
					jsonReports = append(jsonReports, report)
					jsonReportsMu.Unlock()
				}

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
	}, jsonReports
}
