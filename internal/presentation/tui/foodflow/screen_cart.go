package foodflow

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	domainfood "swiggy-ssh/internal/domain/food"
)

func (m foodModel) renderCartReview(sb *strings.Builder) {
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

func (m foodModel) cartReviewOverflows() bool {
	return len(m.cartReviewLines()) > bodyRows
}

func (m foodModel) cartReviewLines() []string {
	cart := m.currentCart
	restLabel := defaultString(cart.RestaurantName, "restaurant")
	if m.selectedRestaurant != nil {
		restLabel = m.selectedRestaurant.Name
	}
	lines := []string{
		brandStyle.Render(" GET /food/cart 200 OK"),
		mutedStyle.Render("context.restaurant") + " " + boldStyle.Render(restLabel),
		mutedStyle.Render("context.address_id") + " " + boldStyle.Render(m.selectedAddressID()),
	}
	lines = append(lines, mutedStyle.Render("response.items"))
	if len(cart.Items) == 0 {
		lines = append(lines, " []")
	} else {
		for _, item := range cart.Items {
			lines = append(lines, foodDiffText("+", fmt.Sprintf("%-3s %-38s Rs %d", fmt.Sprintf("%dx", item.Quantity), item.Name, item.FinalPrice)))
		}
	}
	lines = append(lines, mutedStyle.Render("response.bill"))
	for _, bl := range cart.Bill.Lines {
		lines = append(lines, foodDiffText(foodBillLineSign(bl), fmt.Sprintf("%-43s %s", bl.Label, foodDisplayBillValue(bl.Value))))
	}
	toPayLabel := defaultString(cart.Bill.ToPayLabel, "To Pay")
	toPayValue := cart.Bill.ToPayValue
	if toPayValue == "" && cart.TotalRupees > 0 {
		toPayValue = fmt.Sprintf("Rs %d", cart.TotalRupees)
	}
	lines = append(lines, foodDiffText("+", boldStyle.Render(fmt.Sprintf("%-43s %s", toPayLabel, toPayValue))))
	lines = append(lines,
		mutedStyle.Render("available_payment_methods")+" "+defaultString(strings.Join(cart.AvailablePaymentMethods, ", "), "none"),
		mutedStyle.Render("next")+" c GET /food/coupons · p POST /food/checkout",
	)
	return lines
}

func foodDisplayBillValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "+-")
	return strings.TrimSpace(value)
}

func foodDiffText(sign, body string) string {
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

func foodBillLineSign(bl domainfood.BillLine) string {
	value := strings.TrimSpace(bl.Value)
	if strings.HasPrefix(value, "-") {
		return "-"
	}
	label := strings.ToLower(bl.Label)
	for _, token := range []string{"discount", "saving", "coupon", "offer", "cashback"} {
		if strings.Contains(label, token) {
			return "-"
		}
	}
	return "+"
}

func (m foodModel) renderCheckoutConfirm(sb *strings.Builder) {
	restName := "the restaurant"
	if m.selectedRestaurant != nil {
		restName = m.selectedRestaurant.Name
	} else if m.currentCart.RestaurantName != "" {
		restName = m.currentCart.RestaurantName
	}
	sb.WriteString(centeredLine(brandStyle.Render("POST /food/checkout confirm required")))
	sb.WriteString(centeredLine("request.restaurant " + restName))
	sb.WriteString(line(""))
	sb.WriteString(centeredLine("request.address_id " + m.selectedAddressID()))
	sb.WriteString(centeredLine("request.payment_method " + m.paymentMethod + " · total " + fmt.Sprintf("Rs %d", foodCartToPayRupees(m.currentCart))))
	sb.WriteString(line(""))
	sb.WriteString(centeredLine(mutedStyle.Render("risk: irreversible checkout request · explicit y required")))
	sb.WriteString(line(""))
	sb.WriteString(centeredLine("y send request / n cancel"))
}

func (m foodModel) renderOrderResult(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" POST /food/checkout 201 Created")))
	sb.WriteString(line(" [ok] response.created=true"))
	msg := m.orderResult.Message
	if msg == "" {
		msg = "Order placed."
	}
	sb.WriteString(line(" " + successStyle.Render("[ok] "+msg)))
	if m.orderResult.Status != "" {
		sb.WriteString(line(" [ok] response.status=" + m.orderResult.Status))
	}
	if m.orderResult.PaymentMethod != "" {
		sb.WriteString(line(" [ok] response.payment_method=" + m.orderResult.PaymentMethod))
	}
	if m.orderResult.OrderID != "" {
		sb.WriteString(line(" [info] order_id=" + m.orderResult.OrderID))
	}
	if m.orderResult.CartTotal > 0 {
		sb.WriteString(line(fmt.Sprintf(" [info] total=Rs %d", m.orderResult.CartTotal)))
	}
	if m.orderElapsed > 0 {
		sb.WriteString(line(" [info] response.elapsed=" + formatElapsed(m.orderElapsed)))
	}
	sb.WriteString(line(""))
	sb.WriteString(line(mutedStyle.Render(" To cancel: call Swiggy customer care at 080-67466729")))
}

func (m foodModel) renderCoupons(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" GET /food/coupons 200 OK")))
	if m.coupons.Applicable > 0 {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" response.coupons · %d applicable coupon(s)", m.coupons.Applicable))))
	} else {
		sb.WriteString(line(mutedStyle.Render(" response.coupons")))
	}
	if len(m.coupons.Coupons) == 0 {
		sb.WriteString(line(" No coupons available."))
		return
	}
	sb.WriteString(line(mutedStyle.Render(" enter sends POST /food/cart/coupon")))
	for i, coupon := range m.coupons.Coupons {
		applicableStr := ""
		if !coupon.Applicable {
			applicableStr = " [not applicable]"
		}
		discountStr := ""
		if coupon.Discount > 0 {
			discountStr = fmt.Sprintf(" · saves Rs %d", coupon.Discount)
		}
		label := fmt.Sprintf("%-14s %s%s%s", coupon.Code, truncateFoodTerminal(coupon.Description, 35), discountStr, applicableStr)
		if !coupon.Applicable {
			label = mutedStyle.Render(label)
		}
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}
