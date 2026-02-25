package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockReadOnlyClearsAllWriteBits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o666); err != nil {
		t.Fatalf("write: %v", err)
	}

	mgr := NewPermissionManager()
	if err := mgr.LockReadOnly(root); err != nil {
		t.Fatalf("lock: %v", err)
	}
	t.Cleanup(func() {
		_ = mgr.UnlockWritable(root)
	})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("expected no write bits, got %o", info.Mode().Perm())
	}
}

func TestUnlockWritableAddsAllWriteBits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o444); err != nil {
		t.Fatalf("write: %v", err)
	}

	mgr := NewPermissionManager()
	if err := mgr.UnlockWritable(root); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o222 != 0o222 {
		t.Fatalf("expected all write bits, got %o", info.Mode().Perm())
	}
}
