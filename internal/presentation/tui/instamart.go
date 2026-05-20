package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// InstamartPlaceholderView is the backward-compat alias used by runSession.
// Renders the full Instamart screen with placeholder/zero values.
type InstamartPlaceholderView struct {
	SSHIdentityID string
	StatusMessage string
	In            io.Reader
}

func (v InstamartPlaceholderView) Render(ctx context.Context, w io.Writer) error {
	return InstamartView{
		AddressLabel:  "Home",
		AddressLine:   "221B, 12th Main, Indiranagar, Bengaluru",
		CartItemCount: 0,
		StatusMessage: v.StatusMessage,
		In:            v.In,
	}.Render(ctx, w)
}

// InstamartView renders the full Instamart screen.
type InstamartView struct {
	AddressLabel  string // e.g. "Home"
	AddressLine   string // e.g. "221B, 12th Main, Indiranagar, Bengaluru"
	CartItemCount int
	StatusMessage string
	In            io.Reader // if nil, static render
}

var instamartChoices = []string{
	"Search products",
	"Your go-to items",
	"View cart",
	"Track recent order",
	"Change address",
}

type instamartModel struct {
	ctx          context.Context
	viewport     Viewport
	cursor       int
	choices      []string
	addressLabel string
	addressLine  string
	cartCount    int
	status       string
}

func (m instamartModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

func (m instamartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m instamartModel) View() string {
	deliverTo := creamStyle.Render("Delivering to ") + brandStyle.Render(m.addressLabel)

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" "+brandStyle.Render("swiggy.ssh")+creamStyle.Render(" > Instamart"), deliverTo))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Instamart") + creamStyle.Render(" — Groceries and daily essentials in minutes.")))
	if m.status != "" {
		sb.WriteString(line(" " + successStyle.Render("✓ "+m.status)))
	}
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Address:") + " " + boldStyle.Render(m.addressLabel) + creamStyle.Render(" — "+m.addressLine)))
	sb.WriteString(line(" " + brandStyle.Render("Cart:") + " " + creamStyle.Render(fmt.Sprintf("%d items", m.cartCount))))
	sb.WriteString(line(""))
	sb.WriteString(divider())
	for i, choice := range m.choices {
		label := fmt.Sprintf("%d. %s", i+1, choice)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(footerLine(
		KeyHint{Key: "/", Label: "search"},
		KeyHint{Key: "c", Label: "cart"},
		KeyHint{Key: "a", Label: "address"},
		KeyHint{Key: "b", Label: "back"},
		KeyHint{Key: "?", Label: "help"},
		KeyHint{Key: "q", Label: "quit"},
	))
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (v InstamartView) Render(ctx context.Context, w io.Writer) error {
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := instamartModel{
		ctx:          ctx,
		viewport:     viewportFromContext(ctx),
		cursor:       0,
		choices:      instamartChoices,
		addressLabel: v.AddressLabel,
		addressLine:  v.AddressLine,
		cartCount:    v.CartItemCount,
		status:       v.StatusMessage,
	}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}
