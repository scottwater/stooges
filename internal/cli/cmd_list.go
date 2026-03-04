package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

const maxListCommitMessageLen = 72

func newListCmd(svc engine.WorkspaceService, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List base and managed workspaces",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := svc.List(cmd.Context(), model.ListOptions{})
			if err != nil {
				return err
			}
			printWorkspaceList(streams.Out, result)
			return nil
		},
	}
}

func printWorkspaceList(out io.Writer, result model.ListResult) {
	fmt.Fprintf(out, "workspace: %s\n", result.WorkspaceRoot)
	if len(result.Entries) == 0 {
		fmt.Fprintln(out, "no workspaces found")
		return
	}

	headerWorkspace := "workspace"
	headerBranch := "branch"
	headerSHA := "sha"
	headerCommit := "last commit"

	rows := make([]model.WorkspaceListEntry, 0, len(result.Entries))
	nameWidth := len(headerWorkspace)
	branchWidth := len(headerBranch)
	shaWidth := len(headerSHA)
	for _, entry := range result.Entries {
		row := entry
		row.LastCommitMessage = truncateListCommitMessage(row.LastCommitMessage)
		rows = append(rows, row)
		nameWidth = max(nameWidth, len(row.Name))
		branchWidth = max(branchWidth, len(row.Branch))
		shaWidth = max(shaWidth, len(row.LastCommitShort))
	}

	var headerStyle lipgloss.Style
	var baseStyle lipgloss.Style
	if isTTYWriter(out) {
		r := lipgloss.NewRenderer(out)
		headerStyle = r.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
		baseStyle = r.NewStyle().Bold(true).Foreground(lipgloss.Color("120"))
	}

	headerLine := fmt.Sprintf("%-*s  %-*s  %-*s  %s", nameWidth, headerWorkspace, branchWidth, headerBranch, shaWidth, headerSHA, headerCommit)
	fmt.Fprintln(out, headerStyle.Render(headerLine))
	for _, row := range rows {
		line := fmt.Sprintf("%-*s  %-*s  %-*s  %s", nameWidth, row.Name, branchWidth, row.Branch, shaWidth, row.LastCommitShort, row.LastCommitMessage)
		switch {
		case row.Name == "base":
			fmt.Fprintln(out, baseStyle.Render(line))
		default:
			fmt.Fprintln(out, line)
		}
	}
}

func truncateListCommitMessage(msg string) string {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return "-"
	}
	if len(trimmed) <= maxListCommitMessageLen {
		return trimmed
	}
	return trimmed[:maxListCommitMessageLen-3] + "..."
}
