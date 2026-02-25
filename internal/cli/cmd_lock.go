package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newLockCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock .stooges base repo as read-only",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.Lock(cmd.Context(), model.LockOptions{Repo: repo})
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "locked %s\n", result.RepoPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Path to target repo")
	return cmd
}
