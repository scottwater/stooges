package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newUnlockCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock .stooges base repo for writing",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.Unlock(cmd.Context(), model.UnlockOptions{Repo: repo})
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "unlocked %s\n", result.RepoPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Path to target repo")
	return cmd
}
