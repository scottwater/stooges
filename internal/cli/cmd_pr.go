package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	term "github.com/charmbracelet/x/term"
	"github.com/scottwater/stooges/internal/engine"
	apperrors "github.com/scottwater/stooges/internal/errors"
	gitops "github.com/scottwater/stooges/internal/git"
	"github.com/scottwater/stooges/internal/model"
	"github.com/spf13/cobra"
)

type githubPRClient interface {
	EnsureAuth(context.Context, string) error
	ListOpen(context.Context, string) ([]githubPR, error)
	View(context.Context, string, int) (githubPR, error)
	Checkout(context.Context, string, int, string) error
}

type githubRepoResolver func(context.Context) (string, error)

type githubPR struct {
	Number            int            `json:"number"`
	Title             string         `json:"title"`
	HeadRefName       string         `json:"headRefName"`
	IsCrossRepository bool           `json:"isCrossRepository"`
	Author            githubPRAuthor `json:"author"`
}

type githubPRAuthor struct {
	Login string `json:"login"`
}

func (pr githubPR) authorLogin() string {
	if login := strings.TrimSpace(pr.Author.Login); login != "" {
		return login
	}
	return "unknown"
}

type systemGitHubPRClient struct{}

const (
	defaultPRPickerWidth = 96
	maxPRAuthorWidth     = 16
	minPRTitleWidth      = 20
)

var errNotGitRepo = errors.New("not a git repository")

func newSystemGitHubPRClient() githubPRClient {
	return systemGitHubPRClient{}
}

func (systemGitHubPRClient) EnsureAuth(ctx context.Context, repoPath string) error {
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found") {
		return apperrors.New(apperrors.KindInvalidInput, "`stooges pr` requires the GitHub CLI (`gh`) in PATH; install it and run `gh auth login`")
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	lowered := strings.ToLower(message)
	if strings.Contains(lowered, "gh auth login") || strings.Contains(lowered, "not logged into") || strings.Contains(lowered, "not authenticated") {
		return apperrors.New(apperrors.KindInvalidInput, "GitHub CLI is not authenticated; run `gh auth status` or `gh auth login` and try again")
	}
	return apperrors.Wrap(apperrors.KindGitFailure, "gh auth status", fmt.Errorf("%s", message))
}

func (systemGitHubPRClient) ListOpen(ctx context.Context, repoPath string) ([]githubPR, error) {
	output, err := runGitHubCLI(ctx, repoPath, "pr", "list", "--state", "open", "--limit", "100", "--json", "number,title,author,headRefName")
	if err != nil {
		return nil, err
	}
	var prs []githubPR
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, apperrors.Wrap(apperrors.KindGitFailure, "parse `gh pr list` output", err)
	}
	return prs, nil
}

func (systemGitHubPRClient) View(ctx context.Context, repoPath string, number int) (githubPR, error) {
	output, err := runGitHubCLI(ctx, repoPath, "pr", "view", strconv.Itoa(number), "--json", "number,title,author,headRefName,isCrossRepository")
	if err != nil {
		return githubPR{}, err
	}
	var pr githubPR
	if err := json.Unmarshal(output, &pr); err != nil {
		return githubPR{}, apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("parse `gh pr view %d` output", number), err)
	}
	return pr, nil
}

func (systemGitHubPRClient) Checkout(ctx context.Context, repoPath string, number int, branch string) error {
	args := []string{"pr", "checkout", strconv.Itoa(number)}
	if trimmed := strings.TrimSpace(branch); trimmed != "" {
		args = append(args, "--branch", trimmed)
	}
	_, err := runGitHubCLI(ctx, repoPath, args...)
	return err
}

func runGitHubCLI(ctx context.Context, repoPath string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, nil
	}
	if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found") {
		return nil, apperrors.New(apperrors.KindInvalidInput, "`stooges pr` requires the GitHub CLI (`gh`) in PATH; install it and run `gh auth login`")
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return nil, apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("gh %s", strings.Join(args, " ")), fmt.Errorf("%s", message))
}

func defaultGitHubRepoResolver() githubRepoResolver {
	resolver := &systemGitHubRepoLocator{cwd: os.Getwd, git: gitops.NewSystemOps()}
	return resolver.Resolve
}

type systemGitHubRepoLocator struct {
	cwd func() (string, error)
	git gitops.Ops
}

func (r *systemGitHubRepoLocator) Resolve(ctx context.Context) (string, error) {
	cwd, err := r.cwd()
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	repo, err := resolveGitHubRepoPath(ctx, cwd, r.git)
	if err == nil {
		return repo, nil
	}
	if errors.Is(err, errNotGitRepo) {
		return "", apperrors.New(apperrors.KindInvalidInput, "`stooges pr` must run from a git repo or a configured stooges workspace")
	}
	return "", err
}

func resolveGitHubRepoPath(ctx context.Context, cwd string, gitOps gitops.Ops) (string, error) {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return "", fmt.Errorf("cwd cannot be empty")
	}
	if gitOps != nil {
		repo, err := gitOps.TopLevel(ctx, trimmed)
		if err == nil {
			ok, repoErr := isGitRepoDir(repo)
			if repoErr != nil {
				return "", repoErr
			}
			if ok {
				return repo, nil
			}
			return "", errNotGitRepo
		}
		if !isNotGitRepoError(err) {
			return "", err
		}
	}

	current := filepath.Clean(trimmed)
	for {
		if filepath.Base(current) == ".stooges" {
			ok, repoErr := isGitRepoDir(current)
			if repoErr != nil {
				return "", repoErr
			}
			if ok {
				return current, nil
			}
		}
		candidate := filepath.Join(current, ".stooges")
		ok, repoErr := isGitRepoDir(candidate)
		if repoErr != nil {
			return "", repoErr
		}
		if ok {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errNotGitRepo
}

func isGitRepoDir(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, ".git"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("inspect git metadata for %s", path), err)
}

func isNotGitRepoError(err error) bool {
	if err == nil {
		return false
	}
	lowered := strings.ToLower(err.Error())
	return strings.Contains(lowered, "not a git repository") || strings.Contains(lowered, "not a git repo")
}

func newPRCmd(svc engine.WorkspaceService, streams Streams, gh githubPRClient, repoResolver githubRepoResolver) *cobra.Command {
	var branch string
	var noCD bool
	var noSetup bool
	var rollbackOnSetupFailure bool

	cmd := &cobra.Command{
		Use:   "pr [number]",
		Short: "Create a workspace for a GitHub pull request",
		Long:  "Look up a GitHub pull request with `gh`, create a workspace for it, and check out the PR branch in that workspace. With no argument, choose from open PRs in the current repository.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
			}
			if len(args) == 1 {
				if _, err := parsePRNumber(args[0]); err != nil {
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPRAction(cmd.Context(), svc, streams, gh, repoResolver, args, strings.TrimSpace(branch), noCD, noSetup, rollbackOnSetupFailure)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Optional local branch name to use for the PR checkout")
	cmd.Flags().BoolVar(&noCD, "no-cd", false, "Stay in the current directory even when shell integration is enabled")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "Skip the configured setup script for this PR workspace")
	cmd.Flags().BoolVar(&rollbackOnSetupFailure, "rollback-on-setup-failure", false, "Remove the created PR workspace if the setup script fails")
	return cmd
}

func runPRAction(ctx context.Context, svc engine.WorkspaceService, streams Streams, gh githubPRClient, repoResolver githubRepoResolver, args []string, branch string, noCD, noSetup, rollbackOnSetupFailure bool) error {
	repoPath, err := repoResolver(ctx)
	if err != nil {
		return err
	}
	if err := gh.EnsureAuth(ctx, repoPath); err != nil {
		return err
	}

	pr, err := resolvePullRequestSelection(ctx, streams, gh, repoPath, args)
	if err != nil {
		return err
	}

	workspace := derivePRWorkspaceName(pr)
	stop := startSpinner(streams.ErrOut, fmt.Sprintf("Creating workspace for PR #%d", pr.Number))
	result, err := checkoutPullRequestWorkspace(ctx, svc, gh, pr, workspace, strings.TrimSpace(branch), noSetup, rollbackOnSetupFailure)
	stop(err)
	if err != nil {
		return err
	}
	if err := writeAddCDTarget(result, noCD); err != nil {
		warnAutoCDFailure(streams, result, err)
	}
	if len(result.Created) > 0 {
		fmt.Fprintf(streams.Out, "created: %s\n", strings.Join(result.Created, ", "))
	}
	fmt.Fprintf(streams.Out, "checked out: PR #%d by %s — %s\n", pr.Number, pr.authorLogin(), pr.Title)
	if result.Guidance != "" {
		fmt.Fprintln(streams.Out, result.Guidance)
	}
	return nil
}

func parsePRNumber(input string) (int, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(input, "#"))
	if trimmed == "" {
		return 0, apperrors.New(apperrors.KindInvalidInput, "pull request number cannot be empty")
	}
	number, err := strconv.Atoi(trimmed)
	if err != nil || number <= 0 {
		return 0, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("invalid pull request number %q", input))
	}
	return number, nil
}

func resolvePullRequestSelection(ctx context.Context, streams Streams, gh githubPRClient, repoPath string, args []string) (githubPR, error) {
	if len(args) == 1 {
		number, err := parsePRNumber(args[0])
		if err != nil {
			return githubPR{}, err
		}
		return gh.View(ctx, repoPath, number)
	}

	prs, err := gh.ListOpen(ctx, repoPath)
	if err != nil {
		return githubPR{}, err
	}
	if len(prs) == 0 {
		return githubPR{}, apperrors.New(apperrors.KindInvalidInput, "no open pull requests found for this repository")
	}
	selection, err := selectPullRequest(streams.In, streams.Out, prs)
	if err != nil {
		return githubPR{}, err
	}
	return gh.View(ctx, repoPath, selection.Number)
}

func derivePRWorkspaceName(pr githubPR) string {
	derived := deriveWorkspaceName(pr.HeadRefName)
	if derived == "" || derived == "base" {
		return fmt.Sprintf("pr-%d", pr.Number)
	}
	return derived
}

func checkoutPullRequestWorkspace(ctx context.Context, svc engine.WorkspaceService, gh githubPRClient, pr githubPR, workspace, branch string, noSetup, rollbackOnSetupFailure bool) (model.MakeResult, error) {
	if !pr.IsCrossRepository && strings.TrimSpace(pr.HeadRefName) != "" {
		return svc.Make(ctx, model.MakeOptions{Agent: workspace, Source: "base", Track: strings.TrimSpace(pr.HeadRefName), Branch: branch, NoSetup: noSetup, RollbackOnSetupFailure: rollbackOnSetupFailure})
	}

	result, err := svc.Make(ctx, model.MakeOptions{Agent: workspace, Source: "base", NoSetup: true})
	if err != nil {
		return model.MakeResult{}, err
	}
	if len(result.Created) != 1 || strings.TrimSpace(result.WorkspaceRoot) == "" {
		return model.MakeResult{}, apperrors.New(apperrors.KindFilesystemFailure, "expected exactly one created workspace with a workspace root")
	}
	workspacePath := filepath.Join(result.WorkspaceRoot, result.Created[0])
	if err := gh.Checkout(ctx, workspacePath, pr.Number, branch); err != nil {
		rollbackErr := rollbackPartialPRWorkspace(ctx, svc, result.Created[0], workspacePath)
		if rollbackErr != nil {
			return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, fmt.Sprintf("gh pr checkout failed in %s and rollback failed", workspacePath), errors.Join(err, rollbackErr))
		}
		return model.MakeResult{}, wrapPRCheckoutFailure(workspacePath, err)
	}
	if !noSetup {
		if _, err := svc.Setup(ctx, model.SetupOptions{Workspace: result.Created[0], Source: "base", Branch: branch, RollbackOnSetupFailure: rollbackOnSetupFailure}); err != nil {
			return model.MakeResult{}, err
		}
	}
	return result, nil
}

type workspaceCreationRollbacker interface {
	RollbackWorkspaceCreation(context.Context, string) error
}

func rollbackPartialPRWorkspace(ctx context.Context, svc engine.WorkspaceService, workspace, workspacePath string) error {
	if rollbacker, ok := svc.(workspaceCreationRollbacker); ok {
		return rollbacker.RollbackWorkspaceCreation(ctx, workspace)
	}
	if err := os.RemoveAll(workspacePath); err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("remove partial workspace %s", workspacePath), err)
	}
	return nil
}

func wrapPRCheckoutFailure(workspacePath string, err error) error {
	message := fmt.Sprintf("gh pr checkout failed in %s; removed partial workspace and metadata", workspacePath)
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return apperrors.Wrap(appErr.Kind, message, err)
	}
	return apperrors.Wrap(apperrors.KindGitFailure, message, err)
}

type rawReaderProvider interface {
	RawReader() io.Reader
}

func selectPullRequest(in io.Reader, out io.Writer, prs []githubPR) (githubPR, error) {
	if len(prs) == 0 {
		return githubPR{}, apperrors.New(apperrors.KindInvalidInput, "no pull requests to choose from")
	}
	if fileIn, ok := ttyFileFromReader(in); ok && isTTYWriter(out) {
		fileOut := out.(*os.File)
		return selectPullRequestTTY(fileIn, fileOut, prs)
	}
	return selectPullRequestNumbered(in, out, prs)
}

func selectPullRequestNumbered(in io.Reader, out io.Writer, prs []githubPR) (githubPR, error) {
	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "Choose a pull request:")
	authorWidth := pullRequestAuthorWidth(prs)
	for i, pr := range prs {
		fmt.Fprintf(out, "  %d) %s\n", i+1, formatPullRequestLine(pr, defaultPRPickerWidth-5, authorWidth))
	}
	fmt.Fprint(out, "> ")
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return githubPR{}, err
	}
	selection := strings.TrimSpace(line)
	number, err := strconv.Atoi(selection)
	if err != nil || number < 1 || number > len(prs) {
		return githubPR{}, apperrors.New(apperrors.KindInvalidInput, "invalid pull request selection")
	}
	return prs[number-1], nil
}

func selectPullRequestTTY(in *os.File, out *os.File, prs []githubPR) (githubPR, error) {
	oldState, err := term.MakeRaw(in.Fd())
	if err != nil {
		return githubPR{}, apperrors.Wrap(apperrors.KindInvalidInput, "enable interactive PR picker", err)
	}
	defer term.Restore(in.Fd(), oldState)

	fmt.Fprint(out, "\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?25h")

	selected := 0
	renderPullRequestSelection(out, prs, selected)

	reader := bufio.NewReader(in)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return githubPR{}, err
		}
		switch b {
		case '\r', '\n':
			fmt.Fprint(out, "\r\n")
			return prs[selected], nil
		case 'k':
			if selected > 0 {
				selected--
				renderPullRequestSelection(out, prs, selected)
			}
		case 'j':
			if selected < len(prs)-1 {
				selected++
				renderPullRequestSelection(out, prs, selected)
			}
		case 3, 'q':
			return githubPR{}, apperrors.New(apperrors.KindInvalidInput, "pull request selection cancelled")
		case 27:
			next, _ := reader.ReadByte()
			if next != '[' {
				continue
			}
			dir, _ := reader.ReadByte()
			switch dir {
			case 'A':
				if selected > 0 {
					selected--
					renderPullRequestSelection(out, prs, selected)
				}
			case 'B':
				if selected < len(prs)-1 {
					selected++
					renderPullRequestSelection(out, prs, selected)
				}
			}
		}
	}
}

func renderPullRequestSelection(out io.Writer, prs []githubPR, selected int) {
	fmt.Fprint(out, "\x1b[2J\x1b[H")
	fmt.Fprint(out, "Choose a pull request (↑/↓, j/k, Enter):\r\n\r\n")
	authorWidth := pullRequestAuthorWidth(prs)
	availableWidth := defaultPRPickerWidth
	if fileOut, ok := out.(*os.File); ok {
		if width, _, err := term.GetSize(fileOut.Fd()); err == nil && width > 0 {
			availableWidth = width - 2
		}
	}
	for i, pr := range prs {
		prefix := "  "
		line := formatPullRequestLine(pr, availableWidth-2, authorWidth)
		if i == selected {
			prefix = "❯ "
			line = "\x1b[1;36m" + line + "\x1b[0m"
		}
		fmt.Fprintf(out, "%s%s\r\n", prefix, line)
	}
}
func formatPullRequestLine(pr githubPR, maxWidth, authorWidth int) string {
	if authorWidth < 6 {
		authorWidth = 6
	}
	numberPart := fmt.Sprintf("#%d", pr.Number)
	authorPart := truncateDisplay(pr.authorLogin(), authorWidth)
	fixedWidth := len(numberPart) + 2 + authorWidth + 2
	titleWidth := maxWidth - fixedWidth
	if titleWidth < minPRTitleWidth {
		titleWidth = minPRTitleWidth
	}
	titlePart := truncateDisplay(strings.TrimSpace(pr.Title), titleWidth)
	return fmt.Sprintf("%-6s  %-*s  %s", numberPart, authorWidth, authorPart, titlePart)
}

func pullRequestAuthorWidth(prs []githubPR) int {
	width := 6
	for _, pr := range prs {
		width = max(width, len([]rune(pr.authorLogin())))
	}
	if width > maxPRAuthorWidth {
		return maxPRAuthorWidth
	}
	return width
}

func truncateDisplay(value string, maxWidth int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxWidth {
		return trimmed
	}
	if maxWidth == 1 {
		return "…"
	}
	return string(runes[:maxWidth-1]) + "…"
}

func rawInputReader(in io.Reader) io.Reader {
	if provider, ok := in.(rawReaderProvider); ok {
		if raw := provider.RawReader(); raw != nil {
			return raw
		}
	}
	return in
}

func ttyFileFromReader(in io.Reader) (*os.File, bool) {
	file, ok := rawInputReader(in).(*os.File)
	if !ok {
		return nil, false
	}
	info, err := file.Stat()
	if err != nil {
		return nil, false
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		return nil, false
	}
	return file, true
}

func isTTYReader(in io.Reader) bool {
	_, ok := ttyFileFromReader(in)
	return ok
}
