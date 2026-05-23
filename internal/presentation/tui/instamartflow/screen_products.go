package instamartflow

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const productListRows = 9
const quantityModalInnerWidth = 36
const quantityModalTopPadding = 4

func (m instamartModel) renderSearch(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" grep groceries")))
	sb.WriteString(line(""))
	sb.WriteString(line(" query" + m.searchAPIStatus()))
	sb.WriteString(line(" pattern: " + boldStyle.Render(m.searchQuery) + cursorStyle.Render("_")))
	sb.WriteString(line(" address_id: " + m.selectedAddressID()))

	if m.searchPreviewDebouncing {
		sb.WriteString(line(""))
		sb.WriteString(line(" debounce: indexing pantry..."))
		return
	}
	if m.searchPreviewLoading {
		frame := searchSpinnerFrames[m.searchPreviewSpinner%len(searchSpinnerFrames)]
		sb.WriteString(line(""))
		sb.WriteString(line(" " + frame + " grep groceries --live..."))
		return
	}
	if m.searchPreviewErr != "" {
		sb.WriteString(line(""))
		sb.WriteString(line(" " + errorStyle.Render(m.searchPreviewErr)))
		return
	}
	if !m.searchPreviewLoaded || m.searchPreviewQuery != m.searchQuery {
		return
	}

	sb.WriteString(line(""))
	sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" live preview · up/down selects · enter edits quantity · %d matches", len(m.searchPreviewRows)))))
	if len(m.searchPreviewRows) == 0 {
		sb.WriteString(line(" No matching products found yet."))
		return
	}
	renderPreviewProductTable(sb, m.searchPreviewRows, m.cursor, 5)
	if len(m.searchPreviewRows) > 5 {
		start := productWindowStart(m.cursor, len(m.searchPreviewRows), 5)
		end := start + 5
		if end > len(m.searchPreviewRows) {
			end = len(m.searchPreviewRows)
		}
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" showing %d-%d of %d", start+1, end, len(m.searchPreviewRows)))))
	}
}

func (m instamartModel) searchAPIStatus() string {
	if m.searchPreviewLoading {
		return mutedStyle.Render("  calling...")
	}
	if m.searchPreviewErr != "" {
		return errorStyle.Render("  error")
	}
	if m.searchPreviewLoaded && m.searchPreviewQuery == m.searchQuery {
		return mutedStyle.Render("  200 OK · " + formatElapsed(m.searchPreviewElapsed))
	}
	return ""
}

func renderPreviewProductTable(sb *strings.Builder, rows []productVariationRow, cursor, limit int) {
	sb.WriteString(line("   # item                         pack      price"))
	start := productWindowStart(cursor, len(rows), limit)
	end := len(rows)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	for i := start; i < end; i++ {
		row := rows[i]
		label := productPreviewRow(i, row)
		if !productRowAvailable(row) {
			label = mutedStyle.Render(label)
		}
		if i == cursor {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func productPreviewRow(index int, row productVariationRow) string {
	name := defaultString(row.Variation.DisplayName, row.Product.DisplayName)
	if row.Product.Promoted {
		name = "[ad] " + name
	}
	pack := defaultString(row.Variation.QuantityDescription, "-")
	price := fmt.Sprintf("Rs %d", row.Variation.Price.OfferPrice)
	if !productRowAvailable(row) {
		price = "[x] unavailable"
	}
	return fmt.Sprintf("%d  %-28s %-9s %s", index+1, truncateTerminal(name, 28), truncateTerminal(pack, 9), price)
}

func (m instamartModel) renderProducts(sb *strings.Builder) {
	title := "GET /instamart/search?query=" + m.searchQuery + " 200 OK"
	if strings.TrimSpace(m.searchQuery) == "" {
		title = "git add from recent cache"
	}
	sb.WriteString(line(brandStyle.Render(" " + title)))
	if len(m.rows) > productListRows {
		start := productWindowStart(m.cursor, len(m.rows), productListRows)
		end := start + productListRows
		if end > len(m.rows) {
			end = len(m.rows)
		}
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" 1-9/enter opens quantity · +/- opens adjusted quantity · showing %d-%d of %d", start+1, end, len(m.rows)))))
	} else {
		sb.WriteString(line(mutedStyle.Render(" 1-9/enter opens quantity · +/- opens adjusted quantity")))
	}
	renderProductTable(sb, m.rows, m.cursor, productListRows)
}

func renderProductTable(sb *strings.Builder, rows []productVariationRow, cursor, limit int) {
	sb.WriteString(line("   # item                         pack      price"))
	start := productWindowStart(cursor, len(rows), limit)
	end := len(rows)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	for i := start; i < end; i++ {
		row := rows[i]
		label := productTableRow(i-start, row)
		if !productRowAvailable(row) {
			label = mutedStyle.Render(label)
		}
		if i == cursor {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func productTableRow(index int, row productVariationRow) string {
	name := defaultString(row.Variation.DisplayName, row.Product.DisplayName)
	if row.Product.Promoted {
		name = "[ad] " + name
	}
	pack := defaultString(row.Variation.QuantityDescription, "-")
	price := fmt.Sprintf("Rs %d", row.Variation.Price.OfferPrice)
	if !productRowAvailable(row) {
		price = "[x] unavailable"
	}
	return fmt.Sprintf("%d  %-28s %-9s %s", index+1, truncateTerminal(name, 28), truncateTerminal(pack, 9), price)
}

func productRowIcon(row productVariationRow) string {
	if !productRowAvailable(row) {
		return "×"
	}
	if row.Product.Promoted {
		return "◆"
	}
	return "▦"
}

func productWindowStart(cursor, total, limit int) int {
	if limit <= 0 || total <= limit {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	if cursor >= limit {
		return cursor - limit + 1
	}
	return 0
}

func (m instamartModel) renderQuantity(sb *strings.Builder) {
	for i := 0; i < quantityModalTopPadding; i++ {
		sb.WriteString(line(""))
	}
	for _, modalLine := range strings.Split(strings.TrimSuffix(m.renderQuantityModal(), "\r\n"), "\r\n") {
		renderQuantityModalLine(sb, modalLine)
	}
}

func (m instamartModel) renderQuantityModal() string {
	if m.selectedRow == nil {
		return "┌─ " + brandStyle.Render("Add to cart") + " " + strings.Repeat("─", 24) + "┐\r\n" +
			quantityModalBody("No variation selected.") + "\r\n" +
			"└" + strings.Repeat("─", quantityModalInnerWidth+2) + "┘\r\n"
	}
	row := *m.selectedRow
	name := truncateTerminal(defaultString(row.Variation.DisplayName, row.Product.DisplayName), quantityModalInnerWidth)
	pack := truncateTerminal(defaultString(row.Variation.QuantityDescription, "-"), 17)
	price := fmt.Sprintf("₹%d", row.Variation.Price.OfferPrice)
	status := "available"
	statusStyle := successStyle
	if !productRowAvailable(row) {
		status = "unavailable"
		statusStyle = errorStyle
	}
	action := "enter add/update   esc cancel"
	if m.quantity == 0 {
		action = "enter remove       esc cancel"
	}

	var sb strings.Builder
	sb.WriteString("┌─ " + brandStyle.Render("Add to cart") + " " + strings.Repeat("─", 24) + "┐\r\n")
	sb.WriteString(quantityModalBody(name) + "\r\n")
	sb.WriteString(quantityModalBody(fmt.Sprintf("Pack: %-17s %9s", pack, price)) + "\r\n")
	sb.WriteString(quantityModalBody("Status: "+statusStyle.Render(status)) + "\r\n")
	sb.WriteString(quantityModalBody("") + "\r\n")
	sb.WriteString(quantityModalBody(fmt.Sprintf("Quantity:        [-]  %d  [+]", m.quantity)) + "\r\n")
	if m.quantity == 0 {
		sb.WriteString(quantityModalBody(mutedStyle.Render("0 means remove this item")) + "\r\n")
	} else {
		sb.WriteString(quantityModalBody("") + "\r\n")
	}
	sb.WriteString(quantityModalBody(mutedStyle.Render(action)) + "\r\n")
	sb.WriteString("└" + strings.Repeat("─", quantityModalInnerWidth+2) + "┘\r\n")
	return sb.String()
}

func quantityModalBody(content string) string {
	w := lipgloss.Width(content)
	if w > quantityModalInnerWidth {
		runes := []rune(content)
		content = string(runes[:quantityModalInnerWidth])
		w = quantityModalInnerWidth
	}
	return "│ " + content + strings.Repeat(" ", quantityModalInnerWidth-w) + " │"
}

func renderQuantityModalLine(sb *strings.Builder, content string) {
	pad := (innerWidth - lipgloss.Width(content)) / 2
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(line(strings.Repeat(" ", pad) + content))
}

func productRowAvailable(row productVariationRow) bool {
	return row.Product.InStock && row.Product.Available && row.Variation.InStock
}

func truncateTerminal(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
