package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureForkSourceSafeRejectsMergeState(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "MERGE_HEAD"), []byte("abc123\n"), 0o644); err != nil {
		t.Fatalf("write merge head: %v", err)
	}

	err := ensureForkSourceSafe(repo)
	if err == nil || !strings.Contains(err.Error(), "merge in progress") {
		t.Fatalf("expected merge-in-progress error, got %v", err)
	}
}
