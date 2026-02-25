package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) rollbackInit(workspaceRoot, basePath, metadataPath string, movedEntries, createdWorkspaces []string, baseLocked, baseCreated bool) error {
	rollbackErrs := make([]error, 0)

	for i := len(createdWorkspaces) - 1; i >= 0; i-- {
		workspace := createdWorkspaces[i]
		path := filepath.Join(workspaceRoot, workspace)
		if !pathExists(path) {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove workspace %s: %w", workspace, err))
		}
	}

	if baseLocked && pathExists(basePath) {
		if err := s.perms.UnlockWritable(basePath); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("unlock base repo during rollback: %w", err))
		}
	}

	for i := len(movedEntries) - 1; i >= 0; i-- {
		name := movedEntries[i]
		src := filepath.Join(basePath, name)
		dst := filepath.Join(workspaceRoot, name)
		if !pathExists(src) {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore %s: %w", name, err))
		}
	}

	if pathExists(metadataPath) {
		if err := os.Remove(metadataPath); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove workspace metadata during rollback: %w", err))
		}
	}

	if baseCreated && pathExists(basePath) {
		if err := os.RemoveAll(basePath); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove .stooges during rollback: %w", err))
		}
	}

	if len(rollbackErrs) == 0 {
		return nil
	}
	return errors.Join(rollbackErrs...)
}
