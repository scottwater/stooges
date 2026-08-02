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

const autoBranchSentinel = "__stooges_auto_branch__"

func branchAllowsExtraPositionalArg(cmd *cobra.Command, branch string) bool {
	return cmd.Flags().Changed("branch") && branch == autoBranchSentinel
}

func resolveBranchSelection(cmd *cobra.Command, args []string, branch string) (branchName string, branchAuto bool) {
	branchChanged := cmd.Flags().Changed("branch")
	branchValue := branch
	if len(args) == 2 && branchChanged && branch == autoBranchSentinel {
		branchValue = strings.TrimSpace(args[1])
	}
	branchAuto = branchChanged && (branchValue == autoBranchSentinel || strings.TrimSpace(branchValue) == "")
	branchName = strings.TrimSpace(branchValue)
	if branchName == autoBranchSentinel {
		branchName = ""
	}
	return branchName, branchAuto
}

func sourceUsesBase(source string) bool {
	switch strings.TrimSpace(source) {
	case "", "base", "main", "master", ".stooges":
		return true
	default:
		return false
	}
}

func maybeSyncBaseSource(ctx context.Context, svc engine.WorkspaceService, source string, noSync bool) error {
	if noSync || !sourceUsesBase(source) {
		return nil
	}
	_, err := svc.Sync(ctx, model.SyncOptions{})
	return err
}

func newMakeCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var source string
	var branch string
	var track string
	var noCD bool
	var noSync bool
	var noSetup bool
	var rollbackOnSetupFailure bool

	cmd := &cobra.Command{
		Use:   "add [workspace]",
		Short: "Add one or more workspaces",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) <= 1 {
				return nil
			}
			if len(args) == 2 && branchAllowsExtraPositionalArg(cmd, branch) {
				return nil
			}
			return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) >= 1 {
				workspace = args[0]
			}
			branchName, branchAuto := resolveBranchSelection(cmd, args, branch)
			if strings.TrimSpace(track) == "" {
				if err := maybeSyncBaseSource(cmd.Context(), svc, source, noSync); err != nil {
					return err
				}
			}
			result, err := svc.Make(cmd.Context(), model.MakeOptions{
				Agent:                  workspace,
				Source:                 source,
				Track:                  track,
				Branch:                 branchName,
				BranchAuto:             branchAuto,
				NoSetup:                noSetup,
				RollbackOnSetupFailure: rollbackOnSetupFailure,
			})
			if err != nil {
				return err
			}
			if err := writeAddCDTarget(result, noCD); err != nil {
				warnAutoCDFailure(streams, result, err)
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
	cmd.Flags().StringVar(&track, "track", "", "Track remote branch in new workspace (fails when origin/<branch> is missing or the local branch already exists)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Optional branch to checkout/create in new workspace (`-b` uses workspace name)")
	cmd.Flags().BoolVar(&noCD, "no-cd", false, "Stay in the current directory even when shell integration is enabled")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "Skip the automatic base sync before creating workspaces from base")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "Skip the configured setup script for this run")
	cmd.Flags().BoolVar(&rollbackOnSetupFailure, "rollback-on-setup-failure", false, "Remove created workspace(s) if the setup script fails")
	if branchFlag := cmd.Flags().Lookup("branch"); branchFlag != nil {
		branchFlag.NoOptDefVal = autoBranchSentinel
	}
	return cmd
}

func writeAddCDTarget(result model.MakeResult, noCD bool) error {
	if noCD || envDisablesAutoCD() || len(result.Created) != 1 || strings.TrimSpace(result.WorkspaceRoot) == "" {
		return nil
	}
	targetFile := strings.TrimSpace(os.Getenv("STOOGES_CD_FILE"))
	if targetFile == "" {
		return nil
	}
	targetDir := filepath.Join(result.WorkspaceRoot, result.Created[0])
	return os.WriteFile(targetFile, []byte(targetDir), 0o600)
}

func envDisablesAutoCD() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("STOOGES_NO_CD"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func warnAutoCDFailure(streams Streams, result model.MakeResult, err error) {
	targetFile := strings.TrimSpace(os.Getenv("STOOGES_CD_FILE"))
	workspaceName := ""
	workspacePath := ""
	if len(result.Created) == 1 {
		workspaceName = strings.TrimSpace(result.Created[0])
		if strings.TrimSpace(result.WorkspaceRoot) != "" {
			workspacePath = filepath.Join(result.WorkspaceRoot, workspaceName)
		}
	}

	switch {
	case workspaceName != "" && targetFile != "" && workspacePath != "":
		fmt.Fprintf(streams.ErrOut, "warning: workspace %q was created, but auto-cd failed: could not write %q for %q: %v\n", workspaceName, targetFile, workspacePath, err)
	case targetFile != "":
		fmt.Fprintf(streams.ErrOut, "warning: workspace creation succeeded, but auto-cd failed: could not write %q: %v\n", targetFile, err)
	default:
		fmt.Fprintf(streams.ErrOut, "warning: workspace creation succeeded, but auto-cd failed: %v\n", err)
	}
}
