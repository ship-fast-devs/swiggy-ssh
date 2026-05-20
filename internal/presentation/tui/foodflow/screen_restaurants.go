package foodflow

import (
	"fmt"
	"strings"

	domainfood "swiggy-ssh/internal/domain/food"
)

const restaurantListRows = 10

func (m foodModel) renderRestaurantSearch(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" search restaurants")))
	sb.WriteString(line(""))
	sb.WriteString(line(" query: " + boldStyle.Render(m.searchQuery) + cursorStyle.Render("_")))

	if m.searchPreviewLoading {
		frame := foodSearchSpinnerFrames[m.searchPreviewSpinner%len(foodSearchSpinnerFrames)]
		sb.WriteString(line(""))
		sb.WriteString(line(" " + frame + " searching restaurants..."))
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
	sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" preview · enter opens results · %d matches in %s", len(m.searchPreviewRestaurants), formatElapsed(m.searchPreviewElapsed)))))
	if len(m.searchPreviewRestaurants) == 0 {
		sb.WriteString(line(" No matching restaurants found yet."))
		return
	}
	renderPreviewRestaurantTable(sb, m.searchPreviewRestaurants, 4)
	if len(m.searchPreviewRestaurants) > 4 {
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" ...and %d more", len(m.searchPreviewRestaurants)-4))))
	}
}

func renderPreviewRestaurantTable(sb *strings.Builder, restaurants []domainfood.Restaurant, limit int) {
	sb.WriteString(line("   status  name                            rating  eta"))
	for i, r := range restaurants {
		if limit > 0 && i >= limit {
			break
		}
		label := restaurantPreviewRow(r)
		if strings.ToUpper(r.Availability) != "OPEN" {
			label = mutedStyle.Render(label)
		}
		sb.WriteString(line("   " + label))
	}
}

func restaurantPreviewRow(r domainfood.Restaurant) string {
	status := "OPEN "
	if strings.ToUpper(r.Availability) != "OPEN" {
		status = "CLOS "
	}
	name := truncateFoodTerminal(r.Name, 30)
	rating := r.Rating
	if rating == "" {
		rating = "-"
	}
	eta := r.ETA
	if eta == "" {
		eta = "-"
	}
	return fmt.Sprintf("%-5s %-30s %-7s %s", status, name, rating+"★", eta)
}

func (m foodModel) renderRestaurantList(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" restaurant results")))
	if len(m.restaurants) > restaurantListRows {
		start := restaurantWindowStart(m.cursor, len(m.restaurants), restaurantListRows)
		end := start + restaurantListRows
		if end > len(m.restaurants) {
			end = len(m.restaurants)
		}
		sb.WriteString(line(mutedStyle.Render(fmt.Sprintf(" showing %d-%d of %d · OPEN restaurants only", start+1, end, len(m.restaurants)))))
	} else {
		sb.WriteString(line(mutedStyle.Render(" select an OPEN restaurant")))
	}
	renderRestaurantTable(sb, m.restaurants, m.cursor, restaurantListRows)
}

func renderRestaurantTable(sb *strings.Builder, restaurants []domainfood.Restaurant, cursor, limit int) {
	sb.WriteString(line("   status  name                     cuisine       rating  eta"))
	start := restaurantWindowStart(cursor, len(restaurants), limit)
	end := len(restaurants)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	for i := start; i < end; i++ {
		r := restaurants[i]
		label := restaurantTableRow(r)
		isOpen := strings.ToUpper(r.Availability) == "OPEN"
		if !isOpen {
			label = mutedStyle.Render(label)
		}
		if i == cursor {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func restaurantTableRow(r domainfood.Restaurant) string {
	status := "OPEN "
	if strings.ToUpper(r.Availability) != "OPEN" {
		status = "[x]  "
	}
	if r.IsAd {
		status = "[ad] "
	}
	name := truncateFoodTerminal(r.Name, 23)
	cuisine := truncateFoodTerminal(r.Cuisines, 12)
	rating := r.Rating
	if rating == "" {
		rating = "-"
	}
	eta := r.ETA
	if eta == "" {
		eta = "-"
	}
	price := r.PriceForTwo
	if price != "" {
		price = " · " + price
	}
	return fmt.Sprintf("%-5s %-23s %-12s %-7s %s%s", status, name, cuisine, rating+"★", eta, price)
}

func (m foodModel) renderItemDetail(sb *strings.Builder) {
	if m.selectedItem == nil {
		sb.WriteString(line(" No item selected."))
		return
	}
	item := m.selectedItem
	sb.WriteString(line(brandStyle.Render(" item detail")))
	vegIcon := "🔴"
	if item.IsVeg {
		vegIcon = "🟢"
	}
	sb.WriteString(line(" " + vegIcon + " " + boldStyle.Render(item.Name)))
	if item.Price > 0 {
		sb.WriteString(line(" price: Rs " + fmt.Sprintf("%d", item.Price)))
	}
	if item.Rating != "" {
		sb.WriteString(line(" rating: " + item.Rating + "★"))
	}
	if item.Description != "" {
		sb.WriteString(line(" " + mutedStyle.Render(truncateFoodTerminal(item.Description, 70))))
	}

	groups := allVariantGroups(*item)
	if len(groups) == 0 {
		sb.WriteString(line(""))
		sb.WriteString(line(" No variants — press enter to add to cart."))
		return
	}

	sb.WriteString(line(""))
	sb.WriteString(line(mutedStyle.Render(" choose variants · tab switches group · j/k moves · enter adds to cart")))
	for gi, group := range groups {
		isActiveGroup := gi == (m.cursor % len(groups))
		groupLabel := group.Name
		if isActiveGroup {
			groupLabel = accentStyle.Render(groupLabel)
		} else {
			groupLabel = mutedStyle.Render(groupLabel)
		}
		sb.WriteString(line(" " + groupLabel + ":"))
		for vi, v := range group.Variants {
			varCursor := 0
			if gi < len(m.variantCursors) {
				varCursor = m.variantCursors[gi]
			}
			if isActiveGroup && vi == varCursor {
				sb.WriteString(line(cursorStyle.Render("   > ") + boldStyle.Render(v.Label)))
			} else {
				sb.WriteString(line("     " + v.Label))
			}
		}
	}
}

func restaurantWindowStart(cursor, total, limit int) int {
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

func truncateFoodTerminal(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
