package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

type currentWorkspaceResolver interface {
	ResolveCurrentWorkspace(context.Context) (engine.CurrentWorkspace, error)
}

func newForkCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var noCD bool

	cmd := &cobra.Command{
		Use:   "fork <branch>",
		Short: "Fork the current workspace into a new branched workspace",
		Long:  "Equivalent to `stooges add <derived-workspace> --source <current-workspace> --branch <branch>` using the current managed workspace as the source.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver, ok := svc.(currentWorkspaceResolver)
			if !ok {
				return fmt.Errorf("fork unavailable: workspace source resolver not implemented")
			}
			branch := strings.TrimSpace(args[0])
			workspace, err := deriveBranchWorkspaceName(branch)
			if err != nil {
				return err
			}
			current, err := resolver.ResolveCurrentWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			if err := ensureForkSourceSafe(current.Path); err != nil {
				return err
			}
			result, err := svc.Make(cmd.Context(), model.MakeOptions{
				Agent:      workspace,
				Source:     current.Name,
				Branch:     branch,
				BranchAuto: false,
			})
			if err != nil {
				return err
			}
			if err := writeAddCDTarget(result, noCD); err != nil {
				fmt.Fprintf(streams.ErrOut, "warning: unable to persist add cd target: %v\n", err)
			}
			if len(result.Created) > 0 {
				fmt.Fprintf(streams.Out, "created: %s\n", strings.Join(result.Created, ", "))
			}
			if result.Guidance != "" {
				fmt.Fprintln(streams.Out, result.Guidance)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noCD, "no-cd", false, "Stay in the current directory even when shell integration is enabled")
	return cmd
}

func ensureForkSourceSafe(repoPath string) error {
	checks := []struct {
		relPath string
		label   string
	}{
		{relPath: filepath.Join(".git", "rebase-merge"), label: "rebase in progress"},
		{relPath: filepath.Join(".git", "rebase-apply"), label: "rebase/apply in progress"},
		{relPath: filepath.Join(".git", "MERGE_HEAD"), label: "merge in progress"},
		{relPath: filepath.Join(".git", "CHERRY_PICK_HEAD"), label: "cherry-pick in progress"},
		{relPath: filepath.Join(".git", "REVERT_HEAD"), label: "revert in progress"},
		{relPath: filepath.Join(".git", "sequencer"), label: "git sequencer in progress"},
		{relPath: filepath.Join(".git", "index.lock"), label: "git index lock present"},
	}
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(repoPath, check.relPath)); err == nil {
			return fmt.Errorf("fork cannot run while the source workspace has %s", check.label)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check fork source git state: %w", err)
		}
	}
	return nil
}
