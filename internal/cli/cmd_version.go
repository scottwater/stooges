package cli

import (
	"fmt"

	"github.com/scottwater/stooges/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print installed version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintln(streams.Out, version.Value)
		},
	}
}
