package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/prompt"
	"github.com/spf13/cobra"
)

func newUndoCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "undo",
		Aliases: []string{"remove"},
		Short:   "Remove workspace wrappers and restore base repo as project root",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				ok, err := prompt.ConfirmIO(streams.In, streams.Out, "Undo workspace layout (destructive, non-transactional)")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(streams.Out, "cancelled")
					return nil
				}
			}

			stop := startSpinner(streams.ErrOut, "Undoing workspace layout")
			result, err := svc.Undo(cmd.Context(), model.UndoOptions{})
			stop(err)
			for _, step := range result.Steps {
				fmt.Fprintf(streams.Out, "- %s\n", step)
			}
			if result.BackupPath != "" {
				fmt.Fprintf(streams.Out, "backup path: %s\n", result.BackupPath)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Out, "undo complete: restored %s\n", result.WorkspaceRoot)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
