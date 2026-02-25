package cli

import (
	"fmt"
	"strings"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newRebaseCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var repo string
	var prune bool

	cmd := &cobra.Command{
		Use:   "rebase",
		Short: "Sync base repo, then rebase workspace branches onto it when conflict-free",
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := svc.Rebase(cmd.Context(), model.RebaseOptions{Repo: repo, Prune: prune})
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "base synced: %s\n", res.BaseRepoPath)
			if len(res.Rebased) > 0 {
				fmt.Fprintf(streams.Out, "rebased: %s\n", strings.Join(res.Rebased, ", "))
			}
			if len(res.SkippedDirty) > 0 {
				fmt.Fprintf(streams.Out, "dirty (skipped): %s\n", strings.Join(res.SkippedDirty, ", "))
			}
			if len(res.SkippedCurrent) > 0 {
				fmt.Fprintf(streams.Out, "already current (skipped): %s\n", strings.Join(res.SkippedCurrent, ", "))
			}
			if len(res.Conflicted) > 0 {
				fmt.Fprintf(streams.Out, "conflicts (manual rebase needed): %s\n", strings.Join(res.Conflicted, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Path to target base repo")
	cmd.Flags().BoolVar(&prune, "prune", false, "Use fetch --prune during sync")
	return cmd
}
