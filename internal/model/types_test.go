package model

import "testing"

func TestNormalizeAgents_DefaultsWhenEmpty(t *testing.T) {
	got := NormalizeAgents(nil)
	if len(got) != 3 || got[0] != "larry" || got[1] != "curly" || got[2] != "moe" {
		t.Fatalf("unexpected defaults: %#v", got)
	}
}

func TestNormalizeAgents_DedupesAndTrims(t *testing.T) {
	got := NormalizeAgents([]string{" larry ", "", "moe", "larry"})
	if len(got) != 2 || got[0] != "larry" || got[1] != "moe" {
		t.Fatalf("unexpected normalized agents: %#v", got)
	}
}

func TestValidateName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{name: "larry", ok: true},
		{name: "", ok: false},
		{name: "../bad", ok: false},
		{name: "bad/name", ok: false},
	}

	for _, tc := range cases {
		err := ValidateName(tc.name)
		if tc.ok && err != nil {
			t.Fatalf("expected ok for %q: %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("expected error for %q", tc.name)
		}
	}
}

func TestCanonicalBaseDir(t *testing.T) {
	if got := CanonicalBaseDir("feature/foo"); got != "feature-foo" {
		t.Fatalf("unexpected canonical branch: %q", got)
	}
	if got := CanonicalBaseDir(""); got != "main" {
		t.Fatalf("expected main default, got %q", got)
	}
}
