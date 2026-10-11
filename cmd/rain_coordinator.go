package cmd

import (
	"context"
	"sync"
)

// rainCoordinator provides per-repo cancel contexts under a shared run context.
type rainCoordinator struct {
	parent context.Context
	mu     sync.Mutex
	repos  map[string]context.Context
	cancel map[string]context.CancelFunc
}

func newRainCoordinator(parent context.Context, repoPaths []string) *rainCoordinator {
	if parent == nil {
		parent = context.Background()
	}
	c := &rainCoordinator{
		parent: parent,
		repos:  make(map[string]context.Context, len(repoPaths)),
		cancel: make(map[string]context.CancelFunc, len(repoPaths)),
	}
	for _, p := range repoPaths {
		ctx, cancel := context.WithCancel(parent)
		c.repos[p] = ctx
		c.cancel[p] = cancel
	}
	return c
}

func (c *rainCoordinator) ContextFor(repoPath string) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx, ok := c.repos[repoPath]; ok {
		return ctx
	}
	return c.parent
}

func (c *rainCoordinator) SkipRepo(repoPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel, ok := c.cancel[repoPath]; ok {
		cancel()
	}
}

func (c *rainCoordinator) AbortAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cancel := range c.cancel {
		cancel()
	}
}
