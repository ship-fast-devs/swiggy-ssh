package foodflow

import (
	"fmt"
	"strings"

	domainfood "swiggy-ssh/internal/domain/food"
)

const menuBrowseRows = 9

func (m foodModel) renderMenuBrowse(sb *strings.Builder) {
	restName := ""
	if m.selectedRestaurant != nil {
		restName = " · " + m.selectedRestaurant.Name
	} else if m.menuPage.RestaurantName != "" {
		restName = " · " + m.menuPage.RestaurantName
	}
	sb.WriteString(line(brandStyle.Render(" menu"+restName)))

	items := m.allMenuItems()
	if len(items) == 0 {
		sb.WriteString(line(mutedStyle.Render(" No items found in this menu.")))
		return
	}

	if len(items) > menuBrowseRows {
		start := restaurantWindowStart(m.menuCursor, len(items), menuBrowseRows)
		end := start + menuBrowseRows
		if end > len(items) {
			end = len(items)
		}
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" showing %d-%d of %d items · s to search", start+1, end, len(items)))))
	} else {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" %d items · s to search", len(items)))))
	}

	renderMenuItemTable(sb, items, m.menuCursor, menuBrowseRows)
}

func renderMenuItemTable(sb *strings.Builder, items []domainfood.MenuItem, cursor, limit int) {
	sb.WriteString(line("   veg  name                           rating  price"))
	start := restaurantWindowStart(cursor, len(items), limit)
	end := len(items)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	for i := start; i < end; i++ {
		item := items[i]
		label := menuItemTableRow(item)
		if i == cursor {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func menuItemTableRow(item domainfood.MenuItem) string {
	vegMark := "●"
	if item.IsVeg {
		vegMark = "○"
	}
	name := truncateFoodTerminal(item.Name, 30)
	rating := item.Rating
	if rating == "" {
		rating = "-"
	}
	variants := ""
	if item.HasVariants {
		variants = " +"
	}
	return fmt.Sprintf("%-3s %-30s %-7s Rs %d%s", vegMark, name, rating+"★", item.Price, variants)
}

func (m foodModel) renderMenuSearch(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" search dish")))
	sb.WriteString(line(""))
	if m.selectedRestaurant != nil {
		sb.WriteString(line(mutedStyle.Render(" searching in: " + m.selectedRestaurant.Name)))
	} else {
		sb.WriteString(line(mutedStyle.Render(" searching across all restaurants")))
	}
	sb.WriteString(line(" query: " + boldStyle.Render(m.searchQuery) + cursorStyle.Render("_")))

	if m.searchPreviewLoading {
		frame := foodSearchSpinnerFrames[m.searchPreviewSpinner%len(foodSearchSpinnerFrames)]
		sb.WriteString(line(""))
		sb.WriteString(line(" " + frame + " searching dishes..."))
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
	sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" preview · enter opens results · %d matches in %s", len(m.searchPreviewMenuItems), formatElapsed(m.searchPreviewElapsed)))))
	if len(m.searchPreviewMenuItems) == 0 {
		sb.WriteString(line(" No matching dishes found yet."))
		return
	}
	renderPreviewMenuItemTable(sb, m.searchPreviewMenuItems, 4)
	if len(m.searchPreviewMenuItems) > 4 {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" ...and %d more", len(m.searchPreviewMenuItems)-4))))
	}
}

func renderPreviewMenuItemTable(sb *strings.Builder, items []domainfood.MenuItemDetail, limit int) {
	sb.WriteString(line("   veg  name                           rating  price"))
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		label := menuItemDetailPreviewRow(item)
		sb.WriteString(line("   " + label))
	}
}

func menuItemDetailPreviewRow(item domainfood.MenuItemDetail) string {
	vegMark := "●"
	if item.IsVeg {
		vegMark = "○"
	}
	name := truncateFoodTerminal(item.Name, 30)
	rating := item.Rating
	if rating == "" {
		rating = "-"
	}
	variants := ""
	if hasVariants(item) {
		variants = " +"
	}
	return fmt.Sprintf("%-3s %-30s %-7s Rs %d%s", vegMark, name, rating+"★", item.Price, variants)
}

func (m foodModel) renderMenuResults(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" dish results")))
	if len(m.menuItems) > menuBrowseRows {
		start := restaurantWindowStart(m.cursor, len(m.menuItems), menuBrowseRows)
		end := start + menuBrowseRows
		if end > len(m.menuItems) {
			end = len(m.menuItems)
		}
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" showing %d-%d of %d · + means has variants", start+1, end, len(m.menuItems)))))
	} else {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" %d dishes found · + means has variants", len(m.menuItems)))))
	}
	renderMenuDetailTable(sb, m.menuItems, m.cursor, menuBrowseRows)
}

func renderMenuDetailTable(sb *strings.Builder, items []domainfood.MenuItemDetail, cursor, limit int) {
	sb.WriteString(line("   veg  name                           rating  price"))
	start := restaurantWindowStart(cursor, len(items), limit)
	end := len(items)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	for i := start; i < end; i++ {
		item := items[i]
		label := menuItemDetailPreviewRow(item)
		if i == cursor {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

// categoryHeader renders a category name as a section header.
func categoryHeader(name string) string {
	return line(accentStyle.Render(" ── " + strings.ToUpper(name) + " ──"))
}
