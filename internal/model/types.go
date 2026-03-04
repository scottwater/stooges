package model

import (
	"fmt"
	"strings"
)

var DefaultAgents = []string{"larry", "curly", "moe"}

type Platform string

const (
	PlatformDarwin  Platform = "darwin"
	PlatformLinux   Platform = "linux"
	PlatformUnknown Platform = "unknown"
)

type PreflightReport struct {
	Platform          Platform `json:"platform"`
	GitAvailable      bool     `json:"gitAvailable"`
	COWCloneSupported bool     `json:"cowCloneSupported"`
	WorkspaceValid    bool     `json:"workspaceValid"`
	SourceRepoValid   bool     `json:"sourceRepoValid"`
}

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type DoctorReport struct {
	Checks      []DoctorCheck `json:"checks"`
	Platform    Platform      `json:"platform"`
	Workspace   string        `json:"workspace"`
	Suggestions []string      `json:"suggestions,omitempty"`
}

type InitOptions struct {
	MainBranch string
	Agents     []string
}

type MakeOptions struct {
	Agent      string
	Source     string
	Agents     []string
	Track      string
	Branch     string
	BranchAuto bool
}

type SyncOptions struct {
	Repo string
}

type CleanOptions struct {
	Repo string
}

type UnlockOptions struct {
	Repo string
}

type LockOptions struct {
	Repo string
}

type DoctorOptions struct {
	Repo string
}

type RebaseOptions struct {
	Repo  string
	Prune bool
}

type ListOptions struct{}

type UndoOptions struct {
	Base string
}

type InitResult struct {
	BaseDir string
	Agents  []string
}

type MakeResult struct {
	Created  []string
	Guidance string
}

type SyncResult struct {
	RepoPath     string
	SymlinkCount int
}

type CleanResult struct {
	RepoPath     string
	SymlinkCount int
}

type UnlockResult struct {
	RepoPath string
}

type LockResult struct {
	RepoPath string
}

type RebaseResult struct {
	BaseRepoPath   string
	Rebased        []string
	Conflicted     []string
	SkippedDirty   []string
	SkippedCurrent []string
}

type WorkspaceListEntry struct {
	Name              string
	Path              string
	Branch            string
	LastCommitShort   string
	LastCommitMessage string
	Missing           bool
}

type ListResult struct {
	WorkspaceRoot string
	Entries       []WorkspaceListEntry
}

type UndoResult struct {
	WorkspaceRoot string
	BaseRepoPath  string
	BackupPath    string
	Steps         []string
}

func (r DoctorReport) HasCriticalPreflightFailure() bool {
	for _, check := range r.Checks {
		if (check.Name == "git" || check.Name == "cow_clone" || check.Name == "workspace") && !check.OK {
			return true
		}
	}
	return false
}

func NormalizeAgents(in []string) []string {
	if len(in) == 0 {
		return append([]string(nil), DefaultAgents...)
	}

	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, agent := range in {
		n := strings.TrimSpace(agent)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func ParseAgentsCSV(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	return NormalizeAgents(strings.Split(csv, ","))
}

func ValidateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "..") {
		return fmt.Errorf("invalid name %q: cannot contain '/' or '..'", trimmed)
	}
	return nil
}

func CanonicalBaseDir(branch string) string {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return "main"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	canonical := replacer.Replace(trimmed)
	if canonical == "" {
		return "main"
	}
	return canonical
}
