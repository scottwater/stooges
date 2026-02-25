package interactive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/prompt"
)

type initBranchPreviewer interface {
	PreviewInitBranch(context.Context) (string, error)
}

func Run(ctx context.Context, svc engine.WorkspaceService, in io.Reader, out, errOut io.Writer) error {
	reader := bufio.NewReader(in)
	theme := newTheme(out)
	errTheme := newTheme(errOut)

	fmt.Fprintln(out, theme.renderBanner())
	doc, docErr := svc.Doctor(ctx, model.DoctorOptions{})
	if docErr != nil {
		fmt.Fprintln(errOut, errTheme.warning.Render("Safety check failed. Run `stooges doctor` for details."))
		return docErr
	} else if hasInitSuggestion(doc) {
		fmt.Fprintln(out, theme.hint.Render("No workspace setup yet. Run `stooges init` to cue up Larry, Curly, and Moe."))
	}

	for {
		selected, err := promptAction(reader, out, doc)
		if err != nil {
			fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Whoop-whoop. Invalid pick:"), err)
			continue
		}

		switch selected {
		case actionExit:
			fmt.Fprintln(out, theme.secondary.Render("Nyuk nyuk... signing off."))
			return nil
		case actionDoctor:
			report, err := svc.Doctor(ctx, model.DoctorOptions{})
			doc = report
			for _, check := range doc.Checks {
				label := "ok"
				if !check.OK {
					label = "warn"
				}
				fmt.Fprintf(out, "- %s %s (%s)\n", theme.menuNum.Render("["+label+"]"), check.Name, check.Message)
			}
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Doctor found issues:"), err)
			}
		case actionMake:
			workspaceName, err := promptInput(reader, out, "Workspace name (blank creates missing default workspaces): ")
			if err != nil {
				return err
			}
			source, err := promptInput(reader, out, "Source workspace [base]: ")
			if err != nil {
				return err
			}
			if source == "" {
				source = "base"
			}
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: add workspace=%q source=%q", workspaceName, source))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Make(ctx, model.MakeOptions{Agent: strings.TrimSpace(workspaceName), Source: source})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Add failed:"), err)
				continue
			}
			if len(result.Created) > 0 {
				fmt.Fprintf(out, "%s %s\n", theme.success.Render("Created workspaces:"), strings.Join(result.Created, ", "))
			}
			if result.Guidance != "" {
				fmt.Fprintln(out, theme.hint.Render(result.Guidance))
			}
			doc, _ = svc.Doctor(ctx, model.DoctorOptions{})
		case actionUnlock:
			repo, err := promptInput(reader, out, "Repo path (blank auto-resolve): ")
			if err != nil {
				return err
			}
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: unlock repo=%q", repo))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Unlock(ctx, model.UnlockOptions{Repo: repo})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Unlock failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s %s\n", theme.success.Render("Unlocked:"), result.RepoPath)
			doc, _ = svc.Doctor(ctx, model.DoctorOptions{})
		case actionSync:
			repo, err := promptInput(reader, out, "Repo path (blank auto-resolve): ")
			if err != nil {
				return err
			}
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: sync repo=%q", repo))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Sync(ctx, model.SyncOptions{Repo: repo})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Sync failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s %s (symlinks=%d)\n", theme.success.Render("Synced:"), result.RepoPath, result.SymlinkCount)
			doc, _ = svc.Doctor(ctx, model.DoctorOptions{})
		case actionClean:
			repo, err := promptInput(reader, out, "Repo path (blank auto-resolve): ")
			if err != nil {
				return err
			}
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: clean repo=%q", repo))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Clean(ctx, model.CleanOptions{Repo: repo})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Clean failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s %s (symlinks=%d)\n", theme.success.Render("Cleaned:"), result.RepoPath, result.SymlinkCount)
			doc, _ = svc.Doctor(ctx, model.DoctorOptions{})
		case actionLock:
			repo, err := promptInput(reader, out, "Repo path (blank auto-resolve): ")
			if err != nil {
				return err
			}
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: lock repo=%q", repo))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Lock(ctx, model.LockOptions{Repo: repo})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Lock failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s %s\n", theme.success.Render("Locked:"), result.RepoPath)
			doc, _ = svc.Doctor(ctx, model.DoctorOptions{})
		case actionRebase:
			repo, err := promptInput(reader, out, "Repo path (blank auto-resolve): ")
			if err != nil {
				return err
			}
			pruneInput, err := promptInput(reader, out, "Prune remote refs first? [y/N]: ")
			if err != nil {
				return err
			}
			prune := strings.EqualFold(strings.TrimSpace(pruneInput), "y") || strings.EqualFold(strings.TrimSpace(pruneInput), "yes")
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: rebase repo=%q prune=%t", repo, prune))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Rebase(ctx, model.RebaseOptions{Repo: repo, Prune: prune})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Rebase failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s base=%s rebased=%s conflicted=%s skipped-dirty=%s skipped-current=%s\n",
				theme.success.Render("Rebase complete."),
				result.BaseRepoPath,
				humanListOrNone(result.Rebased),
				humanListOrNone(result.Conflicted),
				humanListOrNone(result.SkippedDirty),
				humanListOrNone(result.SkippedCurrent),
			)
			doc, _ = svc.Doctor(ctx, model.DoctorOptions{})
		case actionUndo:
			ok, err := prompt.Confirm(reader, out, "Action: undo workspace layout (destructive, non-transactional)")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			result, err := svc.Undo(ctx, model.UndoOptions{})
			for _, step := range result.Steps {
				fmt.Fprintf(out, "- %s\n", step)
			}
			if result.BackupPath != "" {
				fmt.Fprintf(out, "backup path: %s\n", result.BackupPath)
			}
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Undo failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s %s\n", theme.success.Render("Undo complete:"), result.WorkspaceRoot)
			return nil
		case actionInit:
			mainBranchInput, err := promptInput(reader, out, "Main branch [main] (set only if not main): ")
			if err != nil {
				return err
			}
			workspaceName, err := promptInput(reader, out, fmt.Sprintf("Workspace name (blank creates workspaces for %s): ", humanList(model.DefaultAgents)))
			if err != nil {
				return err
			}
			workspaces := parseInteractiveWorkspaces(workspaceName)
			branchDisplay := resolveMainBranchDisplay(ctx, svc, mainBranchInput)
			workspaceDisplay := resolveWorkspaceDisplay(workspaces)
			ok, err := prompt.Confirm(reader, out, fmt.Sprintf("Action: init main-branch=%s workspaces=%s", branchDisplay, workspaceDisplay))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, theme.secondary.Render("Bonk. Cancelled."))
				continue
			}
			fmt.Fprintln(out, theme.hint.Render("Nyuk nyuk... setting up your stooge workspaces."))
			res, err := svc.Init(ctx, model.InitOptions{MainBranch: strings.TrimSpace(mainBranchInput), Agents: workspaces})
			if err != nil {
				fmt.Fprintf(errOut, "%s %v\n", errTheme.error.Render("Init failed:"), err)
				continue
			}
			fmt.Fprintf(out, "%s base=%s workspaces=%s\n", theme.success.Render("All set."), res.BaseDir, strings.Join(res.Agents, ", "))
			return nil
		}
	}
}

func parseInteractiveWorkspaces(nameInput string) []string {
	name := strings.TrimSpace(nameInput)
	if name == "" {
		return nil
	}
	return []string{name}
}

func resolveMainBranchDisplay(ctx context.Context, svc engine.WorkspaceService, input string) string {
	branch := strings.TrimSpace(input)
	if branch != "" {
		return fmt.Sprintf("%q (user)", branch)
	}

	previewer, ok := svc.(initBranchPreviewer)
	if !ok {
		return `"main" (default)`
	}
	detected, err := previewer.PreviewInitBranch(ctx)
	if err != nil {
		return `"main" (default)`
	}
	detected = strings.TrimSpace(detected)
	if detected == "" {
		return `"main" (default)`
	}
	return fmt.Sprintf("%q (default)", detected)
}

func resolveWorkspaceDisplay(workspaces []string) string {
	if len(workspaces) == 0 {
		return fmt.Sprintf("%s (default)", humanList(model.DefaultAgents))
	}
	return humanList(workspaces)
}

func humanList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return values[0]
	}
	if len(values) == 2 {
		return values[0] + " and " + values[1]
	}
	return strings.Join(values[:len(values)-1], ", ") + ", and " + values[len(values)-1]
}

func humanListOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return humanList(values)
}

func hasInitSuggestion(report model.DoctorReport) bool {
	for _, suggestion := range report.Suggestions {
		if strings.Contains(strings.ToLower(suggestion), "run `stooges init`") {
			return true
		}
	}
	return false
}
