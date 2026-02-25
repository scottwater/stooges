package interactive

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/scottwater/stooges/internal/model"
)

type action string

const (
	actionInit   action = "init"
	actionMake   action = "make"
	actionSync   action = "sync"
	actionClean  action = "clean"
	actionUnlock action = "unlock"
	actionLock   action = "lock"
	actionRebase action = "rebase"
	actionUndo   action = "undo"
	actionDoctor action = "doctor"
	actionExit   action = "exit"
)

type menuEntry struct {
	label  string
	action action
}

func promptAction(reader *bufio.Reader, out io.Writer, report model.DoctorReport) (action, error) {
	menu := buildMenu(report)
	theme := newTheme(out)

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, theme.title.Render("Choose action:"))
	for i, entry := range menu {
		fmt.Fprintf(out, "  %s %s\n", theme.menuNum.Render(fmt.Sprintf("%d)", i+1)), entry.label)
	}
	fmt.Fprintf(out, "  %s %s\n", theme.menuNum.Render("0)"), "exit")
	fmt.Fprint(out, theme.prompt.Render("> "))

	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	input := strings.TrimSpace(strings.ToLower(line))
	if input == "0" || input == "q" {
		return actionExit, nil
	}

	n, parseErr := strconv.Atoi(input)
	if parseErr != nil || n < 1 || n > len(menu) {
		return "", fmt.Errorf("invalid selection")
	}
	return menu[n-1].action, nil
}

func buildMenu(report model.DoctorReport) []menuEntry {
	if report.HasCriticalPreflightFailure() {
		return []menuEntry{
			{label: "doctor", action: actionDoctor},
		}
	}

	if isUnconfiguredWorkspace(report) {
		return []menuEntry{
			{label: "init", action: actionInit},
			{label: "doctor", action: actionDoctor},
		}
	}

	return []menuEntry{
		{label: "add workspace", action: actionMake},
		{label: "sync", action: actionSync},
		{label: "clean", action: actionClean},
		{label: "unlock", action: actionUnlock},
		{label: "lock", action: actionLock},
		{label: "rebase", action: actionRebase},
		{label: "undo workspace layout", action: actionUndo},
		{label: "doctor", action: actionDoctor},
	}
}

func isUnconfiguredWorkspace(report model.DoctorReport) bool {
	for _, check := range report.Checks {
		if check.Name == "repo_resolution" && strings.Contains(strings.ToLower(check.Message), "not configured yet") {
			return true
		}
	}
	return false
}
