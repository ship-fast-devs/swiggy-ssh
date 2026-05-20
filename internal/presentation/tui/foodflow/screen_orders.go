package foodflow

import (
	"fmt"
	"strings"

	domainfood "swiggy-ssh/internal/domain/food"
)

func (m foodModel) renderOrders(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" order history")))
	if len(m.orders.Orders) == 0 {
		sb.WriteString(line(" No orders found."))
		return
	}
	for i, order := range m.orders.Orders {
		active := ""
		if order.Active {
			active = " · active"
		}
		label := fmt.Sprintf("%s  %-28s · %s · Rs %d%s",
			foodOrderIcon(order),
			truncateFoodTerminal(order.RestaurantName, 28),
			order.Status,
			order.TotalRupees,
			active,
		)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
	if m.cursor >= 0 && m.cursor < len(m.orders.Orders) {
		order := m.orders.Orders[m.cursor]
		if !order.Active {
			sb.WriteString(line(""))
			sb.WriteString(line(mutedStyle.Render(" Tracking is only available for active orders.")))
		}
	}
}

func foodOrderIcon(order domainfood.FoodOrderSummary) string {
	if order.Active {
		return "◷"
	}
	return "✓"
}

func (m foodModel) renderTracking(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" tail active order")))
	if m.tracking.StatusMessage == "" {
		sb.WriteString(line(" Tracking is unavailable for this order. Please check the Swiggy app."))
		return
	}
	sb.WriteString(line(" " + successStyle.Render(m.tracking.StatusMessage)))
	if m.tracking.SubStatusMessage != "" {
		sb.WriteString(line(" " + m.tracking.SubStatusMessage))
	}
	if m.tracking.ETAText != "" {
		sb.WriteString(line(" ETA: " + m.tracking.ETAText))
	} else if m.tracking.ETAMinutes > 0 {
		sb.WriteString(line(fmt.Sprintf(" ETA: ~%d min", m.tracking.ETAMinutes)))
	}
	if m.tracking.OrderID != "" {
		sb.WriteString(line(""))
		sb.WriteString(line(mutedStyle.Render(" order_id=" + m.tracking.OrderID)))
	}
	sb.WriteString(line(""))
	sb.WriteString(line(mutedStyle.Render(" To cancel: call Swiggy customer care at 080-67466729")))
	_ = strings.TrimSpace // used in other files
}
