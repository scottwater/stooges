package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/interactive"
	"github.com/scottwater/stooges/internal/update"
	"github.com/scottwater/stooges/internal/version"
	"github.com/spf13/cobra"
)

type Streams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

type interactivePRInput struct {
	raw      io.Reader
	buffered *bufio.Reader
}

func (in interactivePRInput) Read(p []byte) (int, error) {
	if in.buffered == nil {
		return 0, io.EOF
	}
	return in.buffered.Read(p)
}

func (in interactivePRInput) RawReader() io.Reader {
	if in.raw != nil {
		return in.raw
	}
	return in.buffered
}

func NewRootCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	return newRootCmd(svc, streams, noopUpdater{}, newSystemGitHubPRClient(), defaultGitHubRepoResolver())
}

func NewRootCmdWithUpdater(svc engine.WorkspaceService, streams Streams, updaterClient Updater) *cobra.Command {
	if updaterClient == nil {
		updaterClient = noopUpdater{}
	}
	return newRootCmd(svc, streams, updaterClient, newSystemGitHubPRClient(), defaultGitHubRepoResolver())
}

type Updater interface {
	MaybeNotify(ctx context.Context, out io.Writer, currentVersion string) error
	Upgrade(ctx context.Context, currentVersion string) (update.UpgradeResult, error)
}

type noopUpdater struct{}

func (noopUpdater) MaybeNotify(context.Context, io.Writer, string) error {
	return nil
}

func (noopUpdater) Upgrade(context.Context, string) (update.UpgradeResult, error) {
	return update.UpgradeResult{}, fmt.Errorf("upgrade unavailable")
}

func skipsPassiveUpdateCheck(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "upgrade", "shell-init", "enabled", "stooged":
		return true
	default:
		return false
	}
}

func newRootCmd(svc engine.WorkspaceService, streams Streams, updaterClient Updater, gh githubPRClient, repoResolver githubRepoResolver) *cobra.Command {
	var showVersion bool

	cobra.EnablePrefixMatching = true

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
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if skipsPassiveUpdateCheck(cmd) {
				return
			}
			_ = updaterClient.MaybeNotify(cmd.Context(), streams.ErrOut, version.Value)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if showVersion {
				fmt.Fprintln(streams.Out, version.Value)
				return nil
			}
			return interactive.RunWithHooks(cmd.Context(), svc, streams.In, streams.Out, streams.ErrOut, interactive.Hooks{
				RunPR: func(ctx context.Context, reader *bufio.Reader, out, errOut io.Writer) error {
					return runPRAction(ctx, svc, Streams{
						In:     interactivePRInput{raw: streams.In, buffered: reader},
						Out:    out,
						ErrOut: errOut,
					}, gh, repoResolver, nil, "", false, false, false, false)
				},
			})
		},
	}

	root.Flags().BoolVar(&showVersion, "version", false, "Print installed version")
	root.AddCommand(newInitCmd(svc, streams))
	root.AddCommand(newMakeCmd(svc, streams))
	root.AddCommand(newTrackCmd(svc, streams))
	root.AddCommand(newPRCmd(svc, streams, gh, repoResolver))
	root.AddCommand(newBranchCmd(svc, streams))
	root.AddCommand(newForkCmd(svc, streams))
	root.AddCommand(newSyncCmd(svc, streams))
	root.AddCommand(newCleanCmd(svc, streams))
	root.AddCommand(newListCmd(svc, streams))
	root.AddCommand(newEnabledCmd(svc, streams))
	root.AddCommand(newStoogedCmd(svc, streams))
	root.AddCommand(newTrashCmd(svc, streams))
	root.AddCommand(newRebaseCmd(svc, streams))
	root.AddCommand(newUnlockCmd(svc, streams))
	root.AddCommand(newLockCmd(svc, streams))
	root.AddCommand(newUndoCmd(svc, streams))
	root.AddCommand(newDoctorCmd(svc, streams))
	root.AddCommand(newShellInitCmd(streams))
	root.AddCommand(newVersionCmd(streams))
	root.AddCommand(newUpgradeCmd(streams, updaterClient))

	return root
}
