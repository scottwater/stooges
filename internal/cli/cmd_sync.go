package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newSyncCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync .stooges base repo and relock read-only",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.Sync(cmd.Context(), model.SyncOptions{Repo: repo})
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "synced %s (symlinks=%d)\n", result.RepoPath, result.SymlinkCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Path to target repo")
	return cmd
}
