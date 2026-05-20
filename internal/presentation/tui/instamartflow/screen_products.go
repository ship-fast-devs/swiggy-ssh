package instamartflow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const productListRows = 9

func (m instamartModel) renderSearch(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" GET /instamart/search")))
	sb.WriteString(line(""))
	sb.WriteString(line(" params" + m.searchAPIStatus()))
	sb.WriteString(line(" query: " + boldStyle.Render(m.searchQuery) + cursorStyle.Render("_")))
	sb.WriteString(line(" address_id: " + m.selectedAddressID()))

	if m.searchPreviewDebouncing {
		sb.WriteString(line(""))
		sb.WriteString(line(" debounce active..."))
		return
	}
	if m.searchPreviewLoading {
		frame := searchSpinnerFrames[m.searchPreviewSpinner%len(searchSpinnerFrames)]
		sb.WriteString(line(""))
		sb.WriteString(line(" " + frame + " calling search endpoint..."))
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
	sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" response preview · enter opens response.products · %d matches", len(m.searchPreviewRows)))))
	if len(m.searchPreviewRows) == 0 {
		sb.WriteString(line(" No matching products found yet."))
		return
	}
	renderPreviewProductTable(sb, m.searchPreviewRows, 5)
	if len(m.searchPreviewRows) > 5 {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" ...and %d more", len(m.searchPreviewRows)-5))))
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

func renderPreviewProductTable(sb *strings.Builder, rows []productVariationRow, limit int) {
	sb.WriteString(line("   type item                         pack      price"))
	for i, row := range rows {
		if limit > 0 && i >= limit {
			break
		}
		label := productPreviewRow(row)
		if !productRowAvailable(row) {
			label = mutedStyle.Render(label)
		}
		sb.WriteString(line("   " + label))
	}
}

func productPreviewRow(row productVariationRow) string {
	name := defaultString(row.Variation.DisplayName, row.Product.DisplayName)
	if row.Product.Promoted {
		name = "[ad] " + name
	}
	pack := defaultString(row.Variation.QuantityDescription, "-")
	price := fmt.Sprintf("Rs %d", row.Variation.Price.OfferPrice)
	if !productRowAvailable(row) {
		price = "[x] unavailable"
	}
	return fmt.Sprintf("%-4s %-28s %-9s %s", productRowIcon(row), truncateTerminal(name, 28), truncateTerminal(pack, 9), price)
}

func (m instamartModel) renderProducts(sb *strings.Builder) {
	title := "GET /instamart/search?query=" + m.searchQuery + " 200 OK"
	if strings.TrimSpace(m.searchQuery) == "" {
		title = "GET /instamart/products/recent 200 OK"
	}
	sb.WriteString(line(brandStyle.Render(" " + title)))
	if len(m.rows) > productListRows {
		start := productWindowStart(m.cursor, len(m.rows), productListRows)
		end := start + productListRows
		if end > len(m.rows) {
			end = len(m.rows)
		}
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" response.products · choose exact pack · showing %d-%d of %d", start+1, end, len(m.rows)))))
	} else {
		sb.WriteString(line(mutedStyle.Render(" response.products · choose exact pack")))
	}
	renderProductTable(sb, m.rows, m.cursor, productListRows)
}

func renderProductTable(sb *strings.Builder, rows []productVariationRow, cursor, limit int) {
	sb.WriteString(line("   type item                         pack      price"))
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

func productTableRow(_ int, row productVariationRow) string {
	name := defaultString(row.Variation.DisplayName, row.Product.DisplayName)
	if row.Product.Promoted {
		name = "[ad] " + name
	}
	pack := defaultString(row.Variation.QuantityDescription, "-")
	price := fmt.Sprintf("Rs %d", row.Variation.Price.OfferPrice)
	if !productRowAvailable(row) {
		price = "[x] unavailable"
	}
	return fmt.Sprintf("%-4s %-28s %-9s %s", productRowIcon(row), truncateTerminal(name, 28), truncateTerminal(pack, 9), price)
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
	if m.selectedRow == nil {
		sb.WriteString(line(" No variation selected."))
		return
	}
	sb.WriteString(line(brandStyle.Render(" POST /instamart/cart/items draft")))
	status := "available"
	statusStyle := successStyle
	if !productRowAvailable(*m.selectedRow) {
		status = "unavailable"
		statusStyle = errorStyle
	}
	sb.WriteString(yamlLine("spin_id", m.selectedRow.Variation.SpinID, yamlValStyle))
	sb.WriteString(yamlLine("item", defaultString(m.selectedRow.Variation.DisplayName, m.selectedRow.Product.DisplayName), yamlValStyle))
	sb.WriteString(yamlLine("pack", defaultString(m.selectedRow.Variation.QuantityDescription, "-"), yamlValStyle))
	sb.WriteString(yamlLine("price", fmt.Sprintf("Rs %d", m.selectedRow.Variation.Price.OfferPrice), yamlValStyle))
	sb.WriteString(yamlLine("status", status, statusStyle))
	sb.WriteString(yamlLine("quantity", strconv.Itoa(m.quantity), successStyle))
	sb.WriteString(line(""))
	sb.WriteString(line(" effect: enter sends the full intended cart."))
	sb.WriteString(line(" effect: quantity 0 removes this variation."))
}

func yamlLine(key, value string, valueStyle lipgloss.Style) string {
	return line(" " + yamlKeyStyle.Render(key+":") + " " + valueStyle.Render(value))
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
