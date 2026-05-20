package mock

import (
	"context"
	"reflect"
	"testing"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

func TestInstamartProviderSupportsLocalSmokeFlow(t *testing.T) {
	ctx := context.Background()
	provider := NewInstamartProvider()

	addresses, err := provider.GetAddresses(ctx)
	if err != nil {
		t.Fatalf("get addresses: %v", err)
	}
	if len(addresses) < 2 || addresses[0].ID == "" || addresses[0].PhoneMasked == "" {
		t.Fatalf("unexpected addresses: %#v", addresses)
	}

	search, err := provider.SearchProducts(ctx, addresses[0].ID, "milk", 0)
	if err != nil {
		t.Fatalf("search products: %v", err)
	}
	if len(search.Products) != 1 || len(search.Products[0].Variations) == 0 {
		t.Fatalf("expected milk with variations, got %#v", search)
	}

	goTo, err := provider.YourGoToItems(ctx, addresses[0].ID, 0)
	if err != nil {
		t.Fatalf("go-to items: %v", err)
	}
	if len(goTo.Products) == 0 || goTo.NextOffset == "" {
		t.Fatalf("expected go-to products with next offset, got %#v", goTo)
	}

	firstCart, err := provider.UpdateCart(ctx, addresses[0].ID, []domaininstamart.CartUpdateItem{{SpinID: "mock-spin-milk-500", Quantity: 2}, {SpinID: "mock-spin-bread-400", Quantity: 1}})
	if err != nil {
		t.Fatalf("update cart: %v", err)
	}
	if len(firstCart.Items) != 2 || firstCart.Bill.ToPayRupees <= 0 || !reflect.DeepEqual(firstCart.AvailablePaymentMethods, []string{"Cash"}) {
		t.Fatalf("unexpected cart after update: %#v", firstCart)
	}
	if len(firstCart.StoreIDs) != 2 {
		t.Fatalf("expected multi-store mock cart, got %#v", firstCart.StoreIDs)
	}

	replacement, err := provider.UpdateCart(ctx, addresses[1].ID, []domaininstamart.CartUpdateItem{{SpinID: "mock-spin-banana-6", Quantity: 1}})
	if err != nil {
		t.Fatalf("replace cart: %v", err)
	}
	if replacement.AddressID != addresses[1].ID || len(replacement.Items) != 1 || replacement.Items[0].SpinID != "mock-spin-banana-6" {
		t.Fatalf("expected full replacement cart, got %#v", replacement)
	}

	checkout, err := provider.Checkout(ctx, addresses[1].ID, "Cash")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if checkout.Status != "CONFIRMED" || checkout.PaymentMethod != "Cash" || checkout.CartTotal >= 1000 || len(checkout.OrderIDs) != 1 {
		t.Fatalf("unexpected checkout result: %#v", checkout)
	}

	activeOrders, err := provider.GetOrders(ctx, domaininstamart.OrderHistoryQuery{ActiveOnly: true})
	if err != nil {
		t.Fatalf("get active orders: %v", err)
	}
	if len(activeOrders.Orders) != 1 || !activeOrders.Orders[0].Active || activeOrders.Orders[0].Location == nil {
		t.Fatalf("unexpected active orders: %#v", activeOrders)
	}

	history, err := provider.GetOrders(ctx, domaininstamart.OrderHistoryQuery{})
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(history.Orders) < 2 {
		t.Fatalf("expected active and history orders, got %#v", history)
	}

	tracking, err := provider.TrackOrder(ctx, checkout.OrderIDs[0], *activeOrders.Orders[0].Location)
	if err != nil {
		t.Fatalf("track order: %v", err)
	}
	if tracking.OrderID != checkout.OrderIDs[0] || tracking.ETAMinutes <= 0 || tracking.PollingIntervalSeconds <= 0 {
		t.Fatalf("unexpected tracking: %#v", tracking)
	}

	if err := provider.ClearCart(ctx); err != nil {
		t.Fatalf("clear cart: %v", err)
	}
	emptyCart, err := provider.GetCart(ctx)
	if err != nil {
		t.Fatalf("get empty cart: %v", err)
	}
	if len(emptyCart.Items) != 0 || emptyCart.TotalRupees != 0 {
		t.Fatalf("expected empty cart, got %#v", emptyCart)
	}
}
