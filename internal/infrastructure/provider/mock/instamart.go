package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

type InstamartProvider struct {
	mu                sync.Mutex
	selectedAddressID string
	items             []domaininstamart.CartUpdateItem
	lastCheckout      domaininstamart.CheckoutResult
}

var _ domaininstamart.Provider = (*InstamartProvider)(nil)

func NewInstamartProvider() *InstamartProvider {
	return &InstamartProvider{selectedAddressID: mockAddresses[0].ID}
}

var mockAddresses = []domaininstamart.Address{
	{ID: "mock-address-home", Label: "Home", DisplayLine: "Mock Street, Test Nagar", PhoneMasked: "****0001", Category: "Home"},
	{ID: "mock-address-work", Label: "Work", DisplayLine: "Demo Tower, Sample District", PhoneMasked: "****0002", Category: "Work"},
}

var mockProducts = []domaininstamart.Product{
	{
		ID:              "mock-product-milk",
		ParentProductID: "mock-parent-milk",
		DisplayName:     "Mock Fresh Milk",
		Brand:           "Mock Dairy",
		InStock:         true,
		Available:       true,
		Variations: []domaininstamart.ProductVariation{
			{SpinID: "mock-spin-milk-500", DisplayName: "Mock Fresh Milk", Brand: "Mock Dairy", QuantityDescription: "500 ml", Price: domaininstamart.Price{MRP: 32, OfferPrice: 30}, InStock: true, ImageURL: "https://example.invalid/mock-milk-500.png"},
			{SpinID: "mock-spin-milk-1000", DisplayName: "Mock Fresh Milk", Brand: "Mock Dairy", QuantityDescription: "1 L", Price: domaininstamart.Price{MRP: 64, OfferPrice: 60}, InStock: true, ImageURL: "https://example.invalid/mock-milk-1000.png"},
		},
	},
	{
		ID:              "mock-product-bread",
		ParentProductID: "mock-parent-bread",
		DisplayName:     "Mock Sandwich Bread",
		Brand:           "Mock Bakery",
		InStock:         true,
		Available:       true,
		Promoted:        true,
		Variations: []domaininstamart.ProductVariation{
			{SpinID: "mock-spin-bread-400", DisplayName: "Mock Sandwich Bread", Brand: "Mock Bakery", QuantityDescription: "400 g", Price: domaininstamart.Price{MRP: 55, OfferPrice: 49}, InStock: true, ImageURL: "https://example.invalid/mock-bread.png"},
		},
	},
	{
		ID:              "mock-product-banana",
		ParentProductID: "mock-parent-banana",
		DisplayName:     "Mock Banana",
		Brand:           "Mock Farms",
		InStock:         true,
		Available:       true,
		Variations: []domaininstamart.ProductVariation{
			{SpinID: "mock-spin-banana-6", DisplayName: "Mock Banana", Brand: "Mock Farms", QuantityDescription: "6 pcs", Price: domaininstamart.Price{MRP: 48, OfferPrice: 45}, InStock: true, ImageURL: "https://example.invalid/mock-banana.png"},
		},
	},
}

func (p *InstamartProvider) GetAddresses(context.Context) ([]domaininstamart.Address, error) {
	return append([]domaininstamart.Address(nil), mockAddresses...), nil
}

func (p *InstamartProvider) SearchProducts(_ context.Context, _ string, query string, offset int) (domaininstamart.ProductSearchResult, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	var products []domaininstamart.Product
	for _, product := range mockProducts {
		if query == "" || strings.Contains(strings.ToLower(product.DisplayName), query) || strings.Contains(strings.ToLower(product.Brand), query) {
			products = append(products, cloneProduct(product))
		}
	}
	return paginateProducts(products, offset), nil
}

func (p *InstamartProvider) YourGoToItems(_ context.Context, _ string, offset int) (domaininstamart.ProductSearchResult, error) {
	products := make([]domaininstamart.Product, 0, len(mockProducts))
	for _, product := range mockProducts {
		products = append(products, cloneProduct(product))
	}
	return paginateProducts(products, offset), nil
}

func (p *InstamartProvider) GetCart(context.Context) (domaininstamart.Cart, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buildCartLocked(), nil
}

func (p *InstamartProvider) UpdateCart(_ context.Context, selectedAddressID string, items []domaininstamart.CartUpdateItem) (domaininstamart.Cart, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.selectedAddressID = strings.TrimSpace(selectedAddressID)
	p.items = append([]domaininstamart.CartUpdateItem(nil), items...)
	return p.buildCartLocked(), nil
}

func (p *InstamartProvider) ClearCart(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = nil
	return nil
}

func (p *InstamartProvider) Checkout(_ context.Context, addressID, paymentMethod string) (domaininstamart.CheckoutResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cart := p.buildCartLocked()
	result := domaininstamart.CheckoutResult{
		Message:       "Instamart order placed successfully! Mock order confirmed.",
		OrderIDs:      []string{"mock-order-1001"},
		Status:        "CONFIRMED",
		PaymentMethod: paymentMethod,
		CartTotal:     cart.TotalRupees,
		MultiStore:    len(cart.StoreIDs) > 1,
	}
	p.selectedAddressID = addressID
	p.lastCheckout = result
	return result, nil
}

func (p *InstamartProvider) GetOrders(_ context.Context, input domaininstamart.OrderHistoryQuery) (domaininstamart.OrderHistory, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	orders := []domaininstamart.OrderSummary{
		{OrderID: "mock-order-history-1", Status: "DELIVERED", ItemCount: 2, TotalRupees: 154, PaymentMethod: "Cash", Active: false, Location: mockLocation()},
	}
	if len(p.lastCheckout.OrderIDs) > 0 {
		orders = append([]domaininstamart.OrderSummary{{OrderID: p.lastCheckout.OrderIDs[0], Status: p.lastCheckout.Status, ItemCount: len(p.items), TotalRupees: p.lastCheckout.CartTotal, PaymentMethod: p.lastCheckout.PaymentMethod, Active: true, Location: mockLocation()}}, orders...)
	}
	if input.ActiveOnly {
		active := make([]domaininstamart.OrderSummary, 0, len(orders))
		for _, order := range orders {
			if order.Active {
				active = append(active, order)
			}
		}
		orders = active
	}
	if input.Count > 0 && input.Count < len(orders) {
		orders = orders[:input.Count]
	}
	return domaininstamart.OrderHistory{Orders: orders, HasMore: false}, nil
}

func (p *InstamartProvider) TrackOrder(_ context.Context, orderID string, _ domaininstamart.Location) (domaininstamart.TrackingStatus, error) {
	return domaininstamart.TrackingStatus{
		OrderID:                orderID,
		StatusMessage:          "Mock order is getting packed",
		SubStatusMessage:       "A mock delivery partner will be assigned soon",
		ETAText:                "7 mins",
		ETAMinutes:             7,
		Items:                  []domaininstamart.CartItem{{Name: "Mock cart items", Quantity: 1, FinalPrice: 100}},
		PollingIntervalSeconds: 30,
	}, nil
}

func (p *InstamartProvider) buildCartLocked() domaininstamart.Cart {
	address := mockAddressByID(p.selectedAddressID)
	items := make([]domaininstamart.CartItem, 0, len(p.items))
	storeIDs := make([]string, 0, len(p.items))
	seenStores := map[string]struct{}{}
	itemTotal := 0
	for idx, updateItem := range p.items {
		variation, ok := mockVariationBySpinID(updateItem.SpinID)
		if !ok {
			continue
		}
		storeID := fmt.Sprintf("mock-store-%d", idx%2+1)
		if _, ok := seenStores[storeID]; !ok {
			seenStores[storeID] = struct{}{}
			storeIDs = append(storeIDs, storeID)
		}
		lineTotal := variation.Price.OfferPrice * updateItem.Quantity
		itemTotal += lineTotal
		items = append(items, domaininstamart.CartItem{
			SpinID:     updateItem.SpinID,
			Name:       compactMockName(variation),
			Quantity:   updateItem.Quantity,
			StoreID:    storeID,
			InStock:    variation.InStock,
			MRP:        variation.Price.MRP,
			FinalPrice: lineTotal,
		})
	}

	fees := 0
	if len(items) > 0 {
		fees = 20
	}
	total := itemTotal + fees
	return domaininstamart.Cart{
		AddressID:          address.ID,
		AddressLabel:       address.Label,
		AddressDisplayLine: address.DisplayLine,
		AddressLocation:    mockLocation(),
		Items:              items,
		Bill: domaininstamart.BillBreakdown{
			Lines: []domaininstamart.BillLine{
				{Label: "Item Total", Value: fmt.Sprintf("Rs %d", itemTotal)},
				{Label: "Mock Fees", Value: fmt.Sprintf("Rs %d", fees)},
			},
			ToPayLabel:  "To Pay",
			ToPayValue:  fmt.Sprintf("Rs %d", total),
			ToPayRupees: total,
		},
		TotalRupees:             total,
		AvailablePaymentMethods: []string{"Cash"},
		StoreIDs:                storeIDs,
	}
}

func mockAddressByID(id string) domaininstamart.Address {
	for _, address := range mockAddresses {
		if address.ID == id {
			return address
		}
	}
	return mockAddresses[0]
}

func mockVariationBySpinID(spinID string) (domaininstamart.ProductVariation, bool) {
	for _, product := range mockProducts {
		for _, variation := range product.Variations {
			if variation.SpinID == spinID {
				return variation, true
			}
		}
	}
	return domaininstamart.ProductVariation{}, false
}

func compactMockName(variation domaininstamart.ProductVariation) string {
	if strings.TrimSpace(variation.QuantityDescription) == "" {
		return variation.DisplayName
	}
	return variation.DisplayName + " " + variation.QuantityDescription
}

func cloneProduct(product domaininstamart.Product) domaininstamart.Product {
	product.Variations = append([]domaininstamart.ProductVariation(nil), product.Variations...)
	return product
}

func paginateProducts(products []domaininstamart.Product, offset int) domaininstamart.ProductSearchResult {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(products) {
		return domaininstamart.ProductSearchResult{Products: nil, NextOffset: "0"}
	}
	const pageSize = 2
	end := offset + pageSize
	if end > len(products) {
		end = len(products)
	}
	nextOffset := "0"
	if end < len(products) {
		nextOffset = fmt.Sprintf("%d", end)
	}
	return domaininstamart.ProductSearchResult{Products: products[offset:end], NextOffset: nextOffset}
}

func mockLocation() *domaininstamart.Location {
	return &domaininstamart.Location{Lat: 10.0001, Lng: 20.0001}
}
