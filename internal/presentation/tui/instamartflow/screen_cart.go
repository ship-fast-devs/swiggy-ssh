package instamartflow

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

func (m instamartModel) renderCartReview(sb *strings.Builder) {
	lines := m.cartReviewLines()
	rows := bodyRows
	overflow := m.cartReviewOverflows()
	if overflow {
		rows = bodyRows - 1
	}
	if m.cartScroll > len(lines)-rows {
		m.cartScroll = len(lines) - rows
	}
	if m.cartScroll < 0 {
		m.cartScroll = 0
	}
	end := m.cartScroll + rows
	if end > len(lines) {
		end = len(lines)
	}
	for _, text := range lines[m.cartScroll:end] {
		sb.WriteString(line(text))
	}
	if overflow {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf("scroll %d/%d", m.cartScroll+1, len(lines)-rows+1))))
	}
}

func (m instamartModel) cartReviewOverflows() bool {
	return len(m.cartReviewLines()) > bodyRows
}

func (m instamartModel) cartReviewLines() []string {
	cart := m.currentCart
	lines := []string{brandStyle.Render(" review staged cart"), mutedStyle.Render("target") + " " + boldStyle.Render(defaultString(cart.AddressLabel, selectedAddressLabel(m.selectedAddress))) + creamStyle.Render(" — "+redactLine(cart.AddressDisplayLine))}
	if len(cart.StoreIDs) > 1 {
		lines = append(lines, " "+accentStyle.Render(fmt.Sprintf("warn: cart spans %d stores. Swiggy may split deploy.", len(cart.StoreIDs))))
	}
	lines = append(lines, mutedStyle.Render("staged"))
	if len(cart.Items) == 0 {
		lines = append(lines, " working tree clean")
	} else {
		for _, item := range cart.Items {
			lines = append(lines, diffText("+", fmt.Sprintf("%-3s %-38s Rs %d", fmt.Sprintf("%dx", item.Quantity), item.Name, item.FinalPrice)))
		}
	}
	lines = append(lines, mutedStyle.Render("diff"))
	for _, bill := range cart.Bill.Lines {
		lines = append(lines, diffText(billLineSign(bill), fmt.Sprintf("%-43s %s", bill.Label, displayBillValue(bill.Value))))
	}
	toPayLabel := defaultString(cart.Bill.ToPayLabel, "To Pay")
	toPayValue := cart.Bill.ToPayValue
	if toPayValue == "" && cart.TotalRupees > 0 {
		toPayValue = fmt.Sprintf("Rs %d", cart.TotalRupees)
	}
	lines = append(lines, diffText("+", boldStyle.Render(fmt.Sprintf("%-43s %s", toPayLabel, toPayValue))))
	lines = append(lines, mutedStyle.Render("payment")+" "+defaultString(strings.Join(cart.AvailablePaymentMethods, ", "), "none"), mutedStyle.Render("next")+" p deploy gate")
	return lines
}

func displayBillValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "+-")
	return strings.TrimSpace(value)
}

func diffLine(sign, body string) string {
	return line(diffText(sign, body))
}

func diffText(sign, body string) string {
	marker := successStyle.Render(sign)
	rowStyle := diffAddStyle
	if sign == "-" {
		marker = errorStyle.Render(sign)
		rowStyle = diffDelStyle
	}
	text := " " + marker + " " + body
	if width := lipgloss.Width(text); width < innerWidth-1 {
		text += strings.Repeat(" ", innerWidth-1-width)
	}
	return rowStyle.Render(text)
}

func billLineSign(bill domaininstamart.BillLine) string {
	value := strings.TrimSpace(bill.Value)
	if strings.HasPrefix(value, "-") {
		return "-"
	}
	label := strings.ToLower(bill.Label)
	for _, token := range []string{"discount", "saving", "coupon", "offer", "cashback"} {
		if strings.Contains(label, token) {
			return "-"
		}
	}
	return "+"
}

func (m instamartModel) renderCheckoutConfirm(sb *strings.Builder) {
	sb.WriteString(centeredLine("Are you sure you want to push --force groceries?"))
	sb.WriteString(centeredLine(brandStyle.Render("git push --force groceries")))
	sb.WriteString(line(""))
	sb.WriteString(centeredLine(defaultString(m.currentCart.AddressLabel, selectedAddressLabel(m.selectedAddress))))
	sb.WriteString(centeredLine("payment " + m.paymentMethod + " · total " + fmt.Sprintf("Rs %d", cartToPayRupees(m.currentCart))))
	sb.WriteString(line(""))
	sb.WriteString(centeredLine(mutedStyle.Render("deploy gate")))
	if len(m.currentCart.StoreIDs) > 1 {
		sb.WriteString(centeredLine(accentStyle.Render(fmt.Sprintf("%d stores; Swiggy may split deploy", len(m.currentCart.StoreIDs)))))
	}
	sb.WriteString(line(""))
	sb.WriteString(centeredLine("y deploy / n cancel"))
}

func (m instamartModel) renderOrderResult(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" deploy logs")))
	sb.WriteString(line(" [ok] git push --force origin groceries"))
	message := m.checkoutResult.Message
	if message == "" {
		message = "Checkout completed."
	}
	sb.WriteString(line(" " + successStyle.Render("[ok] "+message)))
	if m.checkoutResult.Status != "" {
		sb.WriteString(line(" [ok] status: " + m.checkoutResult.Status))
	}
	if m.checkoutResult.PaymentMethod != "" {
		sb.WriteString(line(" [ok] payment method: " + m.checkoutResult.PaymentMethod))
	}
	for _, orderID := range m.checkoutResult.OrderIDs {
		sb.WriteString(line(" [info] order_id=" + orderID))
	}
	stores := receiptStoreCount(m.checkoutResult)
	if stores > 0 {
		sb.WriteString(line(fmt.Sprintf(" [info] stores=%d", stores)))
	}
	if m.checkoutResult.CartTotal > 0 {
		sb.WriteString(line(fmt.Sprintf(" [info] total=Rs %d", m.checkoutResult.CartTotal)))
	}
	if m.checkoutElapsed > 0 {
		sb.WriteString(line(" [info] deployed_in=" + formatElapsed(m.checkoutElapsed)))
	}
}

func okLine(label string, ok bool) string {
	if ok {
		return "[ok] " + label
	}
	return "[wait] " + label
}

func receiptStoreCount(result domaininstamart.CheckoutResult) int {
	if result.MultiStore && len(result.OrderIDs) > 0 {
		return len(result.OrderIDs)
	}
	if result.MultiStore {
		return 2
	}
	if len(result.OrderIDs) > 0 {
		return 1
	}
	return 0
}
