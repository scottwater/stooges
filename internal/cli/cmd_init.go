package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/prompt"
	"github.com/spf13/cobra"
)

type initBranchPreviewer interface {
	PreviewInitBranch(context.Context) (string, error)
}

func newInitCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var mainBranch string
	var agentsCSV string
	var workspaces []string
	var confirm bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .stooges base repo and agent workspaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			agents := normalizeInitAgents(agentsCSV, workspaces)
			if !confirm {
				branchDisplay := resolveInitMainBranchDisplay(cmd.Context(), svc, mainBranch)
				ok, err := prompt.ConfirmIO(streams.In, streams.Out, fmt.Sprintf("Initialize workspace with main-branch=%s workspaces=%s", branchDisplay, strings.Join(model.NormalizeAgents(agents), ",")))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(streams.Out, "cancelled")
					return nil
				}
			}
			stop := startSpinner(streams.ErrOut, "Initializing workspace")
			result, err := svc.Init(cmd.Context(), model.InitOptions{MainBranch: strings.TrimSpace(mainBranch), Agents: agents})
			stop(err)
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "initialized base=%s workspaces=%s\n", result.BaseDir, strings.Join(result.Agents, ", "))
			return nil
		},
	}

	cmd.Flags().StringVarP(&mainBranch, "main-branch", "m", "", "Base branch name override (supports master; defaults to main)")
	cmd.Flags().StringVar(&agentsCSV, "agents", "", "Comma-separated agent list (default: larry,curly,moe)")
	cmd.Flags().StringArrayVar(&workspaces, "workspace", nil, "Workspace name (repeatable)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Skip init confirmation prompt")
	return cmd
}

func normalizeInitAgents(agentsCSV string, workspaces []string) []string {
	combined := append([]string{}, model.ParseAgentsCSV(agentsCSV)...)
	combined = append(combined, workspaces...)
	return model.NormalizeAgents(combined)
}

func resolveInitMainBranchDisplay(ctx context.Context, svc engine.WorkspaceService, input string) string {
	mainBranch := strings.TrimSpace(input)
	if mainBranch != "" {
		return fmt.Sprintf("%q (user)", mainBranch)
	}
	previewer, ok := svc.(initBranchPreviewer)
	if !ok {
		return `"main" (default)`
	}
	detected, err := previewer.PreviewInitBranch(ctx)
	if err != nil {
		return `"main" (default)`
	}
	detected = strings.TrimSpace(detected)
	if detected == "" {
		return `"main" (default)`
	}
	return fmt.Sprintf("%q (default)", detected)
}
