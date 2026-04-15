package cli

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

const maxDerivedWorkspaceLen = 50

func newTrackCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	var source string
	var branch string
	var noCD bool

	cmd := &cobra.Command{
		Use:   "track <branch>",
		Short: "Add a workspace for a tracked remote branch",
		Long:  "Equivalent to `stooges add <derived-workspace> --track <branch>` with the workspace name derived from the track branch.",
		Args: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) == 0:
				return fmt.Errorf("accepts 1 arg(s), received 0")
			case len(args) == 1:
				return nil
			case len(args) == 2 && cmd.Flags().Changed("branch"):
				return nil
			default:
				return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			track := strings.TrimSpace(args[0])
			workspace, err := deriveTrackWorkspaceName(track)
			if err != nil {
				return err
			}
			branchName, branchAuto := resolveBranchSelection(cmd, args, branch)
			result, err := svc.Make(cmd.Context(), model.MakeOptions{
				Agent:      workspace,
				Source:     source,
				Track:      track,
				Branch:     branchName,
				BranchAuto: branchAuto,
			})
			if err != nil {
				return err
			}
			if err := writeAddCDTarget(result, noCD); err != nil {
				fmt.Fprintf(streams.ErrOut, "warning: unable to persist add cd target: %v\n", err)
			}
			if len(result.Created) > 0 {
				fmt.Fprintf(streams.Out, "created: %s\n", strings.Join(result.Created, ", "))
			}
			if result.Guidance != "" {
				fmt.Fprintln(streams.Out, result.Guidance)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "base", "Source workspace name (default: base/.stooges)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Optional local branch name while tracking the remote branch")
	cmd.Flags().BoolVar(&noCD, "no-cd", false, "Stay in the current directory even when shell integration is enabled")
	if branchFlag := cmd.Flags().Lookup("branch"); branchFlag != nil {
		branchFlag.NoOptDefVal = autoBranchSentinel
	}
	return cmd
}

func deriveTrackWorkspaceName(track string) (string, error) {
	trimmed := strings.TrimSpace(track)
	if trimmed == "" {
		return "", fmt.Errorf("track branch cannot be empty")
	}
	derived := deriveWorkspaceName(trimmed)
	if derived == "" {
		return "", fmt.Errorf("could not derive a workspace name from track branch %q", trimmed)
	}
	if derived == "base" {
		return "", fmt.Errorf("derived workspace name %q is reserved; choose a branch with a different suffix or use `stooges add <workspace> --track %s`", derived, trimmed)
	}
	return derived, nil
}

func deriveWorkspaceName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	candidate := trimmed
	if strings.Contains(trimmed, "/") {
		parts := strings.Split(trimmed, "/")
		candidate = ""
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(parts[i])
			if part == "" {
				continue
			}
			candidate = part
			break
		}
	}
	return sanitizeDerivedWorkspaceName(candidate)
}

func sanitizeDerivedWorkspaceName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	outLen := 0
	lastWasSeparator := false
	for _, r := range trimmed {
		if outLen >= maxDerivedWorkspaceLen {
			break
		}
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			outLen++
			lastWasSeparator = false
		case r == '-', r == '_':
			if outLen == 0 || lastWasSeparator {
				continue
			}
			b.WriteRune(r)
			outLen++
			lastWasSeparator = true
		default:
			if outLen == 0 || lastWasSeparator {
				continue
			}
			b.WriteByte('-')
			outLen++
			lastWasSeparator = true
		}
	}

	return strings.Trim(b.String(), "-_")
}
