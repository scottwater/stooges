package cli

import (
	"fmt"
	"strings"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newMakeCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var source string
	var branch string
	const autoBranchSentinel = "__stooges_auto_branch__"

	cmd := &cobra.Command{
		Use:   "add [workspace]",
		Short: "Add one or more workspaces",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) <= 1 {
				return nil
			}
			branchChanged := cmd.Flags().Changed("branch")
			if len(args) == 2 && branchChanged {
				return nil
			}
			return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) >= 1 {
				workspace = args[0]
			}
			branchChanged := cmd.Flags().Changed("branch")
			branchValue := branch
			if len(args) == 2 && branchChanged && branch == autoBranchSentinel {
				branchValue = strings.TrimSpace(args[1])
			}
			branchAuto := branchChanged && (branchValue == autoBranchSentinel || strings.TrimSpace(branchValue) == "")
			branchName := strings.TrimSpace(branch)
			if branchValue != "" {
				branchName = strings.TrimSpace(branchValue)
			}
			if branchName == autoBranchSentinel {
				branchName = ""
			}
			result, err := svc.Make(cmd.Context(), model.MakeOptions{
				Agent:      workspace,
				Source:     source,
				Branch:     branchName,
				BranchAuto: branchAuto,
			})
			if err != nil {
				return err
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
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Optional branch to checkout/create in new workspace (`-b` uses workspace name)")
	if branchFlag := cmd.Flags().Lookup("branch"); branchFlag != nil {
		branchFlag.NoOptDefVal = autoBranchSentinel
	}
	return cmd
}
