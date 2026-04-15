package cli

import "testing"

func TestDeriveTrackWorkspaceName_UsesLastNonEmptySegment(t *testing.T) {
	got, err := deriveTrackWorkspaceName("feature/foo/")
	if err != nil {
		t.Fatalf("derive failed: %v", err)
	}
	if got != "foo" {
		t.Fatalf("expected foo, got %q", got)
	}
}

func TestDeriveTrackWorkspaceName_SanitizesAndTruncatesWithoutSlash(t *testing.T) {
	got, err := deriveTrackWorkspaceName("release candidate 2026 04 15 with a surprisingly long branch name for testing")
	if err != nil {
		t.Fatalf("derive failed: %v", err)
	}
	if got != "release-candidate-2026-04-15-with-a-surprisingly-l" {
		t.Fatalf("unexpected derived name %q", got)
	}
}

func TestDeriveTrackWorkspaceName_RejectsReservedName(t *testing.T) {
	if _, err := deriveTrackWorkspaceName("feature/base"); err == nil {
		t.Fatal("expected reserved-name error")
	}
}

func TestDeriveBranchWorkspaceName_RejectsReservedName(t *testing.T) {
	if _, err := deriveBranchWorkspaceName("feature/base"); err == nil {
		t.Fatal("expected reserved-name error")
	}
}
