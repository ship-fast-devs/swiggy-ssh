package swiggy

import (
	"strings"

	domainfood "swiggy-ssh/internal/domain/food"
)

func mapFoodAddresses(data mcpAddressesData) []domainfood.Address {
	addresses := make([]domainfood.Address, 0, len(data.Addresses))
	for _, address := range data.Addresses {
		label := strings.TrimSpace(address.AddressTag)
		if label == "" {
			label = strings.TrimSpace(address.AddressCategory)
		}
		addresses = append(addresses, domainfood.Address{
			ID:          address.ID.String(),
			Label:       label,
			DisplayLine: strings.TrimSpace(address.AddressLine),
			PhoneMasked: maskPhone(address.PhoneNumber),
			Category:    strings.TrimSpace(address.AddressCategory),
		})
	}
	return addresses
}

// --- search_restaurants ---

type mcpRestaurantSearchData struct {
	Restaurants []mcpRestaurant `json:"restaurants"`
	NextOffset  flexibleInt     `json:"nextOffset"`
}

type mcpRestaurant struct {
	ID                 flexibleString `json:"id"`
	Name               string         `json:"name"`
	Cuisines           flexibleString `json:"cuisines"`
	Rating             flexibleString `json:"rating"`
	ETA                flexibleString `json:"eta"`
	PriceForTwo        flexibleString `json:"priceForTwo"`
	Availability       flexibleString `json:"availability"`
	AvailabilityStatus flexibleString `json:"availabilityStatus"`
	IsAd               flexibleBool   `json:"isAd"`
}

func mapRestaurantSearch(data mcpRestaurantSearchData) domainfood.RestaurantSearchResult {
	restaurants := make([]domainfood.Restaurant, 0, len(data.Restaurants))
	for _, r := range data.Restaurants {
		restaurants = append(restaurants, domainfood.Restaurant{
			ID:           r.ID.String(),
			Name:         strings.TrimSpace(r.Name),
			Cuisines:     r.Cuisines.String(),
			Rating:       r.Rating.String(),
			ETA:          r.ETA.String(),
			PriceForTwo:  r.PriceForTwo.String(),
			Availability: foodRestaurantAvailability(r),
			IsAd:         r.IsAd.Bool(),
		})
	}
	return domainfood.RestaurantSearchResult{
		Restaurants: restaurants,
		NextOffset:  data.NextOffset.Int(),
	}
}

func foodRestaurantAvailability(r mcpRestaurant) string {
	availability := r.AvailabilityStatus.String()
	if availability == "" {
		availability = r.Availability.String()
	}
	if availability == "" {
		availability = "OPEN"
	}
	return availability
}

// --- get_restaurant_menu ---

type mcpMenuPage struct {
	RestaurantID    flexibleString    `json:"restaurantId"`
	RestaurantName  string            `json:"restaurantName"`
	Categories      []mcpMenuCategory `json:"categories"`
	Page            flexibleInt       `json:"page"`
	TotalCategories flexibleInt       `json:"totalCategories"`
}

type mcpMenuCategory struct {
	Name     string        `json:"name"`
	Title    string        `json:"title"`
	Category string        `json:"category"`
	Items    []mcpMenuItem `json:"items"`
}

type mcpMenuItem struct {
	ID          flexibleString `json:"id"`
	MenuItemID  flexibleString `json:"menu_item_id"`
	ItemID      flexibleString `json:"itemId"`
	Name        string         `json:"name"`
	ItemName    string         `json:"itemName"`
	Price       flexibleInt    `json:"price"`
	IsVeg       flexibleBool   `json:"isVeg"`
	Rating      string         `json:"rating"`
	Description string         `json:"description"`
	HasVariants flexibleBool   `json:"hasVariants"`
	HasAddons   flexibleBool   `json:"hasAddons"`
}

func mapMenuPage(data mcpMenuPage) domainfood.MenuPage {
	categories := make([]domainfood.MenuCategory, 0, len(data.Categories))
	for _, cat := range data.Categories {
		items := make([]domainfood.MenuItem, 0, len(cat.Items))
		for _, item := range cat.Items {
			items = append(items, domainfood.MenuItem{
				ID:          foodMenuItemID(item.ID, item.MenuItemID, item.ItemID),
				Name:        firstNonEmpty(item.Name, item.ItemName),
				Price:       item.Price.Int(),
				IsVeg:       item.IsVeg.Bool(),
				Rating:      strings.TrimSpace(item.Rating),
				Description: strings.TrimSpace(item.Description),
				HasVariants: item.HasVariants.Bool(),
				HasAddons:   item.HasAddons.Bool(),
			})
		}
		categories = append(categories, domainfood.MenuCategory{
			Name:  firstNonEmpty(cat.Name, cat.Title, cat.Category),
			Items: items,
		})
	}
	return domainfood.MenuPage{
		RestaurantID:    data.RestaurantID.String(),
		RestaurantName:  strings.TrimSpace(data.RestaurantName),
		Categories:      categories,
		Page:            data.Page.Int(),
		TotalCategories: data.TotalCategories.Int(),
	}
}

// --- search_menu ---

type mcpMenuSearchData struct {
	Items      []mcpMenuItemDetail `json:"items"`
	NextOffset flexibleInt         `json:"nextOffset"`
}

type mcpMenuItemDetail struct {
	ID          flexibleString    `json:"id"`
	MenuItemID  flexibleString    `json:"menu_item_id"`
	ItemID      flexibleString    `json:"itemId"`
	Name        string            `json:"name"`
	ItemName    string            `json:"itemName"`
	Price       flexibleInt       `json:"price"`
	IsVeg       flexibleBool      `json:"isVeg"`
	Rating      string            `json:"rating"`
	Description string            `json:"description"`
	Variants    []mcpVariantGroup `json:"variants"`
	VariantsV2  []mcpVariantGroup `json:"variantsV2"`
	Addons      []mcpAddonGroup   `json:"addons"`
}

type mcpVariantGroup struct {
	GroupID  flexibleString `json:"groupId"`
	Name     string         `json:"name"`
	Variants []mcpVariant   `json:"variants"`
}

type mcpVariant struct {
	GroupID     flexibleString `json:"groupId"`
	VariationID flexibleString `json:"variationId"`
	Label       string         `json:"label"`
}

type mcpAddonGroup struct {
	GroupID string     `json:"groupId"`
	Name    string     `json:"name"`
	Addons  []mcpAddon `json:"addons"`
}

type mcpAddon struct {
	GroupID  string         `json:"groupId"`
	ChoiceID flexibleString `json:"choiceId"`
	Label    string         `json:"label"`
	Price    flexibleInt    `json:"price"`
}

func mapMenuSearch(data mcpMenuSearchData) domainfood.MenuSearchResult {
	items := make([]domainfood.MenuItemDetail, 0, len(data.Items))
	for _, item := range data.Items {
		variants := mapVariantGroups(item.Variants)
		variantsV2 := mapVariantGroups(item.VariantsV2)
		addons := mapAddonGroups(item.Addons)
		items = append(items, domainfood.MenuItemDetail{
			ID:          foodMenuItemID(item.ID, item.MenuItemID, item.ItemID),
			Name:        firstNonEmpty(item.Name, item.ItemName),
			Price:       item.Price.Int(),
			IsVeg:       item.IsVeg.Bool(),
			Rating:      strings.TrimSpace(item.Rating),
			Description: strings.TrimSpace(item.Description),
			Variants:    variants,
			VariantsV2:  variantsV2,
			Addons:      addons,
		})
	}
	return domainfood.MenuSearchResult{
		Items:      items,
		NextOffset: data.NextOffset.Int(),
	}
}

func foodMenuItemID(values ...flexibleString) string {
	for _, value := range values {
		if id := value.String(); id != "" {
			return id
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func mapVariantGroups(groups []mcpVariantGroup) []domainfood.MenuVariantGroup {
	result := make([]domainfood.MenuVariantGroup, 0, len(groups))
	for _, g := range groups {
		variants := make([]domainfood.MenuVariant, 0, len(g.Variants))
		for _, v := range g.Variants {
			variants = append(variants, domainfood.MenuVariant{
				GroupID:     v.GroupID.String(),
				VariationID: v.VariationID.String(),
				Label:       strings.TrimSpace(v.Label),
			})
		}
		result = append(result, domainfood.MenuVariantGroup{
			GroupID:  g.GroupID.String(),
			Name:     strings.TrimSpace(g.Name),
			Variants: variants,
		})
	}
	return result
}

func mapAddonGroups(groups []mcpAddonGroup) []domainfood.MenuAddonGroup {
	result := make([]domainfood.MenuAddonGroup, 0, len(groups))
	for _, g := range groups {
		addons := make([]domainfood.MenuAddon, 0, len(g.Addons))
		for _, a := range g.Addons {
			addons = append(addons, domainfood.MenuAddon{
				GroupID:  strings.TrimSpace(a.GroupID),
				ChoiceID: a.ChoiceID.String(),
				Label:    strings.TrimSpace(a.Label),
				Price:    a.Price.Int(),
			})
		}
		result = append(result, domainfood.MenuAddonGroup{
			GroupID: strings.TrimSpace(g.GroupID),
			Name:    strings.TrimSpace(g.Name),
			Addons:  addons,
		})
	}
	return result
}

// --- update_food_cart / get_food_cart ---

type mcpFoodCartData struct {
	RestaurantID            flexibleString        `json:"restaurantId"`
	RestaurantName          string                `json:"restaurantName"`
	Restaurant              mcpFoodCartRestaurant `json:"restaurant"`
	AddressID               flexibleString        `json:"addressId"`
	AddressLabel            string                `json:"addressLabel"`
	Items                   []mcpFoodCartItem     `json:"items"`
	BillBreakdown           mcpFoodBill           `json:"billBreakdown"`
	CartTotalAmount         flexibleAmount        `json:"cartTotalAmount"`
	Pricing                 mcpFoodCartPricing    `json:"pricing"`
	AvailablePaymentMethods []string              `json:"availablePaymentMethods"`
}

type mcpFoodCartRestaurant struct {
	Name string `json:"name"`
}

type mcpFoodCartItem struct {
	MenuItemID      flexibleString `json:"menu_item_id"`
	Name            string         `json:"name"`
	Quantity        flexibleInt    `json:"quantity"`
	Price           flexibleInt    `json:"price"`
	Subtotal        flexibleInt    `json:"subtotal"`
	Total           flexibleInt    `json:"total"`
	FinalPrice      flexibleInt    `json:"finalPrice"`
	FinalPriceSnake flexibleInt    `json:"final_price"`
}

type mcpFoodCartPricing struct {
	ItemTotal       flexibleAmount `json:"item_total"`
	DeliveryCharge  flexibleAmount `json:"delivery_charge"`
	TaxesAndCharges flexibleAmount `json:"taxes_and_charges"`
	ToPay           flexibleAmount `json:"to_pay"`
}

type mcpFoodBill struct {
	LineItems []mcpFoodBillLine `json:"lineItems"`
	ToPay     mcpFoodBillLine   `json:"toPay"`
}

type mcpFoodBillLine struct {
	Label string         `json:"label"`
	Value flexibleAmount `json:"value"`
}

func mapFoodCart(data mcpFoodCartData) domainfood.FoodCart {
	items := make([]domainfood.FoodCartItem, 0, len(data.Items))
	for _, item := range data.Items {
		price := item.Price.Int()
		if price == 0 {
			price = item.Subtotal.Int()
		}
		if price == 0 {
			price = item.Total.Int()
		}
		finalPrice := item.FinalPrice.Int()
		if finalPrice == 0 {
			finalPrice = item.FinalPriceSnake.Int()
		}
		if finalPrice == 0 {
			finalPrice = item.Total.Int()
		}
		items = append(items, domainfood.FoodCartItem{
			MenuItemID: item.MenuItemID.String(),
			Name:       strings.TrimSpace(item.Name),
			Quantity:   item.Quantity.Int(),
			Price:      price,
			FinalPrice: finalPrice,
		})
	}

	billLines := make([]domainfood.BillLine, 0, len(data.BillBreakdown.LineItems))
	for _, line := range data.BillBreakdown.LineItems {
		billLines = append(billLines, domainfood.BillLine{
			Label: strings.TrimSpace(line.Label),
			Value: line.Value.String(),
		})
	}

	toPay := data.BillBreakdown.ToPay.Value.Int()
	if toPay == 0 {
		toPay = data.Pricing.ToPay.Int()
	}
	total := data.CartTotalAmount.Int()
	if total == 0 {
		total = toPay
	}
	restaurantName := strings.TrimSpace(data.RestaurantName)
	if restaurantName == "" {
		restaurantName = strings.TrimSpace(data.Restaurant.Name)
	}

	return domainfood.FoodCart{
		RestaurantID:   data.RestaurantID.String(),
		RestaurantName: restaurantName,
		AddressID:      data.AddressID.String(),
		AddressLabel:   strings.TrimSpace(data.AddressLabel),
		Items:          items,
		Bill: domainfood.BillBreakdown{
			Lines:       billLines,
			ToPayLabel:  strings.TrimSpace(data.BillBreakdown.ToPay.Label),
			ToPayValue:  data.BillBreakdown.ToPay.Value.String(),
			ToPayRupees: toPay,
		},
		TotalRupees:             total,
		AvailablePaymentMethods: append([]string(nil), data.AvailablePaymentMethods...),
	}
}

// --- place_food_order ---

type mcpFoodOrderResult struct {
	OrderID       flexibleString `json:"orderId"`
	Status        string         `json:"status"`
	PaymentMethod string         `json:"paymentMethod"`
	CartTotal     flexibleInt    `json:"cartTotal"`
}

func mapFoodOrderResult(data mcpFoodOrderResult, message string) domainfood.FoodOrderResult {
	return domainfood.FoodOrderResult{
		Message:       strings.TrimSpace(message),
		OrderID:       data.OrderID.String(),
		Status:        strings.TrimSpace(data.Status),
		PaymentMethod: strings.TrimSpace(data.PaymentMethod),
		CartTotal:     data.CartTotal.Int(),
	}
}

// --- get_food_orders ---

type mcpFoodOrdersData struct {
	Orders  []mcpFoodOrderSummary `json:"orders"`
	HasMore flexibleBool          `json:"hasMore"`
}

type mcpFoodOrderSummary struct {
	OrderID        flexibleString `json:"orderId"`
	RestaurantName string         `json:"restaurantName"`
	Status         string         `json:"status"`
	TotalAmount    flexibleInt    `json:"totalAmount"`
	IsActive       flexibleBool   `json:"isActive"`
}

func mapFoodOrders(data mcpFoodOrdersData) domainfood.FoodOrderHistory {
	orders := make([]domainfood.FoodOrderSummary, 0, len(data.Orders))
	for _, order := range data.Orders {
		orders = append(orders, domainfood.FoodOrderSummary{
			OrderID:        order.OrderID.String(),
			RestaurantName: strings.TrimSpace(order.RestaurantName),
			Status:         strings.TrimSpace(order.Status),
			TotalRupees:    order.TotalAmount.Int(),
			Active:         order.IsActive.Bool(),
		})
	}
	return domainfood.FoodOrderHistory{
		Orders:  orders,
		HasMore: data.HasMore.Bool(),
	}
}

// --- get_food_order_details ---

type mcpFoodOrderDetails struct {
	OrderID        flexibleString    `json:"orderId"`
	RestaurantName string            `json:"restaurantName"`
	Status         string            `json:"status"`
	Items          []mcpFoodCartItem `json:"items"`
	TotalAmount    flexibleInt       `json:"totalAmount"`
}

func mapFoodOrderDetails(data mcpFoodOrderDetails) domainfood.FoodOrderDetails {
	items := make([]domainfood.FoodCartItem, 0, len(data.Items))
	for _, item := range data.Items {
		items = append(items, domainfood.FoodCartItem{
			MenuItemID: item.MenuItemID.String(),
			Name:       strings.TrimSpace(item.Name),
			Quantity:   item.Quantity.Int(),
			Price:      item.Price.Int(),
			FinalPrice: item.FinalPrice.Int(),
		})
	}
	return domainfood.FoodOrderDetails{
		OrderID:        data.OrderID.String(),
		RestaurantName: strings.TrimSpace(data.RestaurantName),
		Status:         strings.TrimSpace(data.Status),
		Items:          items,
		TotalRupees:    data.TotalAmount.Int(),
	}
}

// --- track_food_order ---

type mcpFoodTrackingData struct {
	OrderID          flexibleString `json:"orderId"`
	StatusMessage    string         `json:"statusMessage"`
	SubStatusMessage string         `json:"subStatusMessage"`
	ETAText          string         `json:"etaText"`
	ETAMinutes       flexibleInt    `json:"etaMinutes"`
}

func mapFoodTracking(data mcpFoodTrackingData) domainfood.FoodTrackingStatus {
	return domainfood.FoodTrackingStatus{
		OrderID:          data.OrderID.String(),
		StatusMessage:    strings.TrimSpace(data.StatusMessage),
		SubStatusMessage: strings.TrimSpace(data.SubStatusMessage),
		ETAText:          strings.TrimSpace(data.ETAText),
		ETAMinutes:       data.ETAMinutes.Int(),
	}
}

// --- fetch_food_coupons ---

type mcpFoodCouponsData struct {
	Coupons    []mcpFoodCoupon `json:"coupons"`
	Applicable flexibleInt     `json:"applicable"`
}

type mcpFoodCoupon struct {
	Code        string       `json:"code"`
	Description string       `json:"description"`
	Discount    flexibleInt  `json:"discount"`
	Applicable  flexibleBool `json:"applicable"`
}

func mapFoodCoupons(data mcpFoodCouponsData) domainfood.FoodCouponsResult {
	coupons := make([]domainfood.FoodCoupon, 0, len(data.Coupons))
	for _, c := range data.Coupons {
		coupons = append(coupons, domainfood.FoodCoupon{
			Code:        strings.TrimSpace(c.Code),
			Description: strings.TrimSpace(c.Description),
			Discount:    c.Discount.Int(),
			Applicable:  c.Applicable.Bool(),
		})
	}
	return domainfood.FoodCouponsResult{
		Coupons:    coupons,
		Applicable: data.Applicable.Int(),
	}
}
