package instamart

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

type fakeProvider struct {
	addresses      []domaininstamart.Address
	searchResult   domaininstamart.ProductSearchResult
	goToResult     domaininstamart.ProductSearchResult
	cart           domaininstamart.Cart
	checkoutResult domaininstamart.CheckoutResult
	orderHistory   domaininstamart.OrderHistory
	trackingStatus domaininstamart.TrackingStatus

	searchCalled    bool
	searchAddressID string
	searchQuery     string
	searchOffset    int

	goToCalled    bool
	goToAddressID string
	goToOffset    int

	getCartCalls int

	updateCalled    bool
	updateAddressID string
	updateItems     []domaininstamart.CartUpdateItem

	clearCalls int

	checkoutCalled        bool
	checkoutAddressID     string
	checkoutPaymentMethod string

	ordersCalled bool
	ordersInput  domaininstamart.OrderHistoryQuery

	trackCalled   bool
	trackOrderID  string
	trackLocation domaininstamart.Location

	calls []string
}

func (p *fakeProvider) GetAddresses(_ context.Context) ([]domaininstamart.Address, error) {
	p.calls = append(p.calls, "get_addresses")
	return p.addresses, nil
}

func (p *fakeProvider) SearchProducts(_ context.Context, addressID, query string, offset int) (domaininstamart.ProductSearchResult, error) {
	p.calls = append(p.calls, "search_products")
	p.searchCalled = true
	p.searchAddressID = addressID
	p.searchQuery = query
	p.searchOffset = offset
	return p.searchResult, nil
}

func (p *fakeProvider) YourGoToItems(_ context.Context, addressID string, offset int) (domaininstamart.ProductSearchResult, error) {
	p.calls = append(p.calls, "your_go_to_items")
	p.goToCalled = true
	p.goToAddressID = addressID
	p.goToOffset = offset
	return p.goToResult, nil
}

func (p *fakeProvider) GetCart(_ context.Context) (domaininstamart.Cart, error) {
	p.calls = append(p.calls, "get_cart")
	p.getCartCalls++
	return p.cart, nil
}

func (p *fakeProvider) UpdateCart(_ context.Context, selectedAddressID string, items []domaininstamart.CartUpdateItem) (domaininstamart.Cart, error) {
	p.calls = append(p.calls, "update_cart")
	p.updateCalled = true
	p.updateAddressID = selectedAddressID
	p.updateItems = append([]domaininstamart.CartUpdateItem(nil), items...)
	return p.cart, nil
}

func (p *fakeProvider) ClearCart(_ context.Context) error {
	p.calls = append(p.calls, "clear_cart")
	p.clearCalls++
	return nil
}

func (p *fakeProvider) Checkout(_ context.Context, addressID, paymentMethod string) (domaininstamart.CheckoutResult, error) {
	p.calls = append(p.calls, "checkout")
	p.checkoutCalled = true
	p.checkoutAddressID = addressID
	p.checkoutPaymentMethod = paymentMethod
	return p.checkoutResult, nil
}

func (p *fakeProvider) GetOrders(_ context.Context, input domaininstamart.OrderHistoryQuery) (domaininstamart.OrderHistory, error) {
	p.calls = append(p.calls, "get_orders")
	p.ordersCalled = true
	p.ordersInput = input
	return p.orderHistory, nil
}

func (p *fakeProvider) TrackOrder(_ context.Context, orderID string, location domaininstamart.Location) (domaininstamart.TrackingStatus, error) {
	p.calls = append(p.calls, "track_order")
	p.trackCalled = true
	p.trackOrderID = orderID
	p.trackLocation = location
	return p.trackingStatus, nil
}

func TestSearchProductsRejectsEmptyAddressID(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.SearchProducts(context.Background(), SearchProductsInput{AddressID: "  ", Query: "milk"})
	if !errors.Is(err, ErrAddressRequired) {
		t.Fatalf("expected ErrAddressRequired, got %v", err)
	}
	if provider.searchCalled {
		t.Fatal("provider must not be called without address")
	}
}

func TestGoToItemsRejectsEmptyAddressID(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.GetGoToItems(context.Background(), GetGoToItemsInput{AddressID: "\t"})
	if !errors.Is(err, ErrAddressRequired) {
		t.Fatalf("expected ErrAddressRequired, got %v", err)
	}
	if provider.goToCalled {
		t.Fatal("provider must not be called without address")
	}
}

func TestUpdateCartRejectsEmptyAddressID(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.UpdateCart(context.Background(), UpdateCartInput{
		Items: []domaininstamart.CartUpdateItem{{SpinID: "spin-1", Quantity: 1}},
	})
	if !errors.Is(err, ErrAddressRequired) {
		t.Fatalf("expected ErrAddressRequired, got %v", err)
	}
	if provider.updateCalled {
		t.Fatal("provider must not be called without address")
	}
}

func TestUpdateCartRejectsMissingSpinID(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.UpdateCart(context.Background(), UpdateCartInput{
		SelectedAddressID: "address-1",
		Items:             []domaininstamart.CartUpdateItem{{Quantity: 1}},
	})
	if !errors.Is(err, ErrVariantRequired) {
		t.Fatalf("expected ErrVariantRequired, got %v", err)
	}
	if provider.updateCalled {
		t.Fatal("provider must not be called without exact variation")
	}
}

func TestUpdateCartPassesFullReplacementList(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)
	items := []domaininstamart.CartUpdateItem{
		{SpinID: "spin-1", Quantity: 2},
		{SpinID: "spin-2", Quantity: 1},
	}

	_, err := service.UpdateCart(context.Background(), UpdateCartInput{
		SelectedAddressID: " address-1 ",
		Items:             items,
	})
	if err != nil {
		t.Fatalf("update cart: %v", err)
	}
	if provider.updateAddressID != "address-1" {
		t.Fatalf("expected trimmed address id, got %q", provider.updateAddressID)
	}
	if !reflect.DeepEqual(provider.updateItems, items) {
		t.Fatalf("expected full replacement list %#v, got %#v", items, provider.updateItems)
	}
}

func TestUpdateCartAllowsEmptyReplacementList(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.UpdateCart(context.Background(), UpdateCartInput{SelectedAddressID: "address-1"})
	if err != nil {
		t.Fatalf("update empty replacement list: %v", err)
	}
	if !provider.updateCalled {
		t.Fatal("provider should receive empty replacement list")
	}
	if len(provider.updateItems) != 0 {
		t.Fatalf("expected empty replacement list, got %#v", provider.updateItems)
	}
}

func TestCheckoutRejectsMissingConfirmation(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)
	review := validReview()

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		ReviewedCart:  &review,
	})
	if !errors.Is(err, ErrCheckoutRequiresConfirmation) {
		t.Fatalf("expected ErrCheckoutRequiresConfirmation, got %v", err)
	}
	if provider.getCartCalls != 0 || provider.checkoutCalled {
		t.Fatalf("provider must not be called without confirmation, calls=%v", provider.calls)
	}
}

func TestCheckoutRejectsMissingReview(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		Confirmed:     true,
	})
	if !errors.Is(err, ErrCheckoutRequiresReview) {
		t.Fatalf("expected ErrCheckoutRequiresReview, got %v", err)
	}
	if provider.getCartCalls != 0 || provider.checkoutCalled {
		t.Fatalf("provider must not be called without review, calls=%v", provider.calls)
	}
}

func TestCheckoutGetsFreshCartAndRejectsCartChangesAfterReview(t *testing.T) {
	provider := &fakeProvider{cart: validCart()}
	provider.cart.Items[0].Quantity = 2
	service := NewService(provider)
	review := validReview()

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if !errors.Is(err, ErrCheckoutRequiresReview) {
		t.Fatalf("expected ErrCheckoutRequiresReview, got %v", err)
	}
	if !reflect.DeepEqual(provider.calls, []string{"get_cart"}) {
		t.Fatalf("expected only fresh cart call, got %v", provider.calls)
	}
}

func TestCheckoutRejectsAddressIDMismatchAfterReview(t *testing.T) {
	provider := &fakeProvider{cart: validCart()}
	service := NewService(provider)
	review := validReview()

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-2",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if !errors.Is(err, ErrCheckoutRequiresReview) {
		t.Fatalf("expected ErrCheckoutRequiresReview, got %v", err)
	}
	if !reflect.DeepEqual(provider.calls, []string{"get_cart"}) {
		t.Fatalf("expected only fresh cart call, got %v", provider.calls)
	}
}

func TestCheckoutAllowsFreshCartWithoutAddressID(t *testing.T) {
	cart := validCart()
	cart.AddressID = ""
	provider := &fakeProvider{cart: cart, checkoutResult: domaininstamart.CheckoutResult{Status: "CONFIRMED"}}
	service := NewService(provider)
	review := validReview()

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if err != nil {
		t.Fatalf("checkout should tolerate missing fresh cart address ID: %v", err)
	}
	if !reflect.DeepEqual(provider.calls, []string{"get_cart", "checkout"}) {
		t.Fatalf("expected checkout after fresh cart, got %v", provider.calls)
	}
}

func TestCheckoutComparesReviewedTotalToEffectiveFreshTotal(t *testing.T) {
	cart := validCart()
	cart.Bill.ToPayRupees = 0
	cart.TotalRupees = 150
	provider := &fakeProvider{cart: cart}
	service := NewService(provider)
	review := validReview()
	review.ToPayRupees = 0

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if !errors.Is(err, ErrCheckoutRequiresReview) {
		t.Fatalf("expected ErrCheckoutRequiresReview, got %v", err)
	}
	if provider.checkoutCalled {
		t.Fatal("provider checkout must not be called when reviewed total is stale")
	}
}

func TestCheckoutBlocksAmountLimit(t *testing.T) {
	cart := validCart()
	cart.Bill.ToPayRupees = 1000
	cart.TotalRupees = 1000
	provider := &fakeProvider{cart: cart}
	service := NewService(provider)
	review := validReview()
	review.ToPayRupees = 1000

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if !errors.Is(err, ErrCheckoutAmountLimit) {
		t.Fatalf("expected ErrCheckoutAmountLimit, got %v", err)
	}
	if provider.checkoutCalled {
		t.Fatal("provider checkout must not be called over amount limit")
	}
}

func TestCheckoutRejectsUnavailablePaymentMethod(t *testing.T) {
	cart := validCart()
	cart.AvailablePaymentMethods = []string{"Cash"}
	provider := &fakeProvider{cart: cart}
	service := NewService(provider)
	review := validReview()
	review.PaymentMethod = "Card"

	_, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Card",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if !errors.Is(err, ErrPaymentMethodUnavailable) {
		t.Fatalf("expected ErrPaymentMethodUnavailable, got %v", err)
	}
	if provider.checkoutCalled {
		t.Fatal("provider checkout must not be called with unavailable payment method")
	}
}

func TestCheckoutSucceedsWithReturnedPaymentMethodBelowLimit(t *testing.T) {
	provider := &fakeProvider{
		cart: validCart(),
		checkoutResult: domaininstamart.CheckoutResult{
			Message:       "Instamart order placed successfully!",
			Status:        "CONFIRMED",
			PaymentMethod: "Cash",
			CartTotal:     150,
		},
	}
	service := NewService(provider)
	review := validReview()

	result, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     " address-1 ",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if result.Status != "CONFIRMED" {
		t.Fatalf("unexpected checkout result: %#v", result)
	}
	if !reflect.DeepEqual(provider.calls, []string{"get_cart", "checkout"}) {
		t.Fatalf("expected get_cart immediately before checkout, got %v", provider.calls)
	}
	if provider.checkoutAddressID != "address-1" || provider.checkoutPaymentMethod != "Cash" {
		t.Fatalf("unexpected checkout args: address=%q payment=%q", provider.checkoutAddressID, provider.checkoutPaymentMethod)
	}
}

func TestCheckoutSurfacesMultiStoreWarning(t *testing.T) {
	cart := validCart()
	cart.StoreIDs = []string{"store-1", "store-2"}
	provider := &fakeProvider{cart: cart, checkoutResult: domaininstamart.CheckoutResult{Status: "CONFIRMED"}}
	service := NewService(provider)
	review := validReview()

	result, err := service.Checkout(context.Background(), CheckoutInput{
		AddressID:     "address-1",
		PaymentMethod: "Cash",
		Confirmed:     true,
		ReviewedCart:  &review,
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if !result.MultiStore {
		t.Fatal("expected multi-store result warning")
	}
}

func TestTrackOrderRejectsNilOrInvalidLocation(t *testing.T) {
	tests := []struct {
		name     string
		location *domaininstamart.Location
	}{
		{name: "nil"},
		{name: "zero", location: &domaininstamart.Location{}},
		{name: "invalid latitude", location: &domaininstamart.Location{Lat: 91, Lng: 77}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &fakeProvider{}
			service := NewService(provider)

			_, err := service.TrackOrder(context.Background(), TrackOrderInput{OrderID: "order-1", Location: tt.location})
			if !errors.Is(err, ErrTrackingLocationUnavailable) {
				t.Fatalf("expected ErrTrackingLocationUnavailable, got %v", err)
			}
			if provider.trackCalled {
				t.Fatal("provider must not be called without usable location")
			}
		})
	}
}

func TestCancellationReturnsUnsupportedWithoutProviderCall(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	err := service.HandleCancellation(context.Background())
	if !errors.Is(err, ErrCancellationUnsupported) {
		t.Fatalf("expected ErrCancellationUnsupported, got %v", err)
	}
	if len(provider.calls) != 0 {
		t.Fatalf("provider must not be called for cancellation, calls=%v", provider.calls)
	}
}

func TestGetOrdersDefaultsOrderTypeToDASH(t *testing.T) {
	provider := &fakeProvider{}
	service := NewService(provider)

	_, err := service.GetOrders(context.Background(), GetOrdersInput{Count: 5, ActiveOnly: true})
	if err != nil {
		t.Fatalf("get orders: %v", err)
	}
	if provider.ordersInput.OrderType != domaininstamart.DefaultOrderType {
		t.Fatalf("expected default order type %q, got %q", domaininstamart.DefaultOrderType, provider.ordersInput.OrderType)
	}
	if !provider.ordersInput.ActiveOnly || provider.ordersInput.Count != 5 {
		t.Fatalf("unexpected order query: %#v", provider.ordersInput)
	}
}

func validReview() domaininstamart.CartReviewSnapshot {
	return domaininstamart.CartReviewSnapshot{
		AddressID:     "address-1",
		Items:         []domaininstamart.CartUpdateItem{{SpinID: "spin-1", Quantity: 1}},
		ToPayRupees:   150,
		PaymentMethod: "Cash",
	}
}

func validCart() domaininstamart.Cart {
	return domaininstamart.Cart{
		AddressID: "address-1",
		Items: []domaininstamart.CartItem{
			{SpinID: "spin-1", Name: "Milk", Quantity: 1, StoreID: "store-1", InStock: true, MRP: 80, FinalPrice: 80},
		},
		Bill: domaininstamart.BillBreakdown{
			ToPayLabel:  "To Pay",
			ToPayValue:  "₹150",
			ToPayRupees: 150,
		},
		TotalRupees:             150,
		AvailablePaymentMethods: []string{"Cash"},
		StoreIDs:                []string{"store-1"},
	}
}
