package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/interactive"
	"github.com/scottwater/stooges/internal/version"
	"github.com/spf13/cobra"
)

type Streams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

func NewRootCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var showVersion bool

	if streams.In == nil {
		streams.In = os.Stdin
	}
	if streams.Out == nil {
		streams.Out = os.Stdout
	}
	if streams.ErrOut == nil {
		streams.ErrOut = os.Stderr
	}

	root := &cobra.Command{
		Use:           "stooges",
		Short:         "Unified AFS workspace CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if showVersion {
				fmt.Fprintln(streams.Out, version.Value)
				return nil
			}
			return interactive.Run(cmd.Context(), svc, streams.In, streams.Out, streams.ErrOut)
		},
	}

	root.Flags().BoolVar(&showVersion, "version", false, "Print installed version")
	root.AddCommand(newInitCmd(svc, streams))
	root.AddCommand(newMakeCmd(svc, streams))
	root.AddCommand(newSyncCmd(svc, streams))
	root.AddCommand(newCleanCmd(svc, streams))
	root.AddCommand(newListCmd(svc, streams))
	root.AddCommand(newRebaseCmd(svc, streams))
	root.AddCommand(newUnlockCmd(svc, streams))
	root.AddCommand(newLockCmd(svc, streams))
	root.AddCommand(newUndoCmd(svc, streams))
	root.AddCommand(newDoctorCmd(svc, streams))
	root.AddCommand(newVersionCmd(streams))

	return root
}
