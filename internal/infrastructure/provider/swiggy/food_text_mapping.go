package swiggy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	foodRestaurantLinePattern     = regexp.MustCompile(`^\s*\d+\.\s+(.+?)\s+—\s+(.+?)\s+\|\s+([^|]+)\|\s+([^|]+)\|\s+(.+?)\s+\(ID:\s*([^\)]+)\)`)
	foodMenuItemLinePattern       = regexp.MustCompile(`^\s*\d+\.\s+(.+?)\s+—\s+₹\s*([0-9]+(?:\.[0-9]+)?)\s+\|\s*(.*?)\s*\(ID:\s*([^\)]+)\)`)
	foodAddonLinePattern          = regexp.MustCompile(`^\s*Addons \(([^\)]+)\):\s+\[(.+)\]\s*$`)
	foodAddonChoicePattern        = regexp.MustCompile(`([^\[,]+?)\s+₹\s*([0-9]+(?:\.[0-9]+)?)\s+\(group:([^,\)]+),\s*choice:([^\)]+)\)`)
	foodCartItemLinePattern       = regexp.MustCompile(`^\s*-\s+(.+?)\s+—\s+₹\s*([0-9]+(?:\.[0-9]+)?)\s+\(ID:\s*([^\)]+)\)`)
	foodCartQuantityPrefixPattern = regexp.MustCompile(`(?i)^\s*(?:(\d+)\s*[x×]\s+|x\s*(\d+)\s+)`)
	foodCartQuantitySuffixPattern = regexp.MustCompile(`(?i)\s+(?:[x×]\s*(\d+)|\((\d+)\s*(?:qty|quantity)\))$`)
	foodOrderLinePattern          = regexp.MustCompile(`^\s*\d+\.\s+Order\s+(.+?)\s+—\s+(.+?)\s+\|\s+(.+?)\s+\|\s+₹\s*([0-9]+(?:\.[0-9]+)?)`)
)

func mapFoodTextResponse(toolName, text string, target any) error {
	switch out := target.(type) {
	case *mcpRestaurantSearchData:
		*out = parseFoodRestaurantSearchText(text)
		return nil
	case *mcpMenuSearchData:
		*out = parseFoodMenuSearchText(text)
		return nil
	case *mcpFoodCartData:
		*out = parseFoodCartText(text)
		return nil
	case *mcpFoodOrdersData:
		*out = parseFoodOrdersText(text)
		return nil
	default:
		return fmt.Errorf("food mcp %s failed: text response cannot be mapped", toolName)
	}
}

func mergeFoodTextResponse(toolName, text string, target any) {
	if strings.TrimSpace(text) == "" {
		return
	}
	switch out := target.(type) {
	case *mcpFoodCartData:
		parsed := parseFoodCartText(text)
		if len(out.AvailablePaymentMethods) == 0 {
			out.AvailablePaymentMethods = parsed.AvailablePaymentMethods
		}
		if out.BillBreakdown.ToPay.Value.Int() == 0 {
			out.BillBreakdown.ToPay = parsed.BillBreakdown.ToPay
		}
		if out.CartTotalAmount.Int() == 0 {
			out.CartTotalAmount = parsed.CartTotalAmount
		}
	}
}

func parseFoodRestaurantSearchText(text string) mcpRestaurantSearchData {
	var data mcpRestaurantSearchData
	for _, line := range strings.Split(text, "\n") {
		matches := foodRestaurantLinePattern.FindStringSubmatch(line)
		if len(matches) != 7 {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(matches[1], "(Ad)"))
		data.Restaurants = append(data.Restaurants, mcpRestaurant{
			ID:           flexibleString(matches[6]),
			Name:         name,
			Cuisines:     flexibleString(matches[2]),
			Rating:       flexibleString(matches[3]),
			ETA:          flexibleString(matches[4]),
			PriceForTwo:  flexibleString(matches[5]),
			Availability: "OPEN",
			IsAd:         flexibleBool(strings.Contains(matches[1], "(Ad)")),
		})
	}
	return data
}

func parseFoodMenuSearchText(text string) mcpMenuSearchData {
	var data mcpMenuSearchData
	for _, line := range strings.Split(text, "\n") {
		if matches := foodMenuItemLinePattern.FindStringSubmatch(line); len(matches) == 5 {
			meta := strings.TrimSpace(matches[3])
			data.Items = append(data.Items, mcpMenuItemDetail{
				ID:     flexibleString(matches[4]),
				Name:   strings.TrimSpace(matches[1]),
				Price:  flexibleInt(parseFoodNumber(matches[2])),
				IsVeg:  flexibleBool(strings.Contains(strings.ToLower(meta), "veg")),
				Rating: firstPipeSegment(meta),
			})
			continue
		}
		if len(data.Items) == 0 {
			continue
		}
		matches := foodAddonLinePattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		group := mcpAddonGroup{Name: strings.TrimSpace(matches[1])}
		for _, choice := range foodAddonChoicePattern.FindAllStringSubmatch(matches[2], -1) {
			if len(choice) != 5 {
				continue
			}
			if group.GroupID == "" {
				group.GroupID = strings.TrimSpace(choice[3])
			}
			group.Addons = append(group.Addons, mcpAddon{
				GroupID:  strings.TrimSpace(choice[3]),
				ChoiceID: flexibleString(choice[4]),
				Label:    strings.TrimSpace(choice[1]),
				Price:    flexibleInt(parseFoodNumber(choice[2])),
			})
		}
		if len(group.Addons) > 0 {
			last := len(data.Items) - 1
			data.Items[last].Addons = append(data.Items[last].Addons, group)
		}
	}
	return data
}

func parseFoodCartText(text string) mcpFoodCartData {
	var data mcpFoodCartData
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Restaurant:"):
			data.RestaurantName = strings.TrimSpace(strings.TrimPrefix(line, "Restaurant:"))
		case strings.HasPrefix(line, "TO PAY:"):
			value := strings.TrimSpace(strings.TrimPrefix(line, "TO PAY:"))
			data.CartTotalAmount = flexibleAmount{raw: value, rupees: parseRupees(value)}
			data.BillBreakdown.ToPay = mcpFoodBillLine{Label: "TO PAY", Value: data.CartTotalAmount}
		case strings.HasPrefix(line, "Payment methods:"):
			methods := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Payment methods:")), ",")
			for _, method := range methods {
				if method = strings.TrimSpace(method); method != "" {
					data.AvailablePaymentMethods = append(data.AvailablePaymentMethods, method)
				}
			}
		default:
			matches := foodCartItemLinePattern.FindStringSubmatch(line)
			if len(matches) == 4 {
				name, quantity := parseFoodCartItemNameQuantity(matches[1])
				data.Items = append(data.Items, mcpFoodCartItem{
					MenuItemID: flexibleString(matches[3]),
					Name:       name,
					Quantity:   flexibleInt(quantity),
					Price:      flexibleInt(parseFoodNumber(matches[2])),
					FinalPrice: flexibleInt(parseFoodNumber(matches[2])),
				})
			}
		}
	}
	return data
}

func parseFoodCartItemNameQuantity(rawName string) (string, int) {
	name := strings.TrimSpace(rawName)
	quantity := 1

	if matches := foodCartQuantityPrefixPattern.FindStringSubmatchIndex(name); len(matches) > 0 && matches[0] == 0 {
		if parsed := parseFirstMatchedInt(name, matches[2:6]); parsed > 0 {
			quantity = parsed
			name = strings.TrimSpace(name[matches[1]:])
		}
	}
	if matches := foodCartQuantitySuffixPattern.FindStringSubmatchIndex(name); len(matches) > 0 && matches[1] == len(name) {
		if parsed := parseFirstMatchedInt(name, matches[2:6]); parsed > 0 {
			quantity = parsed
			name = strings.TrimSpace(name[:matches[0]])
		}
	}
	return name, quantity
}

func parseFirstMatchedInt(value string, indexes []int) int {
	for i := 0; i+1 < len(indexes); i += 2 {
		start, end := indexes[i], indexes[i+1]
		if start < 0 || end < 0 {
			continue
		}
		parsed, err := strconv.Atoi(value[start:end])
		if err == nil {
			return parsed
		}
	}
	return 0
}

func parseFoodOrdersText(text string) mcpFoodOrdersData {
	var data mcpFoodOrdersData
	for _, line := range strings.Split(text, "\n") {
		matches := foodOrderLinePattern.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}
		status := strings.TrimSpace(matches[3])
		data.Orders = append(data.Orders, mcpFoodOrderSummary{
			OrderID:        flexibleString(matches[1]),
			RestaurantName: strings.TrimSpace(matches[2]),
			Status:         status,
			TotalAmount:    flexibleInt(parseFoodNumber(matches[4])),
			IsActive:       flexibleBool(isActiveFoodStatus(status)),
		})
	}
	return data
}

func parseFoodNumber(value string) int {
	amount, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int(amount)
}

func firstPipeSegment(value string) string {
	segment, _, _ := strings.Cut(value, "|")
	return strings.TrimSpace(segment)
}

func isActiveFoodStatus(status string) bool {
	status = strings.ToLower(status)
	return !(strings.Contains(status, "delivered") || strings.Contains(status, "cancelled") || strings.Contains(status, "completed"))
}
