package foodflow

import (
	"fmt"
	"strconv"
	"strings"
)

const bodyRows = 17

func (m foodModel) View() string {
	if m.viewport.Width > 0 && (m.viewport.Width < 80 || m.viewport.Height < 24) {
		return "Food needs an 80x24 terminal. Resize and try again.\r\n"
	}

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" "+mutedStyle.Render(m.sessionStatus()), m.headerRight()))
	sb.WriteString(divider())

	var body strings.Builder
	switch m.screen {
	case foodScreenHome:
		m.renderHome(&body)
	case foodScreenSearchInput:
		m.renderRestaurantSearch(&body)
	case foodScreenRestaurantList:
		m.renderRestaurantList(&body)
	case foodScreenMenuBrowse:
		m.renderMenuBrowse(&body)
	case foodScreenMenuSearch:
		m.renderMenuSearch(&body)
	case foodScreenMenuResults:
		m.renderMenuResults(&body)
	case foodScreenItemDetail:
		m.renderItemDetail(&body)
	case foodScreenCartReview:
		m.renderCartReview(&body)
	case foodScreenCoupons:
		m.renderCoupons(&body)
	case foodScreenCheckoutConfirm:
		m.renderCheckoutConfirm(&body)
	case foodScreenOrderResult:
		m.renderOrderResult(&body)
	case foodScreenOrders:
		m.renderOrders(&body)
	case foodScreenTracking:
		m.renderTracking(&body)
	case foodScreenLoading:
		body.WriteString(line(" " + m.loading))
	case foodScreenMessage:
		msg := m.message
		if msg == "" {
			msg = m.status
		}
		body.WriteString(line(" " + msg))
	}

	sb.WriteString(fixedBody(body.String(), bodyRows))
	sb.WriteString(m.bottomStatusLine())
	sb.WriteString(divider())
	sb.WriteString(m.footer())
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (m foodModel) bottomStatusLine() string {
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

func (m foodModel) headerRight() string {
	if m.selectedAddress != nil {
		return creamStyle.Render("delivering to ") + brandStyle.Render(foodAddressLabel(*m.selectedAddress))
	}
	return mutedStyle.Render("address required")
}

func (m foodModel) sessionStatus() string {
	return "env=food  auth=ok  cart=" + strconv.Itoa(m.sessionCartCount()) + "  mode=" + m.screenMode()
}

func (m foodModel) sessionCartCount() int {
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

func (m foodModel) screenMode() string {
	switch m.screen {
	case foodScreenHome:
		return "home"
	case foodScreenSearchInput:
		return "grep"
	case foodScreenRestaurantList:
		return "restaurants"
	case foodScreenMenuBrowse:
		return "menu"
	case foodScreenMenuSearch:
		return "dish-search"
	case foodScreenMenuResults:
		return "dishes"
	case foodScreenItemDetail:
		return "item"
	case foodScreenLoading:
		return "loading"
	case foodScreenCartReview:
		return "cart"
	case foodScreenCoupons:
		return "coupons"
	case foodScreenCheckoutConfirm:
		return "deploy"
	case foodScreenOrderResult:
		return "logs"
	case foodScreenOrders:
		return "history"
	case foodScreenTracking:
		return "tail"
	default:
		return "message"
	}
}

func (m foodModel) footer() string {
	switch m.screen {
	case foodScreenHome:
		return footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "1-4", Label: "select"},
			KeyHint{Key: "/", Label: "search restaurants"},
			KeyHint{Key: "d", Label: "search dish"},
			KeyHint{Key: "c", Label: "cart"},
			KeyHint{Key: "esc", Label: "main"},
			KeyHint{Key: "q", Label: "quit"},
		)
	case foodScreenSearchInput:
		return footerLine(
			KeyHint{Key: "enter", Label: "open results"},
			KeyHint{Key: "esc", Label: "home"},
			KeyHint{Key: "q", Label: "quit"},
		)
	case foodScreenRestaurantList:
		return footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "1-9", Label: "choose"},
			KeyHint{Key: "enter", Label: "open menu"},
			KeyHint{Key: "esc", Label: "home"},
		)
	case foodScreenMenuBrowse:
		return footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "enter", Label: "add to cart"},
			KeyHint{Key: "s", Label: "search dish"},
			KeyHint{Key: "esc", Label: "back"},
		)
	case foodScreenMenuSearch:
		return footerLine(
			KeyHint{Key: "enter", Label: "open results"},
			KeyHint{Key: "esc", Label: "home"},
			KeyHint{Key: "q", Label: "quit"},
		)
	case foodScreenMenuResults:
		return footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "1-9", Label: "choose"},
			KeyHint{Key: "enter", Label: "add to cart"},
			KeyHint{Key: "esc", Label: "home"},
		)
	case foodScreenItemDetail:
		return footerLine(
			KeyHint{Key: "j/k", Label: "variant"},
			KeyHint{Key: "tab", Label: "next group"},
			KeyHint{Key: "enter", Label: "add to cart"},
			KeyHint{Key: "esc", Label: "back"},
		)
	case foodScreenCartReview:
		hints := []KeyHint{
			{Key: "p/enter", Label: "checkout"},
			{Key: "c", Label: "coupons"},
			{Key: "/", Label: "search dish"},
			{Key: "b", Label: "home"},
		}
		if m.cartReviewOverflows() {
			hints = append([]KeyHint{{Key: "j/k", Label: "scroll"}}, hints...)
		}
		return footerLine(hints...)
	case foodScreenCoupons:
		return footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "1-9", Label: "select"},
			KeyHint{Key: "enter", Label: "apply"},
			KeyHint{Key: "b/esc", Label: "back"},
		)
	case foodScreenCheckoutConfirm:
		return footerLine(
			KeyHint{Key: "y", Label: "place order"},
			KeyHint{Key: "n", Label: "cancel"},
		)
	case foodScreenOrders:
		return footerLine(
			KeyHint{Key: "j/k", Label: "move"},
			KeyHint{Key: "enter", Label: "tail"},
			KeyHint{Key: "b", Label: "home"},
		)
	default:
		return footerLine(
			KeyHint{Key: "enter", Label: "home"},
			KeyHint{Key: "q", Label: "quit"},
		)
	}
}
