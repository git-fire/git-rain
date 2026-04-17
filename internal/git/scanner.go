package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ScanRepositoriesStream finds all git repositories and sends each one to out
// as soon as it is analyzed. out is closed when scanning is complete.
func ScanRepositoriesStream(opts ScanOptions, out chan<- Repository) error {
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	semaphore := make(chan struct{}, opts.Workers)
	var wg sync.WaitGroup

	absRoot, err := filepath.Abs(opts.RootPath)
	if err != nil {
		close(out)
		if opts.FolderProgress != nil {
			close(opts.FolderProgress)
		}
		return fmt.Errorf("resolving scan path: %w", err)
	}

	spawnAnalysis := func(repoPath string) {
		if ctx.Err() != nil {
			return
		}
		wg.Add(1)
		semaphore <- struct{}{}
		go func(p string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			if ctx.Err() != nil {
				return
			}
			fi, err := os.Stat(p)
			if err != nil || !fi.IsDir() {
				return
			}
			repo, err := analyzeRepository(ctx, p)
			if err != nil {
				return
			}
			if ctx.Err() == nil {
				out <- repo
			}
		}(repoPath)
	}

	for knownPath := range opts.KnownPaths {
		spawnAnalysis(knownPath)
	}

	var walkErr error
	if !opts.DisableScan {
		walkErr = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if err != nil {
				return nil
			}

			if !info.IsDir() {
				return nil
			}

			absPath, absErr := filepath.Abs(path)
			if absErr != nil {
				return nil
			}

			if opts.FolderProgress != nil {
				select {
				case opts.FolderProgress <- path:
				default:
				}
			}

			if rescan, isKnown := opts.KnownPaths[absPath]; isKnown && !rescan {
				return filepath.SkipDir
			}

			if info.Name() == ".git" {
				repoPath := filepath.Dir(absPath)
				if _, alreadyKnown := opts.KnownPaths[repoPath]; !alreadyKnown {
					spawnAnalysis(repoPath)
				}
				return filepath.SkipDir
			}

			for _, exclude := range opts.Exclude {
				if info.Name() == exclude {
					return filepath.SkipDir
				}
			}

			depth := strings.Count(strings.TrimPrefix(path, absRoot), string(os.PathSeparator))
			if depth > opts.MaxDepth {
				return filepath.SkipDir
			}

			return nil
		})
	}

	wg.Wait()
	close(out)
	if opts.FolderProgress != nil {
		close(opts.FolderProgress)
	}

	if walkErr != nil && ctx.Err() == nil {
		return fmt.Errorf("error scanning repositories: %w", walkErr)
	}
	return nil
}

// ScanRepositories finds all git repositories and returns them as a slice.
func ScanRepositories(opts ScanOptions) ([]Repository, error) {
	out := make(chan Repository, opts.Workers)
	var repos []Repository

	done := make(chan struct{})
	go func() {
		defer close(done)
		for repo := range out {
			repos = append(repos, repo)
		}
	}()

	err := ScanRepositoriesStream(opts, out)
	<-done
	return repos, err
}

// AnalyzeRepository extracts metadata from a git repository at repoPath.
func AnalyzeRepository(repoPath string) (Repository, error) {
	return analyzeRepository(context.Background(), repoPath)
}

func analyzeRepository(ctx context.Context, repoPath string) (Repository, error) {
	if err := ctx.Err(); err != nil {
		return Repository{}, err
	}
	repo := Repository{
		Path:     repoPath,
		Name:     filepath.Base(repoPath),
		Selected: true,
	}

	remotes, err := getRemotes(ctx, repoPath)
	if err == nil {
		repo.Remotes = remotes
	} else if ctx.Err() != nil {
		return Repository{}, ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		return Repository{}, err
	}

	dirty, err := isDirty(ctx, repoPath)
	if err == nil {
		repo.IsDirty = dirty
	} else if ctx.Err() != nil {
		return Repository{}, ctx.Err()
	}

	return repo, nil
}

func getRemotes(ctx context.Context, repoPath string) ([]Remote, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "-v")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	remotes := make([]Remote, 0)
	seen := make(map[string]bool)

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		url := parts[1]

		if seen[name] {
			continue
		}
		seen[name] = true

		remotes = append(remotes, Remote{
			Name: name,
			URL:  url,
		})
	}

	return remotes, nil
}

func isDirty(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}
