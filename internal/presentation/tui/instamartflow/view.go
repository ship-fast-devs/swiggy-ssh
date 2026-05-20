package instamartflow

import (
	"fmt"
	"strconv"
	"strings"
)

const bodyRows = 17

func (m instamartModel) View() string {
	if m.viewport.Width > 0 && (m.viewport.Width < 80 || m.viewport.Height < 24) {
		return "Instamart needs an 80x24 terminal. Resize and try again.\r\n"
	}

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" "+mutedStyle.Render(m.sessionStatus()), m.headerRight()))
	sb.WriteString(divider())

	var body strings.Builder
	switch m.screen {
	case instamartScreenStatic:
		m.renderStatic(&body)
	case instamartScreenLoadingAddresses:
		body.WriteString(line(" Loading saved deployment addresses..."))
	case instamartScreenAddressSelect:
		m.renderAddresses(&body)
	case instamartScreenHome:
		m.renderHome(&body)
	case instamartScreenSearchInput:
		m.renderSearch(&body)
	case instamartScreenProductList:
		m.renderProducts(&body)
	case instamartScreenQuantity:
		m.renderQuantity(&body)
	case instamartScreenLoading:
		body.WriteString(line(" " + m.loading))
	case instamartScreenCartReview:
		m.renderCartReview(&body)
	case instamartScreenCheckoutConfirm:
		m.renderCheckoutConfirm(&body)
	case instamartScreenOrderResult:
		m.renderOrderResult(&body)
	case instamartScreenOrders:
		m.renderOrders(&body)
	case instamartScreenTracking:
		m.renderTracking(&body)
	case instamartScreenMessage:
		message := m.message
		if message == "" {
			message = m.status
		}
		body.WriteString(line(" " + message))
	case instamartScreenHelp:
		m.renderHelp(&body)
	}

	sb.WriteString(fixedBody(body.String(), bodyRows))
	sb.WriteString(m.bottomStatusLine())
	sb.WriteString(divider())
	sb.WriteString(m.footer())
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (m instamartModel) bottomStatusLine() string {
	if m.err != "" {
		return rightLine(errorStyle.Render(m.err))
	}
	if m.status != "" {
		return rightLine(successStyle.Render("✓ " + m.status))
	}
	return line("")
}

func fixedBody(content string, rows int) string {
	if rows <= 0 {
		return ""
	}
	trimmed := strings.TrimSuffix(content, "\r\n")
	if trimmed == "" {
		trimmed = ""
	}
	lines := []string{}
	if trimmed != "" {
		lines = strings.Split(trimmed, "\r\n")
	}
	overflow := len(lines) > rows
	if overflow {
		total := len(lines)
		lines = append([]string(nil), lines[:rows-1]...)
		lines = append(lines, strings.TrimSuffix(line(mutedStyle.Render(fmt.Sprintf("... %d more lines", total-rows+1))), "\r\n"))
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

func (m instamartModel) headerRight() string {
	if m.screen == instamartScreenStatic {
		return creamStyle.Render("deploying to ") + brandStyle.Render(defaultString(m.staticAddressLabel, "Home"))
	}
	if m.selectedAddress != nil {
		return creamStyle.Render("deploying to ") + brandStyle.Render(addressLabel(*m.selectedAddress))
	}
	return mutedStyle.Render("choose address")
}

func (m instamartModel) sessionStatus() string {
	return "env=instamart  auth=ok  cart=" + strconv.Itoa(m.sessionCartCount()) + "  mode=" + m.screenMode()
}

func (m instamartModel) sessionCartCount() int {
	if m.screen == instamartScreenStatic {
		return m.staticCartCount
	}
	count := 0
	for _, item := range m.intendedItems {
		if item.Quantity > 0 {
			count += item.Quantity
		}
	}
	if count > 0 {
		return count
	}
	for _, item := range m.currentCart.Items {
		if item.Quantity > 0 {
			count += item.Quantity
		}
	}
	return count
}

func (m instamartModel) screenMode() string {
	switch m.screen {
	case instamartScreenStatic, instamartScreenHome:
		return "home"
	case instamartScreenLoadingAddresses, instamartScreenAddressSelect:
		return "address"
	case instamartScreenSearchInput, instamartScreenProductList:
		return "grep"
	case instamartScreenQuantity:
		return "stage"
	case instamartScreenLoading:
		return "loading"
	case instamartScreenCartReview:
		return "cart"
	case instamartScreenCheckoutConfirm:
		return "deploy"
	case instamartScreenOrderResult:
		return "logs"
	case instamartScreenOrders:
		return "history"
	case instamartScreenTracking:
		return "tail"
	case instamartScreenHelp:
		return "help"
	default:
		return "message"
	}
}

func (m instamartModel) footer() string {
	switch m.screen {
	case instamartScreenAddressSelect:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-9", Label: "select"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenHome, instamartScreenStatic:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-3", Label: "select"}, KeyHint{Key: "/", Label: "grep"}, KeyHint{Key: "c", Label: "cart"}, KeyHint{Key: "esc", Label: "main"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenSearchInput:
		return footerLine(KeyHint{Key: "enter", Label: "open results"}, KeyHint{Key: "esc", Label: "home"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenProductList:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-9", Label: "choose"}, KeyHint{Key: "enter", Label: "choose"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "esc", Label: "home"})
	case instamartScreenQuantity:
		return footerLine(KeyHint{Key: "+/-", Label: "quantity"}, KeyHint{Key: "enter", Label: "stage"}, KeyHint{Key: "b/esc", Label: "results"}, KeyHint{Key: "?", Label: "help"})
	case instamartScreenCartReview:
		hints := []KeyHint{{Key: "p/enter", Label: "deploy"}, {Key: "/", Label: "grep"}, {Key: "b", Label: "home"}}
		if m.cartReviewOverflows() {
			hints = append([]KeyHint{{Key: "j/k", Label: "scroll"}}, hints...)
		}
		return footerLine(hints...)
	case instamartScreenCheckoutConfirm:
		return footerLine(KeyHint{Key: "y", Label: "deploy"}, KeyHint{Key: "n", Label: "cancel"})
	case instamartScreenOrders:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "enter", Label: "tail"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "b", Label: "home"})
	case instamartScreenHelp:
		return footerLine(KeyHint{Key: "?", Label: "back"}, KeyHint{Key: "b", Label: "back"}, KeyHint{Key: "q", Label: "quit"})
	default:
		return footerLine(KeyHint{Key: "enter", Label: "home"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	}
}
