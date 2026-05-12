package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newTrashCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "trash <workspace>",
		Short: "Move a managed workspace to Trash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := svc.Trash(cmd.Context(), model.TrashOptions{Workspace: args[0], Force: force})
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "trashed workspace=%s path=%s removal=%s teardown=%s\n", result.Workspace, result.WorkspacePath, result.Removal, result.Teardown)
			if result.TeardownError != "" {
				fmt.Fprintf(streams.ErrOut, "warning: teardown failed before forced removal: %s\n", result.TeardownError)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Permanently delete when the trash command is unavailable")
	return cmd
}
