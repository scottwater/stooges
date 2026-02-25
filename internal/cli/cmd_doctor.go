package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

func newDoctorCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var repo string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check platform support and workspace health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := svc.Doctor(cmd.Context(), model.DoctorOptions{Repo: repo})
			if jsonOut {
				enc := json.NewEncoder(streams.Out)
				enc.SetIndent("", "  ")
				if encErr := enc.Encode(report); encErr != nil {
					if err != nil {
						return errors.Join(err, encErr)
					}
					return encErr
				}
			} else {
				for _, check := range report.Checks {
					fmt.Fprintf(streams.Out, "- %s: %v (%s)\n", check.Name, check.OK, check.Message)
				}
				if len(report.Suggestions) > 0 {
					fmt.Fprintln(streams.Out, "Suggestions:")
					for _, suggestion := range report.Suggestions {
						fmt.Fprintf(streams.Out, "  - %s\n", suggestion)
					}
				}
			}
			return err
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Path to target repo")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output doctor report as JSON")
	return cmd
}
