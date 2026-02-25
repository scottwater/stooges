package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newCleanCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Sync + prune remote-tracking refs, then relock read-only",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.Clean(cmd.Context(), model.CleanOptions{Repo: repo})
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "cleaned %s (symlinks=%d)\n", result.RepoPath, result.SymlinkCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Path to target repo")
	return cmd
}
