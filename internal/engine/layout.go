package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
)

const (
	stoogesDirName       = ".stooges"
	baseRepoAlias        = "base"
	metadataName         = ".stooges-metadata.json"
	mainBranchDefault    = "main"
	mainBranchLegacyName = "master"
)

type WorkspaceLayout struct {
	WorkspaceRoot     string
	BaseRepoPath      string
	MetadataPath      string
	MainBranch        string
	ManagedWorkspaces []string
}

type workspaceMetadata struct {
	MainBranch        string   `json:"mainBranch"`
	ManagedWorkspaces []string `json:"managedWorkspaces"`
}

func workspaceRootFromCWD(cwd string) string {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return trimmed
	}

	current := filepath.Clean(trimmed)
	// If currently somewhere under .stooges, root is the parent of that .stooges dir.
	for {
		if filepath.Base(current) == stoogesDirName {
			return filepath.Dir(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Otherwise, walk ancestors and pick the first directory that contains .stooges.
	current = filepath.Clean(trimmed)
	for {
		if pathExists(filepath.Join(current, stoogesDirName)) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return filepath.Clean(trimmed)
}

func layoutFromRoot(root string) WorkspaceLayout {
	basePath := filepath.Join(root, stoogesDirName)
	return WorkspaceLayout{
		WorkspaceRoot: root,
		BaseRepoPath:  basePath,
		MetadataPath:  filepath.Join(root, metadataName),
		MainBranch:    mainBranchDefault,
	}
}

func loadWorkspaceLayout(root string) (WorkspaceLayout, error) {
	layout := layoutFromRoot(root)
	if !isGitRepoPath(layout.BaseRepoPath) {
		return WorkspaceLayout{}, apperrors.New(apperrors.KindInvalidInput, "workspace not configured (missing .stooges); run `stooges init`")
	}
	data, err := os.ReadFile(layout.MetadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceLayout{}, apperrors.New(apperrors.KindInvalidInput, "workspace metadata missing (.stooges-metadata.json); re-run `stooges init`")
		}
		return WorkspaceLayout{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "read workspace metadata", err)
	}
	var meta workspaceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return WorkspaceLayout{}, apperrors.Wrap(apperrors.KindInvalidInput, "parse workspace metadata", err)
	}
	if strings.TrimSpace(meta.MainBranch) == "" {
		return WorkspaceLayout{}, apperrors.New(apperrors.KindInvalidInput, "workspace metadata missing mainBranch")
	}
	layout.MainBranch = strings.TrimSpace(meta.MainBranch)
	layout.ManagedWorkspaces = normalizeManagedWorkspaces(meta.ManagedWorkspaces)
	for _, workspace := range layout.ManagedWorkspaces {
		if err := validateWorkspaceEntryName(workspace); err != nil {
			return WorkspaceLayout{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("invalid managed workspace entry %q in metadata", workspace))
		}
	}
	return layout, nil
}

func writeWorkspaceMetadata(layout WorkspaceLayout) error {
	meta := workspaceMetadata{
		MainBranch:        strings.TrimSpace(layout.MainBranch),
		ManagedWorkspaces: normalizeManagedWorkspaces(layout.ManagedWorkspaces),
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(layout.MetadataPath, append(data, '\n'), 0o644); err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "write workspace metadata", err)
	}
	return nil
}

func resolveWorkspaceAndLayout(cwd string) (string, WorkspaceLayout, error) {
	root := workspaceRootFromCWD(cwd)
	if strings.TrimSpace(root) == "" {
		return "", WorkspaceLayout{}, apperrors.New(apperrors.KindInvalidInput, "workspace path is empty")
	}
	layout, err := loadWorkspaceLayout(root)
	if err != nil {
		return "", WorkspaceLayout{}, err
	}
	return root, layout, nil
}

func resolveBaseRepo(layout WorkspaceLayout, explicit string) (string, error) {
	if strings.TrimSpace(explicit) == "" {
		return layout.BaseRepoPath, nil
	}
	target, err := filepath.Abs(strings.TrimSpace(explicit))
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindInvalidInput, "invalid repo path", err)
	}
	if !isGitRepoPath(target) {
		return "", apperrors.New(apperrors.KindInvalidInput, "provided path is not a git repo (missing .git)")
	}
	if target != layout.BaseRepoPath {
		return "", apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("unsupported repo path %q; only base repo %q is supported", target, layout.BaseRepoPath))
	}
	return target, nil
}

func resolveSourceRepo(layout WorkspaceLayout, sourceName string) (string, error) {
	source := strings.TrimSpace(sourceName)
	if source == "" || source == mainBranchDefault || source == mainBranchLegacyName || source == baseRepoAlias || source == stoogesDirName {
		if !isGitRepoPath(layout.BaseRepoPath) {
			return "", apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("source workspace %q missing or not a git repo", stoogesDirName))
		}
		return layout.BaseRepoPath, nil
	}
	if err := validateWorkspaceEntryName(source); err != nil {
		return "", err
	}
	sourcePath := filepath.Join(layout.WorkspaceRoot, source)
	if !isGitRepoPath(sourcePath) {
		return "", apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("source workspace %q missing or not a git repo", source))
	}
	return sourcePath, nil
}

func validateWorkspaceEntryName(name string) error {
	if err := validateLayoutName(name); err != nil {
		return apperrors.Wrap(apperrors.KindInvalidInput, "invalid workspace name", err)
	}
	if name == baseRepoAlias {
		return apperrors.New(apperrors.KindInvalidInput, "workspace name \"base\" is reserved")
	}
	return nil
}

func validateLayoutName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "..") {
		return fmt.Errorf("invalid name %q: cannot contain '/' or '..'", trimmed)
	}
	if strings.HasPrefix(trimmed, ".") {
		return fmt.Errorf("invalid name %q: cannot start with '.'", trimmed)
	}
	return nil
}

func listManagedWorkspaces(layout WorkspaceLayout) ([]string, error) {
	repos := []string{layout.BaseRepoPath}
	for _, workspace := range layout.ManagedWorkspaces {
		repoPath := filepath.Join(layout.WorkspaceRoot, workspace)
		if isGitRepoPath(repoPath) {
			repos = append(repos, repoPath)
		}
	}
	slices.Sort(repos)
	return repos, nil
}

func appendManagedWorkspaces(existing []string, names ...string) []string {
	managed := normalizeManagedWorkspaces(existing)
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		managed = append(managed, trimmed)
	}
	return normalizeManagedWorkspaces(managed)
}

func normalizeManagedWorkspaces(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
