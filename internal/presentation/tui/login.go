package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"swiggy-ssh/internal/application/auth"
)

// LoginWaitingView renders the direct browser-login prompt.
type LoginWaitingView struct {
	LoginURL string
	In       io.Reader
}

func (v LoginWaitingView) Render(ctx context.Context, w io.Writer) error {
	if v.In != nil {
		_, err := runInteractive(loginWaitingModel{
			ctx:      ctx,
			viewport: viewportFromContext(ctx),
			loginURL: v.LoginURL,
		}, w, v.In)
		return err
	}
	return runStatic(w, loginWaitingContent(viewportFromContext(ctx), v.LoginURL, "pending", false, false))
}

type loginWaitingModel struct {
	ctx      context.Context
	viewport Viewport
	loginURL string
	copied   bool
}

func (m loginWaitingModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

func (m loginWaitingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			m.copied = true
			return m, nil
		case "q", "b", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m loginWaitingModel) View() string {
	status := "pending"
	if m.copied {
		status = "pending"
	}
	return loginWaitingContent(m.viewport, m.loginURL, status, true, m.copied)
}

func loginWaitingContent(viewport Viewport, loginURL, status string, interactive, copied bool) string {
	var sb strings.Builder
	if copied {
		sb.WriteString(osc52(loginURL))
	}
	sb.WriteString(top())
	sb.WriteString(line(" " + brandStyle.Render("swiggy.dev") + creamStyle.Render(" > auth")))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("POST /auth/browser-attempt 201 Created")))
	sb.WriteString(line(" " + mutedStyle.Render("response:")))
	sb.WriteString(line("   login_url: " + accentStyle.Render(osc8("Open Swiggy login", loginURL))))
	sb.WriteString(line("   expires: " + mutedStyle.Render("short_lived")))
	sb.WriteString(line("   status: " + mutedStyle.Render(status)))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Open login_url in your browser:")))
	sb.WriteString(line(""))
	for _, wrapped := range wrapText(loginURL, 70) {
		sb.WriteString(line(" " + accentStyle.Render(wrapped)))
	}
	sb.WriteString(line(""))
	sb.WriteString(line(" " + creamStyle.Render("This one-time link securely connects browser auth to this SSH session.")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + creamStyle.Render("Waiting for callback...")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("poll:") + " " + mutedStyle.Render("GET /auth/session -> pending")))
	if copied {
		sb.WriteString(line(" " + brandStyle.Render("clipboard:") + " " + mutedStyle.Render("copy attempted")))
	}
	sb.WriteString(line(""))
	sb.WriteString(divider())
	if interactive {
		sb.WriteString(footerLine(
			KeyHint{Key: "c", Label: "copy URL"},
			KeyHint{Key: "click", Label: "Open Swiggy login"},
		))
	} else {
		sb.WriteString(footerLine(KeyHint{Key: "click", Label: "Open Swiggy login"}))
	}
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), viewport)
}

// LoginSuccessView renders the post-login confirmation.
type LoginSuccessView struct {
	IsFirstAuth bool
	WasReauth   bool
	Account     auth.OAuthAccount // kept for safety — never rendered
	DisplayName string            // shown as "Signed in as" name
	Email       string            // shown as email
	In          io.Reader         // if nil, static render
}

var loginSuccessChoices = []string{
	"Continue to Instamart",
}

type loginSuccessModel struct {
	ctx      context.Context
	viewport Viewport
	cursor   int
	choices  []string
	name     string
	email    string
	message  string
}

func (m loginSuccessModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

func (m loginSuccessModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", "b":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m loginSuccessModel) View() string {
	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" "+brandStyle.Render("swiggy.dev")+creamStyle.Render(" > auth/session"), mutedStyle.Render("secure browser login")+" "))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(centeredLine(successStyle.Render("GET /auth/session 200 OK")))
	sb.WriteString(centeredLine(creamStyle.Render(m.message)))
	sb.WriteString(line(""))
	sb.WriteString(line("  " + mutedStyle.Render("┌─ response.account ────────────────────────────────────────────────────┐")))
	sb.WriteString(line("  " + mutedStyle.Render("│") + " provider: " + brandStyle.Render("swiggy")))
	sb.WriteString(line("  " + mutedStyle.Render("│") + " status:   " + successStyle.Render("active")))
	sb.WriteString(line("  " + mutedStyle.Render("│") + " name:     " + boldStyle.Render(m.name)))
	sb.WriteString(line("  " + mutedStyle.Render("│") + " email:    " + mutedStyle.Render(m.email)))
	sb.WriteString(line("  " + mutedStyle.Render("│") + " tokens:   " + mutedStyle.Render("encrypted")))
	sb.WriteString(line("  " + mutedStyle.Render("└──────────────────────────────────────────────────────────────────────┘")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + creamStyle.Render("Fresh auth is stored. Access tokens are encrypted and never shown.")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Next request")))
	sb.WriteString(line(""))
	for i, choice := range m.choices {
		label := fmt.Sprintf("%d. %s", i+1, choice)
		if m.cursor == i {
			sb.WriteString(line(" " + cursorStyle.Render("▸ ") + boldStyle.Render(label) + "  " + loginSuccessChoiceHint(choice)))
		} else {
			sb.WriteString(line("   " + label + "  " + loginSuccessChoiceHint(choice)))
		}
	}
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(footerLine(
		KeyHint{Key: "enter", Label: "continue"},
		KeyHint{Key: "q", Label: "quit"},
	))
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (v LoginSuccessView) Render(ctx context.Context, w io.Writer) error {
	name := v.DisplayName
	if name == "" {
		name = "Unknown"
	}
	email := v.Email
	if email == "" {
		email = "(no email)"
	}
	m := loginSuccessModel{
		ctx:      ctx,
		viewport: viewportFromContext(ctx),
		cursor:   0,
		choices:  loginSuccessChoices,
		name:     name,
		email:    email,
		message:  loginSuccessMessage(v.IsFirstAuth, v.WasReauth),
	}
	_, err := runInteractive(m, w, v.In)
	return err
}

func loginSuccessMessage(isFirstAuth, wasReauth bool) string {
	switch {
	case isFirstAuth:
		return "Welcome! Your Swiggy account is linked to this SSH key."
	case wasReauth:
		return "You are back online with a freshly verified Swiggy session."
	default:
		return "Welcome back. Your saved Swiggy account is ready."
	}
}

func loginSuccessChoiceHint(choice string) string {
	switch choice {
	case "Continue to Instamart":
		return mutedStyle.Render("search groceries and build a cart")
	case "Home":
		return mutedStyle.Render("return to the command center")
	case "Account settings":
		return mutedStyle.Render("review linked account details")
	default:
		return ""
	}
}

// ReconnectView renders the re-auth prompt shown before a new browser auth attempt.
// Inline (no full-screen box): shown mid-session when re-auth is needed.
type ReconnectView struct {
	LoginURL string
}

func (v ReconnectView) Render(ctx context.Context, w io.Writer) error {
	var sb strings.Builder
	sb.WriteString("\r\n")
	sb.WriteString("  " + brandStyle.Render("Your session needs re-authentication.") + "\r\n")
	sb.WriteString("  " + creamStyle.Render("Open this one-time browser login URL:") + "\r\n")
	sb.WriteString("\r\n")
	sb.WriteString("     " + accentStyle.Render(osc8("Open Swiggy login", v.LoginURL)) + "\r\n")
	sb.WriteString("\r\n")
	for _, wrapped := range wrapText(v.LoginURL, 70) {
		sb.WriteString("     " + accentStyle.Render(boldStyle.Render(wrapped)) + "\r\n")
	}
	sb.WriteString("\r\n")
	sb.WriteString("  " + creamStyle.Render("Waiting...") + "\r\n")
	_, err := fmt.Fprint(w, centerInViewport(sb.String(), viewportFromContext(ctx)))
	return err
}

func wrapText(s string, width int) []string {
	if s == "" {
		return []string{""}
	}
	var lines []string
	runes := []rune(s)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	lines = append(lines, string(runes))
	return lines
}

func osc8(label, url string) string {
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

func osc52(text string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
}
