package swiggy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = flexibleString(text)
		return nil
	}
	var texts []string
	if err := json.Unmarshal(data, &texts); err == nil {
		*s = flexibleString(strings.Join(texts, ", "))
		return nil
	}
	var values []any
	if err := json.Unmarshal(data, &values); err == nil {
		parts := make([]string, 0, len(values))
		for _, value := range values {
			part := strings.TrimSpace(fmt.Sprint(value))
			if part != "" {
				parts = append(parts, part)
			}
		}
		*s = flexibleString(strings.Join(parts, ", "))
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*s = flexibleString(number.String())
		return nil
	}
	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		*s = flexibleString(strconv.FormatBool(boolean))
		return nil
	}
	*s = flexibleString(strings.TrimSpace(string(data)))
	return nil
}

func (s flexibleString) String() string { return strings.TrimSpace(string(s)) }

type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*b = false
		return nil
	}
	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		*b = flexibleBool(boolean)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "true", "yes", "1", "available", "in_stock":
			*b = true
		default:
			*b = false
		}
		return nil
	}
	var number int
	if err := json.Unmarshal(data, &number); err == nil {
		*b = flexibleBool(number != 0)
		return nil
	}
	*b = false
	return nil
}

func (b flexibleBool) Bool() bool { return bool(b) }

type flexibleInt int

func (i *flexibleInt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*i = 0
		return nil
	}
	var integer int
	if err := json.Unmarshal(data, &integer); err == nil {
		*i = flexibleInt(integer)
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*i = flexibleInt(int(number))
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*i = flexibleInt(parseRupees(text))
		return nil
	}
	*i = 0
	return nil
}

func (i flexibleInt) Int() int { return int(i) }

func (i flexibleInt) String() string { return strconv.Itoa(int(i)) }

type flexibleAmount struct {
	raw    string
	rupees int
}

func (a *flexibleAmount) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*a = flexibleAmount{}
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*a = flexibleAmount{raw: strings.TrimSpace(text), rupees: parseRupees(text)}
		return nil
	}
	var integer int
	if err := json.Unmarshal(data, &integer); err == nil {
		*a = flexibleAmount{raw: strconv.Itoa(integer), rupees: integer}
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		rupees := int(number)
		*a = flexibleAmount{raw: strconv.FormatFloat(number, 'f', -1, 64), rupees: rupees}
		return nil
	}
	*a = flexibleAmount{}
	return nil
}

func (a flexibleAmount) Int() int { return a.rupees }

func (a flexibleAmount) String() string { return strings.TrimSpace(a.raw) }

type flexibleFloat float64

func (f *flexibleFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = 0
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*f = flexibleFloat(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(text), 64)
		*f = flexibleFloat(parsed)
		return nil
	}
	*f = 0
	return nil
}

func (f flexibleFloat) Float64() float64 { return float64(f) }

func parseRupees(value string) int {
	var filtered strings.Builder
	for _, r := range value {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			filtered.WriteRune(r)
		}
	}
	text := filtered.String()
	if text == "" || text == "." || text == "-" {
		return 0
	}
	amount, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return int(amount)
}

type mcpAddressesData struct {
	Addresses []mcpAddress `json:"addresses"`
}

type mcpAddress struct {
	ID              flexibleString `json:"id"`
	AddressLine     string         `json:"addressLine"`
	PhoneNumber     string         `json:"phoneNumber"`
	AddressCategory string         `json:"addressCategory"`
	AddressTag      string         `json:"addressTag"`
}

func mapAddresses(data mcpAddressesData) []domaininstamart.Address {
	addresses := make([]domaininstamart.Address, 0, len(data.Addresses))
	for _, address := range data.Addresses {
		label := strings.TrimSpace(address.AddressTag)
		if label == "" {
			label = strings.TrimSpace(address.AddressCategory)
		}
		addresses = append(addresses, domaininstamart.Address{
			ID:          address.ID.String(),
			Label:       label,
			DisplayLine: strings.TrimSpace(address.AddressLine),
			PhoneMasked: maskPhone(address.PhoneNumber),
			Category:    strings.TrimSpace(address.AddressCategory),
		})
	}
	return addresses
}

type mcpProductSearchData struct {
	Products   []mcpProduct   `json:"products"`
	NextOffset flexibleString `json:"nextOffset"`
}

type mcpProduct struct {
	ProductID       flexibleString        `json:"productId"`
	ParentProductID flexibleString        `json:"parentProductId"`
	DisplayName     string                `json:"displayName"`
	Brand           string                `json:"brand"`
	InStock         flexibleBool          `json:"inStock"`
	IsAvail         flexibleBool          `json:"isAvail"`
	IsPromoted      flexibleBool          `json:"isPromoted"`
	Variations      []mcpProductVariation `json:"variations"`
}

type mcpProductVariation struct {
	SpinID              flexibleString `json:"spinId"`
	DisplayName         string         `json:"displayName"`
	BrandName           string         `json:"brandName"`
	QuantityDescription string         `json:"quantityDescription"`
	Price               mcpPrice       `json:"price"`
	InStockAndAvailable flexibleBool   `json:"isInStockAndAvailable"`
	ImageURL            string         `json:"imageUrl"`
}

type mcpPrice struct {
	MRP        flexibleInt `json:"mrp"`
	OfferPrice flexibleInt `json:"offerPrice"`
}

func mapProductSearch(data mcpProductSearchData) domaininstamart.ProductSearchResult {
	products := make([]domaininstamart.Product, 0, len(data.Products))
	for _, product := range data.Products {
		variations := make([]domaininstamart.ProductVariation, 0, len(product.Variations))
		for _, variation := range product.Variations {
			brand := strings.TrimSpace(variation.BrandName)
			if brand == "" {
				brand = strings.TrimSpace(product.Brand)
			}
			variations = append(variations, domaininstamart.ProductVariation{
				SpinID:              variation.SpinID.String(),
				DisplayName:         strings.TrimSpace(variation.DisplayName),
				Brand:               brand,
				QuantityDescription: strings.TrimSpace(variation.QuantityDescription),
				Price: domaininstamart.Price{
					MRP:        variation.Price.MRP.Int(),
					OfferPrice: variation.Price.OfferPrice.Int(),
				},
				InStock:  variation.InStockAndAvailable.Bool(),
				ImageURL: strings.TrimSpace(variation.ImageURL),
			})
		}
		products = append(products, domaininstamart.Product{
			ID:              product.ProductID.String(),
			ParentProductID: product.ParentProductID.String(),
			DisplayName:     strings.TrimSpace(product.DisplayName),
			Brand:           strings.TrimSpace(product.Brand),
			InStock:         product.InStock.Bool(),
			Available:       product.IsAvail.Bool(),
			Promoted:        product.IsPromoted.Bool(),
			Variations:      variations,
		})
	}
	return domaininstamart.ProductSearchResult{Products: products, NextOffset: data.NextOffset.String()}
}

type mcpCartData struct {
	SelectedAddress         flexibleString            `json:"selectedAddress"`
	SelectedAddressDetails  mcpSelectedAddressDetails `json:"selectedAddressDetails"`
	CartTotalAmount         flexibleAmount            `json:"cartTotalAmount"`
	Items                   []mcpCartItem             `json:"items"`
	BillBreakdown           mcpBillBreakdown          `json:"billBreakdown"`
	AvailablePaymentMethods []string                  `json:"availablePaymentMethods"`
	StoreIDs                []flexibleString          `json:"storeIds"`
}

type mcpSelectedAddressDetails struct {
	Annotation string        `json:"annotation"`
	Area       string        `json:"area"`
	City       string        `json:"city"`
	Lat        flexibleFloat `json:"lat"`
	Lng        flexibleFloat `json:"lng"`
}

type mcpCartItem struct {
	SpinID               flexibleString `json:"spinId"`
	ItemName             string         `json:"itemName"`
	Name                 string         `json:"name"`
	Quantity             flexibleInt    `json:"quantity"`
	StoreID              flexibleString `json:"storeId"`
	InStockAndAvailable  flexibleBool   `json:"isInStockAndAvailable"`
	MRP                  flexibleInt    `json:"mrp"`
	DiscountedFinalPrice flexibleInt    `json:"discountedFinalPrice"`
}

type mcpBillBreakdown struct {
	LineItems []mcpBillLine `json:"lineItems"`
	ToPay     mcpBillLine   `json:"toPay"`
}

type mcpBillLine struct {
	Label string         `json:"label"`
	Value flexibleAmount `json:"value"`
}

func mapCart(data mcpCartData) domaininstamart.Cart {
	items := make([]domaininstamart.CartItem, 0, len(data.Items))
	storeIDs := make([]string, 0, len(data.Items)+len(data.StoreIDs))
	seenStores := map[string]struct{}{}
	for _, item := range data.Items {
		name := strings.TrimSpace(item.ItemName)
		if name == "" {
			name = strings.TrimSpace(item.Name)
		}
		storeID := item.StoreID.String()
		if storeID != "" {
			if _, ok := seenStores[storeID]; !ok {
				seenStores[storeID] = struct{}{}
				storeIDs = append(storeIDs, storeID)
			}
		}
		items = append(items, domaininstamart.CartItem{
			SpinID:     item.SpinID.String(),
			Name:       name,
			Quantity:   item.Quantity.Int(),
			StoreID:    storeID,
			InStock:    item.InStockAndAvailable.Bool(),
			MRP:        item.MRP.Int(),
			FinalPrice: item.DiscountedFinalPrice.Int(),
		})
	}
	for _, storeIDValue := range data.StoreIDs {
		storeID := storeIDValue.String()
		if storeID == "" {
			continue
		}
		if _, ok := seenStores[storeID]; !ok {
			seenStores[storeID] = struct{}{}
			storeIDs = append(storeIDs, storeID)
		}
	}

	billLines := make([]domaininstamart.BillLine, 0, len(data.BillBreakdown.LineItems))
	for _, line := range data.BillBreakdown.LineItems {
		billLines = append(billLines, domaininstamart.BillLine{Label: strings.TrimSpace(line.Label), Value: line.Value.String()})
	}

	addressLabel := strings.TrimSpace(data.SelectedAddressDetails.Annotation)
	addressDisplayLine := compactJoin([]string{data.SelectedAddressDetails.Area, data.SelectedAddressDetails.City}, ", ")
	var location *domaininstamart.Location
	lat := data.SelectedAddressDetails.Lat.Float64()
	lng := data.SelectedAddressDetails.Lng.Float64()
	if lat != 0 || lng != 0 {
		location = &domaininstamart.Location{Lat: lat, Lng: lng}
	}

	toPay := data.BillBreakdown.ToPay.Value.Int()
	total := data.CartTotalAmount.Int()
	if total == 0 {
		total = toPay
	}
	return domaininstamart.Cart{
		AddressID:          data.SelectedAddress.String(),
		AddressLabel:       addressLabel,
		AddressDisplayLine: addressDisplayLine,
		AddressLocation:    location,
		Items:              items,
		Bill: domaininstamart.BillBreakdown{
			Lines:       billLines,
			ToPayLabel:  strings.TrimSpace(data.BillBreakdown.ToPay.Label),
			ToPayValue:  data.BillBreakdown.ToPay.Value.String(),
			ToPayRupees: toPay,
		},
		TotalRupees:             total,
		AvailablePaymentMethods: append([]string(nil), data.AvailablePaymentMethods...),
		StoreIDs:                storeIDs,
	}
}

type mcpCheckoutData struct {
	OrderID       flexibleString     `json:"orderId"`
	OrderIDs      []flexibleString   `json:"orderIds"`
	Orders        []mcpCheckoutOrder `json:"orders"`
	Results       []mcpCheckoutOrder `json:"results"`
	OrderResults  []mcpCheckoutOrder `json:"orderResults"`
	Status        string             `json:"status"`
	PaymentMethod string             `json:"paymentMethod"`
	CartTotal     flexibleInt        `json:"cartTotal"`
}

type mcpCheckoutOrder struct {
	OrderID flexibleString `json:"orderId"`
	Status  string         `json:"status"`
}

func mapCheckout(data mcpCheckoutData, message string) domaininstamart.CheckoutResult {
	orderIDs := make([]string, 0, 1+len(data.OrderIDs)+len(data.Orders)+len(data.Results)+len(data.OrderResults))
	appendOrderID := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" {
			orderIDs = append(orderIDs, id)
		}
	}
	appendOrderID(data.OrderID.String())
	for _, id := range data.OrderIDs {
		appendOrderID(id.String())
	}
	status := strings.TrimSpace(data.Status)
	for _, order := range append(append(data.Orders, data.Results...), data.OrderResults...) {
		appendOrderID(order.OrderID.String())
		if status == "" {
			status = strings.TrimSpace(order.Status)
		}
	}
	return domaininstamart.CheckoutResult{
		Message:       strings.TrimSpace(message),
		OrderIDs:      orderIDs,
		Status:        status,
		PaymentMethod: strings.TrimSpace(data.PaymentMethod),
		CartTotal:     data.CartTotal.Int(),
		MultiStore:    len(orderIDs) > 1,
	}
}

type mcpOrdersData struct {
	Orders  []mcpOrderSummary `json:"orders"`
	HasMore bool              `json:"hasMore"`
}

type mcpOrderSummary struct {
	OrderID       flexibleString `json:"orderId"`
	Status        string         `json:"status"`
	ItemCount     flexibleInt    `json:"itemCount"`
	TotalAmount   flexibleInt    `json:"totalAmount"`
	PaymentMethod string         `json:"paymentMethod"`
	IsActive      flexibleBool   `json:"isActive"`
	Lat           flexibleFloat  `json:"lat"`
	Lng           flexibleFloat  `json:"lng"`
}

func mapOrders(data mcpOrdersData) domaininstamart.OrderHistory {
	orders := make([]domaininstamart.OrderSummary, 0, len(data.Orders))
	for _, order := range data.Orders {
		var location *domaininstamart.Location
		lat := order.Lat.Float64()
		lng := order.Lng.Float64()
		if lat != 0 || lng != 0 {
			location = &domaininstamart.Location{Lat: lat, Lng: lng}
		}
		orders = append(orders, domaininstamart.OrderSummary{
			OrderID:       order.OrderID.String(),
			Status:        strings.TrimSpace(order.Status),
			ItemCount:     order.ItemCount.Int(),
			TotalRupees:   order.TotalAmount.Int(),
			PaymentMethod: strings.TrimSpace(order.PaymentMethod),
			Active:        order.IsActive.Bool(),
			Location:      location,
		})
	}
	return domaininstamart.OrderHistory{Orders: orders, HasMore: data.HasMore}
}

type mcpTrackingData struct {
	OrderID                flexibleString    `json:"orderId"`
	Status                 mcpTrackingStatus `json:"status"`
	Items                  []mcpTrackingItem `json:"items"`
	PollingIntervalSeconds flexibleInt       `json:"pollingIntervalSeconds"`
}

type mcpTrackingStatus struct {
	StatusMessage    string      `json:"statusMessage"`
	SubStatusMessage string      `json:"subStatusMessage"`
	ETAMinutes       flexibleInt `json:"etaMinutes"`
	ETAText          string      `json:"etaText"`
}

type mcpTrackingItem struct {
	Name     string      `json:"name"`
	Quantity flexibleInt `json:"quantity"`
	Price    flexibleInt `json:"price"`
}

func mapTracking(data mcpTrackingData) domaininstamart.TrackingStatus {
	items := make([]domaininstamart.CartItem, 0, len(data.Items))
	for _, item := range data.Items {
		items = append(items, domaininstamart.CartItem{
			Name:       strings.TrimSpace(item.Name),
			Quantity:   item.Quantity.Int(),
			FinalPrice: item.Price.Int(),
		})
	}
	return domaininstamart.TrackingStatus{
		OrderID:                data.OrderID.String(),
		StatusMessage:          strings.TrimSpace(data.Status.StatusMessage),
		SubStatusMessage:       strings.TrimSpace(data.Status.SubStatusMessage),
		ETAText:                strings.TrimSpace(data.Status.ETAText),
		ETAMinutes:             data.Status.ETAMinutes.Int(),
		Items:                  items,
		PollingIntervalSeconds: data.PollingIntervalSeconds.Int(),
	}
}

func compactJoin(parts []string, sep string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return strings.Join(trimmed, sep)
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if strings.Contains(phone, "*") {
		return phone
	}
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	digitText := digits.String()
	if digitText == "" {
		return ""
	}
	if len(digitText) <= 4 {
		return "****" + digitText
	}
	return "****" + digitText[len(digitText)-4:]
}
