package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/version"
	"github.com/spf13/cobra"
)

func newUpgradeCmd(streams Streams, updaterClient Updater) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade stooges to the latest GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := updaterClient.Upgrade(cmd.Context(), version.Value)
			if err != nil {
				return err
			}
			if result.UpToDate {
				fmt.Fprintf(streams.Out, "stooges is already up to date (%s)\n", updateDisplayVersion(result.LatestVersion))
				return nil
			}
			fmt.Fprintf(streams.Out, "upgraded stooges from %s to %s\n", updateDisplayVersion(result.CurrentVersion), updateDisplayVersion(result.LatestVersion))
			fmt.Fprintf(streams.Out, "binary: %s\n", result.ExecutablePath)
			return nil
		},
	}
}

func updateDisplayVersion(v string) string {
	if v == "" {
		return ""
	}
	if v[0] == 'v' {
		return v
	}
	return "v" + v
}
