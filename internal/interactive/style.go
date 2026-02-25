package interactive

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

type uiTheme struct {
	banner    lipgloss.Style
	subtitle  lipgloss.Style
	hint      lipgloss.Style
	warning   lipgloss.Style
	error     lipgloss.Style
	success   lipgloss.Style
	title     lipgloss.Style
	menuNum   lipgloss.Style
	prompt    lipgloss.Style
	secondary lipgloss.Style
}

func newTheme(out io.Writer) uiTheme {
	r := lipgloss.NewRenderer(out)
	return uiTheme{
		banner:    r.NewStyle().Bold(true).Foreground(lipgloss.Color("213")),
		subtitle:  r.NewStyle().Foreground(lipgloss.Color("117")),
		hint:      r.NewStyle().Foreground(lipgloss.Color("150")),
		warning:   r.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		error:     r.NewStyle().Bold(true).Foreground(lipgloss.Color("203")),
		success:   r.NewStyle().Bold(true).Foreground(lipgloss.Color("120")),
		title:     r.NewStyle().Bold(true).Foreground(lipgloss.Color("229")),
		menuNum:   r.NewStyle().Foreground(lipgloss.Color("111")),
		prompt:    r.NewStyle().Bold(true).Foreground(lipgloss.Color("111")),
		secondary: r.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

func (t uiTheme) renderBanner() string {
	title := t.banner.Render("STOOGES")
	sub := t.subtitle.Render("Larry • Curly • Moe workspace wrangler")
	return fmt.Sprintf("%s\n%s", title, sub)
}
