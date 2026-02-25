package fs

import (
	"io/fs"
	"os"
	"path/filepath"

	apperrors "github.com/scottwater/stooges/internal/errors"
)

type PermissionManager struct{}

func NewPermissionManager() *PermissionManager {
	return &PermissionManager{}
}

func (m *PermissionManager) UnlockWritable(root string) error {
	return applyPermissions(root, func(mode os.FileMode) os.FileMode {
		return mode | 0o222
	})
}

func (m *PermissionManager) LockReadOnly(root string) error {
	return applyPermissions(root, func(mode os.FileMode) os.FileMode {
		return mode &^ 0o222
	})
}

func (m *PermissionManager) CountSymlinks(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, apperrors.Wrap(apperrors.KindFilesystemFailure, "count symlinks", err)
	}
	return count, nil
}

func applyPermissions(root string, modeFn func(os.FileMode) os.FileMode) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.Chmod(path, modeFn(info.Mode()))
	})
	if err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "apply filesystem permissions", err)
	}
	return nil
}
