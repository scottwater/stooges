package cli

import (
	"encoding/json"
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newEnabledCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "enabled",
		Short: "Check whether the current directory is inside a configured Stooges workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.Enabled(cmd.Context(), model.EnabledOptions{})
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(streams.Out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return err
				}
			} else if result.Enabled {
				fmt.Fprintln(streams.Out, "enabled")
			} else {
				fmt.Fprintln(streams.Out, "not enabled")
			}
			if !result.Enabled {
				return errNotEnabled
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output enabled status as JSON")
	return cmd
}

func newStoogedCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "stooged",
		Short: "Print yes when the current directory is inside a configured Stooges workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.Enabled(cmd.Context(), model.EnabledOptions{})
			if err != nil {
				return err
			}
			if result.Enabled {
				fmt.Fprintln(streams.Out, "yes")
				return nil
			}
			fmt.Fprintln(streams.Out, "no")
			return errNotEnabled
		},
	}
}
