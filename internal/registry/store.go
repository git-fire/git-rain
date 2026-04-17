package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/git-rain/git-rain/internal/config"
)

// DefaultRegistryPath returns the default path for the registry file
// in the user config directory (same directory as config.toml).
func DefaultRegistryPath() (string, error) {
	dir, _ := config.UserGitRainDir()
	return filepath.Join(dir, "repos.toml"), nil
}

// Load reads the registry from disk. If the file or directory does not exist
// it is created and an empty registry is returned.
func Load(path string) (*Registry, error) {
	pkgMu.Lock()
	defer pkgMu.Unlock()

	release, err := acquireLock(path)
	defer release()
	if err != nil {
		_ = err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating registry directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Registry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var reg Registry
	if err := toml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}
	return &reg, nil
}

// Save writes the registry to disk atomically.
func Save(reg *Registry, path string) error {
	pkgMu.Lock()
	defer pkgMu.Unlock()

	release, err := acquireLock(path)
	defer release()
	if err != nil {
		_ = err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}

	data, err := toml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}

	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("saving registry: %w", err)
	}
	return nil
}

// Upsert adds a new entry or updates an existing one (matched by path).
func (r *Registry) Upsert(entry RegistryEntry) {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	for i, e := range r.Repos {
		if e.Path == entry.Path {
			entry.AddedAt = e.AddedAt
			if entry.RescanSubmodules == nil {
				entry.RescanSubmodules = e.RescanSubmodules
			}
			if entry.FetchPrune == nil {
				entry.FetchPrune = e.FetchPrune
			}
			r.Repos[i] = entry
			return
		}
	}
	if entry.AddedAt.IsZero() {
		entry.AddedAt = time.Now()
	}
	r.Repos = append(r.Repos, entry)
}

// SetStatus sets the status of the entry at path. Returns false if not found.
func (r *Registry) SetStatus(path, status string) bool {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	for i, e := range r.Repos {
		if e.Path == path {
			r.Repos[i].Status = status
			if status == StatusActive {
				r.Repos[i].LastSeen = time.Now()
			}
			return true
		}
	}
	return false
}

// Remove hard-deletes an entry by path. Returns false if not found.
func (r *Registry) Remove(path string) bool {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	for i, e := range r.Repos {
		if e.Path == path {
			r.Repos = append(r.Repos[:i], r.Repos[i+1:]...)
			return true
		}
	}
	return false
}

// FindByPath returns a pointer to the entry matching path, or nil if not found.
func (r *Registry) FindByPath(path string) *RegistryEntry {
	pkgMu.RLock()
	defer pkgMu.RUnlock()
	for i := range r.Repos {
		if r.Repos[i].Path == path {
			return &r.Repos[i]
		}
	}
	return nil
}

// UpdateByPath finds the entry at path, calls fn with a write-locked pointer,
// and returns true if found.
func (r *Registry) UpdateByPath(path string, fn func(*RegistryEntry)) bool {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	for i := range r.Repos {
		if r.Repos[i].Path == path {
			fn(&r.Repos[i])
			return true
		}
	}
	return false
}
