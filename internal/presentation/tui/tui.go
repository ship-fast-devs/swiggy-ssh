package tui

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

// View is the terminal screen boundary for the SSH delivery adapter.
type View interface {
	Render(ctx context.Context, w io.Writer) error
}

// Viewport is the available terminal size in character cells.
type Viewport struct {
	Width  int
	Height int
}

type viewportContextKey struct{}

// WithViewport stores terminal dimensions for views that need layout padding.
func WithViewport(ctx context.Context, viewport Viewport) context.Context {
	if viewport.Width <= 0 && viewport.Height <= 0 {
		return ctx
	}
	return context.WithValue(ctx, viewportContextKey{}, viewport)
}

func viewportFromContext(ctx context.Context) Viewport {
	viewport, _ := ctx.Value(viewportContextKey{}).(Viewport)
	return viewport
}

// --- Lipgloss styles ---

var (
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019")) // Swiggy orange
	brandStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019")).Bold(true)
	creamStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF7ED"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA44")).Bold(true) // green bold
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // gray
	boldStyle    = lipgloss.NewStyle().Bold(true)
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019")).Bold(true) // orange + bold
	codeStyle    = lipgloss.NewStyle().Bold(true)
	connStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA44")) // green
	shineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB15C")).Bold(true)
)

func gradientRender(content string, colors []string, index, total int) string {
	if len(colors) == 0 {
		return content
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(gradientColor(colors, index, total))).Render(content)
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

func init() {
	// SSH output is an ssh.Channel, so Lip Gloss cannot reliably detect the
	// client's terminal color support from the server process.
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// --- Box-drawing helpers ---
// All screens are 80 characters wide (including │ borders):
//
//	┌ + 78×─ + ┐
//	│ + 78 chars of content + │
//	└ + 78×─ + ┘

const innerWidth = 78
const fixedFrameBodyRows = 18

func top() string {
	return "┌" + strings.Repeat("─", innerWidth) + "┐\r\n"
}

func bottom() string {
	return "└" + strings.Repeat("─", innerWidth) + "┘\r\n"
}

func divider() string {
	return "├" + strings.Repeat("─", innerWidth) + "┤\r\n"
}

// line returns │ + (space + content right-padded to total 78 inner display columns) + │\r\n.
// Width is measured with lipgloss.Width so ANSI escape sequences from lipgloss styling
// are excluded from the measurement and the box stays exactly 80 chars wide.
func line(content string) string {
	inner := " " + content
	w := lipgloss.Width(inner)
	if w > innerWidth {
		// Hard-truncate visible characters; keep trailing reset if styled.
		// For oversized raw strings we slice runes; styled content shouldn't overflow.
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

func fixedBody(content string, rows int) string {
	if rows <= 0 {
		return ""
	}
	trimmed := strings.TrimSuffix(content, "\r\n")
	lines := []string{}
	if trimmed != "" {
		lines = strings.Split(trimmed, "\r\n")
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	var sb strings.Builder
	for _, rendered := range lines {
		sb.WriteString(rendered)
		sb.WriteString("\r\n")
	}
	for i := len(lines); i < rows; i++ {
		sb.WriteString(line(""))
	}
	return sb.String()
}

type KeyHint struct {
	Key       string
	Label     string
	Highlight bool
}

func footerLine(hints ...KeyHint) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		if hint.Key == "" || hint.Label == "" {
			continue
		}
		text := hint.Key + " " + hint.Label
		if hint.Highlight {
			parts = append(parts, brandStyle.Render(text))
		} else {
			parts = append(parts, mutedStyle.Render(text))
		}
	}
	return centeredLine(strings.Join(parts, "    "))
}

// headerLine builds a line with left and right text separated by spaces, filling
// exactly 78 inner display columns. Uses lipgloss.Width for ANSI-aware measurement.
func headerLine(left, right string) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	spaces := innerWidth - leftW - rightW
	if spaces < 1 {
		spaces = 1
	}
	return "│" + left + strings.Repeat(" ", spaces) + right + "│\r\n"
}

// codeBox renders a framed code inside a 22-char-wide box, wrapped in outer │ borders.
// Each of the 3 returned lines is a full 80-char box row.
func codeBox(code string) string {
	// Center the raw code text (no ANSI) within 20 chars for the inner box.
	rawLen := lipgloss.Width(code)
	totalPad := 20 - rawLen
	if totalPad < 0 {
		totalPad = 0
	}
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	inner := strings.Repeat(" ", leftPad) + codeStyle.Render(code) + strings.Repeat(" ", rightPad)
	return line("     ┌────────────────────┐") +
		line("     │"+inner+"│") +
		line("     └────────────────────┘")
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

// --- Shared Bubbletea helpers ---

const (
	enterAltScreen = "\x1b[?1049h"
	exitAltScreen  = "\x1b[?1049l"
	clearScreen    = "\x1b[H\x1b[2J"
)

// EnterFullscreen switches the terminal to the alternate screen buffer.
func EnterFullscreen(w io.Writer) error {
	_, err := io.WriteString(w, enterAltScreen+clearScreen)
	return err
}

// ExitFullscreen restores the terminal's main screen buffer.
func ExitFullscreen(w io.Writer) error {
	_, err := io.WriteString(w, exitAltScreen)
	return err
}

// ClearScreen clears the active terminal buffer and moves the cursor home.
func ClearScreen(w io.Writer) error {
	_, err := io.WriteString(w, clearScreen)
	return err
}

// ctxQuitCmd returns a Cmd that fires QuitMsg when ctx is cancelled.
// Used by interactive models to respect SSH disconnect / deadline.
func ctxQuitCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return tea.QuitMsg{}
	}
}

// staticModel is a Bubbletea model that renders once and immediately quits.
// Used for all non-interactive views so they run inside a proper tea.Program.
type staticModel struct{ view string }

func (m staticModel) Init() tea.Cmd                           { return tea.Quit }
func (m staticModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m staticModel) View() string                            { return m.view }

// runStatic runs a one-shot Bubbletea program that renders content once and exits.
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
