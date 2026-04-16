package git

import (
	"errors"
	"os/exec"
	"strings"
)

// RainFetchPruneConfigKey is the local git config key checked in each repo (highest
// precedence after CLI). Use: git config --local --bool rain.fetchprune true
const RainFetchPruneConfigKey = "rain.fetchprune"

// ReadRainFetchPrune reads local repo config rain.fetchprune as a bool. If unset, set is false.
func ReadRainFetchPrune(repoPath string) (set bool, val bool, err error) {
	cmd := exec.Command("git", "config", "--local", "--bool", "--get", RainFetchPruneConfigKey)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			switch ee.ExitCode() {
			case 1: // unset
				return false, false, nil
			case 128: // not a git repository
				return false, false, nil
			}
		}
		return false, false, err
	}
	return true, strings.TrimSpace(string(out)) == "true", nil
}

// ResolveFetchPrune decides whether to pass --prune to git fetch for one repository.
// Precedence (highest first):
//  1. CLI — when cliSet is true, cliVal is used
//  2. Local git config rain.fetchprune in the repo (when gitSet is true)
//  3. Registry per-repo override (when regPrune is non-nil)
//  4. Global config default (globalPrune)
func ResolveFetchPrune(cliSet, cliVal bool, gitSet bool, gitVal bool, regPrune *bool, globalPrune bool) bool {
	if cliSet {
		return cliVal
	}
	if gitSet {
		return gitVal
	}
	if regPrune != nil {
		return *regPrune
	}
	return globalPrune
}
