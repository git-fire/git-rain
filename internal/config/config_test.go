package config_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/git-rain/git-rain/internal/config"
)

func TestDefaultConfig_Values(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Global.BranchMode != "mainline" {
		t.Errorf("default BranchMode = %q, want %q", cfg.Global.BranchMode, "mainline")
	}
	if cfg.Global.SyncTags {
		t.Error("default SyncTags should be false")
	}
	if cfg.Global.FetchPrune {
		t.Error("default FetchPrune should be false (prune is opt-in)")
	}
	if len(cfg.Global.MainlinePatterns) != 0 {
		t.Errorf("default MainlinePatterns = %v, want empty", cfg.Global.MainlinePatterns)
	}
	if cfg.Global.RiskyMode {
		t.Error("default RiskyMode should be false")
	}
	if cfg.Global.FetchWorkers <= 0 {
		t.Errorf("default FetchWorkers = %d, must be positive", cfg.Global.FetchWorkers)
	}
}

func TestLoad_NoConfigFile_UsesDefaults(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, ".config"))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Global.BranchMode != "mainline" {
		t.Errorf("BranchMode = %q, want %q", cfg.Global.BranchMode, "mainline")
	}
	if cfg.Global.SyncTags {
		t.Error("SyncTags should default to false")
	}
}

func TestLoad_ExplicitConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[global]
branch_mode = "all-local"
sync_tags = true
mainline_patterns = ["feat/", "JIRA-"]
risky_mode = true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigFile: cfgPath})
	if err != nil {
		t.Fatalf("LoadWithOptions() error = %v", err)
	}
	if cfg.Global.BranchMode != "all-local" {
		t.Errorf("BranchMode = %q, want %q", cfg.Global.BranchMode, "all-local")
	}
	if !cfg.Global.SyncTags {
		t.Error("SyncTags should be true from config file")
	}
	if len(cfg.Global.MainlinePatterns) != 2 {
		t.Errorf("MainlinePatterns len = %d, want 2: %v", len(cfg.Global.MainlinePatterns), cfg.Global.MainlinePatterns)
	}
	if cfg.Global.MainlinePatterns[0] != "feat/" {
		t.Errorf("MainlinePatterns[0] = %q, want %q", cfg.Global.MainlinePatterns[0], "feat/")
	}
	if !cfg.Global.RiskyMode {
		t.Error("RiskyMode should be true from config file")
	}
}

func TestLoad_EnvVarOverride(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, ".config"))
	t.Setenv("GIT_RAIN_GLOBAL_BRANCH_MODE", "checked-out")
	t.Setenv("GIT_RAIN_GLOBAL_SYNC_TAGS", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Global.BranchMode != "checked-out" {
		t.Errorf("BranchMode = %q, want %q (env override)", cfg.Global.BranchMode, "checked-out")
	}
	if !cfg.Global.SyncTags {
		t.Error("SyncTags should be true (env override)")
	}
}

func TestLoad_MissingExplicitConfigFile_Error(t *testing.T) {
	_, err := config.LoadWithOptions(config.LoadOptions{ConfigFile: "/nonexistent/path/config.toml"})
	if err == nil {
		t.Fatal("expected error for missing explicit config file, got nil")
	}
}

func TestSaveConfig_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := config.DefaultConfig()
			if i%2 == 0 {
				c.Global.BranchMode = "mainline"
			} else {
				c.Global.BranchMode = "all-local"
			}
			if err := config.SaveConfig(&c, cfgPath); err != nil {
				t.Errorf("SaveConfig %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	loaded, err := config.LoadWithOptions(config.LoadOptions{ConfigFile: cfgPath})
	if err != nil {
		t.Fatalf("Load after concurrent saves: %v", err)
	}
	if loaded.Global.BranchMode != "mainline" && loaded.Global.BranchMode != "all-local" {
		t.Fatalf("unexpected branch mode %q", loaded.Global.BranchMode)
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	original := config.DefaultConfig()
	original.Global.BranchMode = "all-local"
	original.Global.SyncTags = true
	original.Global.MainlinePatterns = []string{"feat/", "release/"}

	if err := config.SaveConfig(&original, cfgPath); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	loaded, err := config.LoadWithOptions(config.LoadOptions{ConfigFile: cfgPath})
	if err != nil {
		t.Fatalf("LoadWithOptions() after save error = %v", err)
	}
	if loaded.Global.BranchMode != "all-local" {
		t.Errorf("BranchMode round-trip = %q, want %q", loaded.Global.BranchMode, "all-local")
	}
	if !loaded.Global.SyncTags {
		t.Error("SyncTags round-trip should be true")
	}
	if len(loaded.Global.MainlinePatterns) != 2 {
		t.Errorf("MainlinePatterns round-trip len = %d, want 2", len(loaded.Global.MainlinePatterns))
	}
}

func TestValidate_ZeroFetchWorkers_Fixed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Global.FetchWorkers = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Global.FetchWorkers <= 0 {
		t.Errorf("Validate() should fix zero FetchWorkers, got %d", cfg.Global.FetchWorkers)
	}
}

func TestExampleConfigTOML_ContainsBranchMode(t *testing.T) {
	toml := config.ExampleConfigTOML()
	for _, want := range []string{"branch_mode", "sync_tags", "fetch_prune", "mainline_patterns"} {
		if !contains(toml, want) {
			t.Errorf("ExampleConfigTOML missing key %q", want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
