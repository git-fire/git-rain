package cmd

import (
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/rainexec"
	"github.com/git-rain/git-rain/internal/registry"
)

func emitRainEvent(ch chan<- rainexec.Event, e rainexec.Event) {
	if ch == nil {
		return
	}
	ch <- e
}

// runRainReposParallel runs fetch/rain for repos using the same bounded worker
// pool as the default streaming CLI path. When progress is non-nil, repo status
// events are emitted; when quiet is true, per-repo CLI blocks are suppressed.
func runRainReposParallel(reg *registry.Registry, repos []git.Repository, rainOpts git.RainOptions, fullSync bool, fetchWorkers int, progress chan rainexec.Event, coord *rainCoordinator, quiet bool) (rainTotals, []jsonRepoResult) {
	if len(repos) == 0 {
		return rainTotals{}, nil
	}

	for _, repo := range repos {
		emitRainEvent(progress, rainexec.Event{
			RepoPath: repo.Path,
			RepoName: repo.Name,
			State:    rainexec.StateQueued,
		})
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
				repoCtx := runCtx
				if coord != nil {
					repoCtx = coord.ContextFor(repo.Path)
				}
				if repoCtx != nil && repoCtx.Err() != nil {
					emitRainEvent(progress, rainexec.Event{
						RepoPath: repo.Path,
						RepoName: repo.Name,
						State:    rainexec.StateSkipped,
						Detail:   "skipped",
					})
					continue
				}

				started := time.Now()
				maxAttempts := rainOpts.FetchRetryMaxAttempts
				if maxAttempts <= 0 {
					maxAttempts = git.DefaultFetchRetryMaxAttempts
				}
				emitRainEvent(progress, rainexec.Event{
					RepoPath:    repo.Path,
					RepoName:    repo.Name,
					State:       rainexec.StateRunning,
					Attempt:     1,
					MaxAttempts: maxAttempts,
					StartedAt:   started,
				})

				repoOpts := rainOpts
				repoOpts.RunCtx = repoCtx
				repoOpts.OnFetchAttempt = func(attempt, max int) {
					emitRainEvent(progress, rainexec.Event{
						RepoPath:    repo.Path,
						RepoName:    repo.Name,
						State:       rainexec.StateRetrying,
						Attempt:     attempt,
						MaxAttempts: max,
						StartedAt:   started,
					})
				}

				current := int(atomic.AddInt64(&seq, 1))
				var buf strings.Builder
				emitHuman := !quiet && !rainJSON
				delta, report := runRainOnRepo(reg, repo, repoOpts, fullSync, current, int(total), &buf, emitHuman)
				state, delta := classifyRepoRainOutcome(repoCtx, delta, buf.String())
				detail := ""
				if state == rainexec.StateSkipped {
					detail = "skipped"
				}
				emitRainEvent(progress, rainexec.Event{
					RepoPath:  repo.Path,
					RepoName:  repo.Name,
					State:     state,
					StartedAt: started,
					Detail:    detail,
				})

				if emitHuman {
					printMu.Lock()
					_, _ = io.WriteString(os.Stdout, buf.String())
					printMu.Unlock()
				}

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
