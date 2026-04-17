package git

import "testing"

func TestResolveFetchPrune_CLIOverridesAll(t *testing.T) {
	// CLI set true beats global false, registry, and git
	if !ResolveFetchPrune(true, true, true, false, boolPtr(false), false) {
		t.Fatal("CLI true should win")
	}
	// CLI set false beats git true
	if ResolveFetchPrune(true, false, true, true, boolPtr(true), true) {
		t.Fatal("CLI false should win over git true")
	}
}

func TestResolveFetchPrune_GitOverridesGlobalAndRegistry(t *testing.T) {
	// User requirement: git repo config overrides global
	if !ResolveFetchPrune(false, false, true, true, boolPtr(false), false) {
		t.Fatal("git true should override global false and registry false")
	}
	if ResolveFetchPrune(false, false, true, false, boolPtr(true), true) {
		t.Fatal("git false should override global true and registry true")
	}
}

func TestResolveFetchPrune_RegistryOverridesGlobal(t *testing.T) {
	if !ResolveFetchPrune(false, false, false, false, boolPtr(true), false) {
		t.Fatal("registry true should override global false when git unset")
	}
	if ResolveFetchPrune(false, false, false, false, boolPtr(false), true) {
		t.Fatal("registry false should override global true when git unset")
	}
}

func TestResolveFetchPrune_GitOverridesRegistry(t *testing.T) {
	if !ResolveFetchPrune(false, false, true, true, boolPtr(false), false) {
		t.Fatal("git true should override registry false")
	}
	if ResolveFetchPrune(false, false, true, false, boolPtr(true), true) {
		t.Fatal("git false should override registry true")
	}
}

func TestResolveFetchPrune_FallsBackToGlobal(t *testing.T) {
	if ResolveFetchPrune(false, false, false, false, nil, false) {
		t.Fatal("want false when only global false")
	}
	if !ResolveFetchPrune(false, false, false, false, nil, true) {
		t.Fatal("want true when only global true")
	}
}

func boolPtr(b bool) *bool { return &b }
