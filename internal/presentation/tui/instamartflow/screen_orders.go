package instamartflow

import (
	"fmt"
	"strings"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

func (m instamartModel) renderOrders(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" deploy history")))
	if len(m.orders.Orders) == 0 {
		sb.WriteString(line(" No matching orders found."))
		return
	}
	for i, order := range m.orders.Orders {
		active := ""
		if order.Active {
			active = " · active"
		}
		label := fmt.Sprintf("%s  %s · %d items · Rs %d%s", orderIcon(order), order.Status, order.ItemCount, order.TotalRupees, active)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
	if m.cursor >= 0 && m.cursor < len(m.orders.Orders) && m.orders.Orders[m.cursor].Location == nil {
		sb.WriteString(line(""))
		sb.WriteString(line(" Tracking is unavailable for this order in the terminal. Please check the Swiggy Instamart app."))
	}
}

func orderIcon(order domaininstamart.OrderSummary) string {
	if order.Active {
		return "◷"
	}
	return "✓"
}

func (m instamartModel) renderTracking(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" tail active order")))
	if m.tracking.StatusMessage == "" {
		sb.WriteString(line(" Tracking is unavailable for this order in the terminal. Please check the Swiggy Instamart app."))
		return
	}
	sb.WriteString(line(" " + successStyle.Render(m.tracking.StatusMessage)))
	if m.tracking.SubStatusMessage != "" {
		sb.WriteString(line(" " + m.tracking.SubStatusMessage))
	}
	if m.tracking.ETAText != "" {
		sb.WriteString(line(" ETA: " + m.tracking.ETAText))
	}
}
