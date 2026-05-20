package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// HomeAction is returned by HomeView.Render to tell the caller what the user selected.
type HomeAction int

const (
	HomeActionNone            HomeAction = iota // user quit without selecting
	HomeActionInstamart                         // user selected Instamart
	HomeActionAddressSelected                   // user selected a session address
	HomeActionTrackOrders                       // user selected active order tracking
	HomeActionFood                              // user selected Food
)

// HomeAddressStatus is the root readiness state for delivery targets.
type HomeAddressStatus int

const (
	HomeAddressUnknown HomeAddressStatus = iota
	HomeAddressRequired
	HomeAddressUnavailable
	HomeAddressSelected
)

// HomeAddressOption is the presentation-safe shape for a saved delivery address.
type HomeAddressOption struct {
	ID          string
	Label       string
	DisplayLine string
	PhoneMasked string
	Category    string
}

// HomeSessionState describes auth and selected-address readiness for Home.
type HomeSessionState struct {
	Authenticated        bool
	AddressStatus        HomeAddressStatus
	SelectedAddressIndex int
	Addresses            []HomeAddressOption
}

// HomeResult is returned by interactive Home sessions.
type HomeResult struct {
	Action       HomeAction
	AddressIndex int
}

// HomeView renders the main home/menu screen.
type HomeView struct {
	Session            HomeSessionState
	StartAddressPicker bool
	StartMenu          bool
	In                 io.Reader // if nil, static render (no keyboard input)
}

type homeItem struct {
	icon      string
	label     string
	action    string
	available bool
}

const (
	homeItemInstamart = "instamart"
	homeItemFood      = "food"
	homeItemAddresses = "addresses"
	homeItemTrack     = "track"
	homeItemAI        = "ai"
)

var homeLogoGradient = []string{"#E97112", "#FC8019", "#FF8B2E", "#FF9843"}

type homeSplashTickMsg struct{}

const homeSplashTickInterval = 500 * time.Millisecond

type homeModel struct {
	ctx           context.Context
	viewport      Viewport
	cursor        int
	addressCursor int
	items         []homeItem
	result        HomeResult
	notice        string // transient "coming soon" message
	menu          bool
	addresses     bool
	animate       bool
	shineStep     int
	session       HomeSessionState
}

func (m homeModel) Init() tea.Cmd {
	if m.animate && !m.menu && !m.addresses {
		return tea.Batch(ctxQuitCmd(m.ctx), homeSplashTickCmd())
	}
	return ctxQuitCmd(m.ctx)
}

func homeSplashTickCmd() tea.Cmd {
	return tea.Tick(homeSplashTickInterval, func(time.Time) tea.Msg {
		return homeSplashTickMsg{}
	})
}

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case homeSplashTickMsg:
		if !m.menu && !m.addresses && m.animate {
			m.shineStep = (m.shineStep + 1) % homeSplashShineFrames
			return m, homeSplashTickCmd()
		}
		return m, nil
	case tea.KeyMsg:
		if m.addresses {
			switch msg.String() {
			case "q", "ctrl+c":
				m.result.Action = HomeActionNone
				return m, tea.Quit
			case "esc", "b":
				m.addresses = false
				m.menu = true
				m.notice = ""
				return m, nil
			case "up", "k":
				if m.addressCursor > 0 {
					m.addressCursor--
				}
			case "down", "j":
				if m.addressCursor < len(m.session.Addresses)-1 {
					m.addressCursor++
				}
			case "enter", " ":
				if len(m.session.Addresses) > 0 {
					m.result.Action = HomeActionAddressSelected
					m.result.AddressIndex = m.addressCursor
					return m, tea.Quit
				}
			default:
				if idx, ok := homeNumberKeyIndex(msg.String(), len(m.session.Addresses)); ok {
					m.result.Action = HomeActionAddressSelected
					m.result.AddressIndex = idx
					return m, tea.Quit
				}
			}
			return m, nil
		}
		if !m.menu {
			switch msg.String() {
			case "q", "ctrl+c":
				m.result.Action = HomeActionNone
				return m, tea.Quit
			case "enter", " ":
				m.menu = true
				return m, nil
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.result.Action = HomeActionNone
			return m, tea.Quit
		case "esc":
			m.menu = false
			m.notice = ""
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.notice = ""
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.notice = ""
			}
		case "enter", " ":
			return m.activateMenuItem(m.cursor)
		default:
			if idx, ok := homeNumberKeyIndex(msg.String(), len(m.items)); ok {
				return m.activateMenuItem(idx)
			}
		}
	}
	return m, nil
}

func (m homeModel) activateMenuItem(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.items) {
		return m, nil
	}
	item := m.items[idx]
	if !item.available {
		m.notice = item.label + " - coming soon"
		return m, nil
	}
	switch item.action {
	case homeItemInstamart:
		m.result.Action = HomeActionInstamart
		return m, tea.Quit
	case homeItemFood:
		m.result.Action = HomeActionFood
		return m, tea.Quit
	case homeItemTrack:
		m.result.Action = HomeActionTrackOrders
		return m, tea.Quit
	case homeItemAddresses:
		m.addresses = true
		m.menu = false
		m.notice = ""
		m.addressCursor = selectedHomeAddressIndex(m.session)
		return m, nil
	default:
		m.notice = item.label + " - coming soon"
		return m, nil
	}
}

const homeSplashShineFrames = 11

func homeSplashRenderLine(content string, row, total, shineStep int) string {
	if row == (shineStep+2)%homeSplashShineFrames {
		return shineStyle.Render(content)
	}
	return gradientRender(content, homeLogoGradient, row, total)
}

func homeSplashContinueHint(shineStep int) KeyHint {
	markers := []struct {
		left  string
		right string
	}{
		{"▸", "◂"},
		{"›", "‹"},
		{"»", "«"},
		{"›", "‹"},
	}
	marker := markers[shineStep%len(markers)]
	return KeyHint{Key: marker.left + " enter", Label: "continue " + marker.right, Highlight: true}
}

func (m homeModel) View() string {
	logo := []string{
		"    ⢀⣠⣴⣶⣶⣶⣶⣦⣄⡀    ",
		"  ⢀⣴⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣦⡀ ",
		"  ⣾⣿⣿⣿⣿⣿⣿⡏⢹⣿⣿⣿⣿⣷ ",
		" ⢰⣿⣿⣿⣿⣿⣿⣿⡇⠸⠿⠿⠿⠿⠟⠃",
		" ⠈⣿⣿⣿⣿⣿⣿⣿⣷⣶⣶⣶⣶⣶⣤ ",
		"  ⢹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡟ ",
		"   ⠻⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠁ ",
		"    ⢶⣶⣶⣶⣤⢸⣿⣿⣿⡿⠁  ",
		"     ⠹⣿⣿⣿⣼⣿⣿⡟⠁   ",
		"      ⠈⢿⣿⣿⡿⠋     ",
		"        ⠹⠏        ",
	}
	wordmark := []string{
		"███████╗██╗    ██╗██╗ ██████╗  ██████╗ ██╗   ██╗",
		"██╔════╝██║    ██║██║██╔════╝ ██╔════╝ ╚██╗ ██╔╝",
		"███████╗██║ █╗ ██║██║██║  ███╗██║  ███╗ ╚████╔╝ ",
		"╚════██║██║███╗██║██║██║   ██║██║   ██║  ╚██╔╝  ",
		"███████║╚███╔███╔╝██║╚██████╔╝╚██████╔╝   ██║   ",
		"╚══════╝ ╚══╝╚══╝ ╚═╝ ╚═════╝  ╚═════╝    ╚═╝   ",
	}

	// Keep the root menu inside the same 80x24 frame as Instamart.
	const logoLines = 11
	const wordmarkLines = 6
	const wordmarkWidth = 48
	const topPad = (logoLines - wordmarkLines) / 2
	const splashTopPad = (fixedFrameBodyRows - logoLines + 1) / 2

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" swiggy.dev", m.headerRight()))
	sb.WriteString(divider())

	var body strings.Builder
	if m.addresses {
		body.WriteString(line(""))
		body.WriteString(line(brandStyle.Render(" Choose deployment address")))
		if len(m.session.Addresses) == 0 {
			body.WriteString(line(" " + mutedStyle.Render("No saved addresses found. Add one in Swiggy first.")))
		} else {
			for i, address := range m.session.Addresses {
				label := fmt.Sprintf("%d. %s", i+1, homeAddressLabel(address))
				if address.PhoneMasked != "" {
					label += "  " + mutedStyle.Render(address.PhoneMasked)
				}
				if address.DisplayLine != "" {
					label += " - " + mutedStyle.Render(truncateAddressLine(address.DisplayLine))
				}
				if m.addressCursor == i {
					body.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
				} else {
					body.WriteString(line("   " + label))
				}
			}
		}
		sb.WriteString(fixedBody(body.String(), fixedFrameBodyRows))
		sb.WriteString(divider())
		sb.WriteString(footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "enter", Label: "select"},
			KeyHint{Key: "esc", Label: "menu"},
			KeyHint{Key: "q", Label: "quit"},
		))
		sb.WriteString(bottom())
		return centerInViewport(sb.String(), m.viewport)
	}
	if !m.menu {
		for range splashTopPad {
			body.WriteString(line(""))
		}
		for i, logoLine := range logo {
			wmIdx := i - topPad
			right := strings.Repeat(" ", wordmarkWidth)
			if wmIdx >= 0 && wmIdx < wordmarkLines {
				right = gradientRender(wordmark[wmIdx], homeLogoGradient, wmIdx, wordmarkLines)
			}
			body.WriteString(centeredLine(homeSplashRenderLine(logoLine, i, logoLines, m.shineStep) + "  " + right))
		}
		sb.WriteString(fixedBody(body.String(), fixedFrameBodyRows))
		sb.WriteString(divider())
		sb.WriteString(footerLine(
			homeSplashContinueHint(m.shineStep),
			KeyHint{Key: "q", Label: "quit"},
		))
		sb.WriteString(bottom())
		return centerInViewport(sb.String(), m.viewport)
	}

	body.WriteString(line(brandStyle.Render(" What would you like to do?")))
	body.WriteString(line(""))
	for i, item := range m.items {
		label := homeItemLabel(item)
		if !item.available {
			label += "  " + mutedStyle.Render("(coming soon)")
		}
		if m.cursor == i {
			body.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			if item.available {
				body.WriteString(line("   " + label))
			} else {
				body.WriteString(line(mutedStyle.Render("   " + label)))
			}
		}
	}
	if m.notice != "" {
		body.WriteString(line(" " + accentStyle.Render("⚡ "+m.notice)))
	} else {
		body.WriteString(line(""))
	}
	sb.WriteString(fixedBody(body.String(), fixedFrameBodyRows))
	sb.WriteString(divider())
	sb.WriteString(footerLine(
		KeyHint{Key: "j/k", Label: "move"},
		KeyHint{Key: "enter", Label: "select"},
		KeyHint{Key: "esc", Label: "splash"},
		KeyHint{Key: "q", Label: "quit"},
	))
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (m homeModel) headerRight() string {
	if !m.session.Authenticated {
		return errorStyle.Render("auth required ")
	}
	switch m.session.AddressStatus {
	case HomeAddressSelected:
		idx := selectedHomeAddressIndex(m.session)
		if idx >= 0 && idx < len(m.session.Addresses) {
			return creamStyle.Render("deploying to ") + brandStyle.Render(homeAddressLabel(m.session.Addresses[idx]))
		}
		return creamStyle.Render("deploying to ") + brandStyle.Render("selected address")
	case HomeAddressRequired:
		return errorStyle.Render("address required ")
	case HomeAddressUnavailable:
		return errorStyle.Render("address unavailable ")
	default:
		return mutedStyle.Render("address unavailable ")
	}
}

func homeAddressLabel(address HomeAddressOption) string {
	if strings.TrimSpace(address.Label) != "" {
		return address.Label
	}
	if strings.TrimSpace(address.Category) != "" {
		return address.Category
	}
	return "Saved address"
}

func truncateAddressLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	const max = 18
	runes := []rune(line)
	if len(runes) <= max {
		return line
	}
	return string(runes[:max]) + "..."
}

func selectedHomeAddressIndex(state HomeSessionState) int {
	if state.SelectedAddressIndex >= 0 && state.SelectedAddressIndex < len(state.Addresses) {
		return state.SelectedAddressIndex
	}
	return 0
}

func homeNumberKeyIndex(key string, length int) (int, bool) {
	if length == 0 || len(key) != 1 || key[0] < '1' || key[0] > '9' {
		return 0, false
	}
	idx := int(key[0] - '1')
	return idx, idx < length
}

func homeItemLabel(item homeItem) string {
	if strings.TrimSpace(item.icon) == "" {
		return item.label
	}
	return item.icon + "  " + item.label
}

func homeItemsForSession(state HomeSessionState) []homeItem {
	addressAvailable := state.Authenticated && len(state.Addresses) > 0
	return []homeItem{
		{icon: "▦", label: "Instamart", action: homeItemInstamart, available: true},
		{icon: "◖", label: "Food", action: homeItemFood, available: true},
		{icon: "⌂", label: "Addresses", action: homeItemAddresses, available: addressAvailable},
		{icon: "◷", label: "Tail active order", action: homeItemTrack, available: true},
		{icon: "✦", label: "swiggy.ai", action: homeItemAI, available: false},
	}
}

func (v HomeView) Render(ctx context.Context, w io.Writer) error {
	m := v.homeModel(ctx)
	_, err := runInteractive(m, w, v.In)
	return err
}

// RenderWithAction runs HomeView and returns what the user selected.
func (v HomeView) RenderWithAction(ctx context.Context, w io.Writer) (HomeAction, error) {
	result, err := v.RenderWithResult(ctx, w)
	return result.Action, err
}

// RenderWithResult runs HomeView and returns the selected action and address index.
func (v HomeView) RenderWithResult(ctx context.Context, w io.Writer) (HomeResult, error) {
	m := v.homeModel(ctx)
	finalModel, err := runInteractive(m, w, v.In)
	if err != nil {
		return HomeResult{Action: HomeActionNone}, err
	}
	if hm, ok := finalModel.(homeModel); ok {
		return hm.result, nil
	}
	return HomeResult{Action: HomeActionNone}, nil
}

func (v HomeView) homeModel(ctx context.Context) homeModel {
	return homeModel{
		ctx:           ctx,
		viewport:      viewportFromContext(ctx),
		cursor:        0,
		addressCursor: selectedHomeAddressIndex(v.Session),
		items:         homeItemsForSession(v.Session),
		menu:          v.StartMenu,
		addresses:     v.StartAddressPicker,
		animate:       v.In != nil,
		session:       v.Session,
	}
}
