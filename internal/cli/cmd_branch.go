package cli

import (
	"fmt"
	"strings"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newBranchCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var source string
	var noCD bool
	var noSync bool

	cmd := &cobra.Command{
		Use:   "branch <branch>",
		Short: "Add a workspace for a local branch",
		Long:  "Equivalent to `stooges add <derived-workspace> -b <branch>` with the workspace name derived from the branch name.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := strings.TrimSpace(args[0])
			workspace, err := deriveBranchWorkspaceName(branch)
			if err != nil {
				return err
			}
			if err := maybeSyncBaseSource(cmd.Context(), svc, source, noSync); err != nil {
				return err
			}
			result, err := svc.Make(cmd.Context(), model.MakeOptions{
				Agent:      workspace,
				Source:     source,
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

	cmd.Flags().StringVar(&source, "source", "base", "Source workspace name (default: base/.stooges)")
	cmd.Flags().BoolVar(&noCD, "no-cd", false, "Stay in the current directory even when shell integration is enabled")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "Skip the automatic base sync before creating a workspace from base")
	return cmd
}

func deriveBranchWorkspaceName(branch string) (string, error) {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return "", fmt.Errorf("branch name cannot be empty")
	}
	derived := deriveWorkspaceName(trimmed)
	if derived == "" {
		return "", fmt.Errorf("could not derive a workspace name from branch %q", trimmed)
	}
	if derived == "base" {
		return "", fmt.Errorf("derived workspace name %q is reserved; choose a branch with a different suffix or use `stooges add <workspace> -b %s`", derived, trimmed)
	}
	return derived, nil
}
