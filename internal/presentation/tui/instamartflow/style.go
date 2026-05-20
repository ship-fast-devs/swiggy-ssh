package instamartflow

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type Viewport struct {
	Width  int
	Height int
}

type KeyHint struct {
	Key   string
	Label string
}

var (
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019"))
	brandStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019")).Bold(true)
	creamStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF7ED"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA44")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	boldStyle    = lipgloss.NewStyle().Bold(true)
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019")).Bold(true)
	yamlKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019"))
	yamlValStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF7ED"))
	diffAddStyle = lipgloss.NewStyle().Background(lipgloss.Color("#103D22"))
	diffDelStyle = lipgloss.NewStyle().Background(lipgloss.Color("#4A1717"))
)

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

const innerWidth = 78

func top() string {
	return "┌" + strings.Repeat("─", innerWidth) + "┐\r\n"
}

func bottom() string {
	return "└" + strings.Repeat("─", innerWidth) + "┘\r\n"
}

func divider() string {
	return "├" + strings.Repeat("─", innerWidth) + "┤\r\n"
}

func line(content string) string {
	inner := " " + content
	w := lipgloss.Width(inner)
	if w > innerWidth {
		runes := []rune(inner)
		inner = string(runes[:innerWidth])
		w = innerWidth
	}
	inner += strings.Repeat(" ", innerWidth-w)
	return "│" + inner + "│\r\n"
}

func centeredLine(content string) string {
	w := lipgloss.Width(content)
	if w > innerWidth {
		runes := []rune(content)
		content = string(runes[:innerWidth])
		w = innerWidth
	}

	leftPad := (innerWidth - w) / 2
	rightPad := innerWidth - w - leftPad
	return "│" + strings.Repeat(" ", leftPad) + content + strings.Repeat(" ", rightPad) + "│\r\n"
}

func rightLine(content string) string {
	w := lipgloss.Width(content)
	if w > innerWidth {
		runes := []rune(content)
		content = string(runes[:innerWidth])
		w = innerWidth
	}
	return "│" + strings.Repeat(" ", innerWidth-w) + content + "│\r\n"
}

func footerLine(hints ...KeyHint) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		if hint.Key == "" || hint.Label == "" {
			continue
		}
		parts = append(parts, hint.Key+" "+hint.Label)
	}
	return centeredLine(mutedStyle.Render(strings.Join(parts, "    ")))
}

func headerLine(left, right string) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	spaces := innerWidth - leftW - rightW
	if spaces < 1 {
		spaces = 1
	}
	return "│" + left + strings.Repeat(" ", spaces) + right + "│\r\n"
}

func centerInViewport(content string, viewport Viewport) string {
	if viewport.Width <= 0 && viewport.Height <= 0 {
		return content
	}

	trimmed := strings.TrimSuffix(content, "\r\n")
	if trimmed == "" {
		return content
	}

	lines := strings.Split(trimmed, "\r\n")
	contentWidth := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > contentWidth {
			contentWidth = w
		}
	}

	leftPad := 0
	if viewport.Width > contentWidth {
		leftPad = (viewport.Width - contentWidth) / 2
	}
	topPad := 0
	if viewport.Height > len(lines) {
		topPad = (viewport.Height - len(lines)) / 2
	}

	var sb strings.Builder
	for i := 0; i < topPad; i++ {
		sb.WriteString("\r\n")
	}
	pad := strings.Repeat(" ", leftPad)
	for _, l := range lines {
		sb.WriteString(pad)
		sb.WriteString(l)
		sb.WriteString("\r\n")
	}
	return sb.String()
}

func ctxQuitCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return tea.QuitMsg{}
	}
}

type staticModel struct{ view string }

func (m staticModel) Init() tea.Cmd                           { return tea.Quit }
func (m staticModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m staticModel) View() string                            { return m.view }

func runStatic(w io.Writer, content string) error {
	p := tea.NewProgram(staticModel{view: content},
		tea.WithOutput(w),
		tea.WithInput(strings.NewReader("")),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}

func runInteractive(m tea.Model, w io.Writer, in io.Reader) (tea.Model, error) {
	if in == nil {
		in = strings.NewReader("")
	}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
		tea.WithAltScreen(),
	)
	return p.Run()
}

func gradientColor(colors []string, index, total int) string {
	if len(colors) == 1 || total <= 1 {
		return colors[0]
	}
	if index < 0 {
		index = 0
	}
	if index >= total {
		index = total - 1
	}

	segments := len(colors) - 1
	stepCount := total - 1
	scaled := index * segments
	start := scaled / stepCount
	frac := float64(scaled%stepCount) / float64(stepCount)
	if start >= segments {
		return colors[len(colors)-1]
	}

	r1, g1, b1 := parseHexColor(colors[start])
	r2, g2, b2 := parseHexColor(colors[start+1])
	return fmt.Sprintf("#%02X%02X%02X", lerp(r1, r2, frac), lerp(g1, g2, frac), lerp(b1, b2, frac))
}

func parseHexColor(color string) (int, int, int) {
	color = strings.TrimPrefix(color, "#")
	if len(color) != 6 {
		return 0, 0, 0
	}
	r, _ := strconv.ParseInt(color[0:2], 16, 0)
	g, _ := strconv.ParseInt(color[2:4], 16, 0)
	b, _ := strconv.ParseInt(color[4:6], 16, 0)
	return int(r), int(g), int(b)
}

func lerp(from, to int, frac float64) int {
	return from + int(float64(to-from)*frac)
}
