package food

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domainfood "swiggy-ssh/internal/domain/food"
)

type fakeFoodProvider struct {
	addresses   []domainfood.Address
	cart        domainfood.FoodCart
	orderResult domainfood.FoodOrderResult
	trackStatus domainfood.FoodTrackingStatus

	addressesCalled bool
	searchCalled    bool
	updateCalled    bool
	placeCalled     bool
	trackCalled     bool
	getCartCalls    int

	getCartAddressID      string
	getCartRestaurantName string
	updateRestaurantID    string
	updateRestaurantName  string
	updateAddressID       string
	updateItems           []domainfood.FoodCartUpdateItem
	placeAddressID        string
	placePaymentMethod    string
	trackOrderID          string
	calls                 []string
}

func (p *fakeFoodProvider) GetAddresses(_ context.Context) ([]domainfood.Address, error) {
	p.calls = append(p.calls, "get_addresses")
	p.addressesCalled = true
	return p.addresses, nil
}

func (p *fakeFoodProvider) SearchRestaurants(_ context.Context, addressID, query string, offset int) (domainfood.RestaurantSearchResult, error) {
	p.calls = append(p.calls, "search_restaurants")
	p.searchCalled = true
	return domainfood.RestaurantSearchResult{}, nil
}

func (p *fakeFoodProvider) GetRestaurantMenu(_ context.Context, addressID, restaurantID string, page, pageSize int) (domainfood.MenuPage, error) {
	p.calls = append(p.calls, "get_restaurant_menu")
	return domainfood.MenuPage{}, nil
}

func (p *fakeFoodProvider) SearchMenu(_ context.Context, addressID, query, restaurantID string, vegFilter, offset int) (domainfood.MenuSearchResult, error) {
	p.calls = append(p.calls, "search_menu")
	return domainfood.MenuSearchResult{}, nil
}

func (p *fakeFoodProvider) UpdateCart(_ context.Context, restaurantID, restaurantName, addressID string, items []domainfood.FoodCartUpdateItem) (domainfood.FoodCart, error) {
	p.calls = append(p.calls, "update_food_cart")
	p.updateCalled = true
	p.updateRestaurantID = restaurantID
	p.updateRestaurantName = restaurantName
	p.updateAddressID = addressID
	p.updateItems = append([]domainfood.FoodCartUpdateItem(nil), items...)
	return p.cart, nil
}

func (p *fakeFoodProvider) GetCart(_ context.Context, addressID, restaurantName string) (domainfood.FoodCart, error) {
	p.calls = append(p.calls, "get_food_cart")
	p.getCartCalls++
	p.getCartAddressID = addressID
	p.getCartRestaurantName = restaurantName
	return p.cart, nil
}

func (p *fakeFoodProvider) FetchCoupons(_ context.Context, restaurantID, addressID string) (domainfood.FoodCouponsResult, error) {
	p.calls = append(p.calls, "fetch_food_coupons")
	return domainfood.FoodCouponsResult{}, nil
}

func (p *fakeFoodProvider) ApplyCoupon(_ context.Context, couponCode, addressID string) error {
	p.calls = append(p.calls, "apply_food_coupon")
	return nil
}

func (p *fakeFoodProvider) PlaceOrder(_ context.Context, addressID, paymentMethod string) (domainfood.FoodOrderResult, error) {
	p.calls = append(p.calls, "place_food_order")
	p.placeCalled = true
	p.placeAddressID = addressID
	p.placePaymentMethod = paymentMethod
	return p.orderResult, nil
}

func (p *fakeFoodProvider) GetOrders(_ context.Context, addressID string, activeOnly bool) (domainfood.FoodOrderHistory, error) {
	p.calls = append(p.calls, "get_food_orders")
	return domainfood.FoodOrderHistory{}, nil
}

func (p *fakeFoodProvider) GetOrderDetails(_ context.Context, orderID string) (domainfood.FoodOrderDetails, error) {
	p.calls = append(p.calls, "get_food_order_details")
	return domainfood.FoodOrderDetails{}, nil
}

func (p *fakeFoodProvider) TrackOrder(_ context.Context, orderID string) (domainfood.FoodTrackingStatus, error) {
	p.calls = append(p.calls, "track_food_order")
	p.trackCalled = true
	p.trackOrderID = orderID
	return p.trackStatus, nil
}

func (p *fakeFoodProvider) FlushCart(_ context.Context) error {
	p.calls = append(p.calls, "flush_food_cart")
	return nil
}

func TestGetAddressesUsesFoodProvider(t *testing.T) {
	provider := &fakeFoodProvider{addresses: []domainfood.Address{{ID: "food-address-1", Label: "Home"}}}
	service := NewService(provider)

	addresses, err := service.GetAddresses(context.Background())
	if err != nil {
		t.Fatalf("get addresses: %v", err)
	}
	if !provider.addressesCalled || len(addresses) != 1 || addresses[0].ID != "food-address-1" {
		t.Fatalf("unexpected addresses: called=%v addresses=%#v", provider.addressesCalled, addresses)
	}
}

func TestSearchRestaurantsRejectsEmptyAddressID(t *testing.T) {
	provider := &fakeFoodProvider{}
	service := NewService(provider)

	_, err := service.SearchRestaurants(context.Background(), SearchRestaurantsInput{AddressID: "  ", Query: "pizza"})
	if !errors.Is(err, ErrAddressRequired) {
		t.Fatalf("expected ErrAddressRequired, got %v", err)
	}
	if provider.searchCalled {
		t.Fatal("provider must not be called without address")
	}
}

func TestUpdateCartRejectsMissingRestaurantID(t *testing.T) {
	provider := &fakeFoodProvider{}
	service := NewService(provider)

	_, err := service.UpdateCart(context.Background(), UpdateCartInput{AddressID: "address-1", Items: []domainfood.FoodCartUpdateItem{{MenuItemID: "item-1", Quantity: 1}}})
	if !errors.Is(err, ErrRestaurantRequired) {
		t.Fatalf("expected ErrRestaurantRequired, got %v", err)
	}
	if provider.updateCalled {
		t.Fatal("provider must not be called without restaurant")
	}
}

func TestUpdateCartTrimsBoundaryIDs(t *testing.T) {
	provider := &fakeFoodProvider{}
	service := NewService(provider)
	items := []domainfood.FoodCartUpdateItem{{MenuItemID: "item-1", Quantity: 1}}

	_, err := service.UpdateCart(context.Background(), UpdateCartInput{RestaurantID: " restaurant-1 ", RestaurantName: "Pizza Place", AddressID: " address-1 ", Items: items})
	if err != nil {
		t.Fatalf("update cart: %v", err)
	}
	if provider.updateRestaurantID != "restaurant-1" || provider.updateAddressID != "address-1" || provider.updateRestaurantName != "Pizza Place" {
		t.Fatalf("unexpected update args: restaurant=%q name=%q address=%q", provider.updateRestaurantID, provider.updateRestaurantName, provider.updateAddressID)
	}
	if !reflect.DeepEqual(provider.updateItems, items) {
		t.Fatalf("expected update items %#v, got %#v", items, provider.updateItems)
	}
}

func TestPlaceOrderRejectsMissingConfirmation(t *testing.T) {
	provider := &fakeFoodProvider{}
	service := NewService(provider)
	review := validFoodReview()

	_, err := service.PlaceOrder(context.Background(), PlaceOrderInput{AddressID: "address-1", PaymentMethod: "Cash", ReviewedCart: &review})
	if !errors.Is(err, ErrCheckoutRequiresConfirmation) {
		t.Fatalf("expected ErrCheckoutRequiresConfirmation, got %v", err)
	}
	if provider.getCartCalls != 0 || provider.placeCalled {
		t.Fatalf("provider must not be called without confirmation, calls=%v", provider.calls)
	}
}

func TestPlaceOrderRejectsMissingReview(t *testing.T) {
	provider := &fakeFoodProvider{}
	service := NewService(provider)

	_, err := service.PlaceOrder(context.Background(), PlaceOrderInput{AddressID: "address-1", PaymentMethod: "Cash", Confirmed: true})
	if !errors.Is(err, ErrCheckoutRequiresReview) {
		t.Fatalf("expected ErrCheckoutRequiresReview, got %v", err)
	}
	if provider.getCartCalls != 0 || provider.placeCalled {
		t.Fatalf("provider must not be called without review, calls=%v", provider.calls)
	}
}

func TestPlaceOrderGetsFreshCartAndRejectsCartChangesAfterReview(t *testing.T) {
	cart := validFoodCart()
	cart.Items[0].Quantity = 2
	provider := &fakeFoodProvider{cart: cart}
	service := NewService(provider)
	review := validFoodReview()

	_, err := service.PlaceOrder(context.Background(), PlaceOrderInput{AddressID: "address-1", PaymentMethod: "Cash", Confirmed: true, ReviewedCart: &review})
	if !errors.Is(err, ErrCheckoutRequiresReview) {
		t.Fatalf("expected ErrCheckoutRequiresReview, got %v", err)
	}
	if !reflect.DeepEqual(provider.calls, []string{"get_food_cart"}) {
		t.Fatalf("expected only fresh cart call, got %v", provider.calls)
	}
}

func TestPlaceOrderBlocksAmountLimit(t *testing.T) {
	cart := validFoodCart()
	cart.Bill.ToPayRupees = 1000
	cart.TotalRupees = 1000
	provider := &fakeFoodProvider{cart: cart}
	service := NewService(provider)
	review := validFoodReview()
	review.ToPayRupees = 1000

	_, err := service.PlaceOrder(context.Background(), PlaceOrderInput{AddressID: "address-1", PaymentMethod: "Cash", Confirmed: true, ReviewedCart: &review})
	if !errors.Is(err, ErrCartAmountLimit) {
		t.Fatalf("expected ErrCartAmountLimit, got %v", err)
	}
	if provider.placeCalled {
		t.Fatal("provider order must not be called over amount limit")
	}
}

func TestPlaceOrderRejectsUnavailablePaymentMethod(t *testing.T) {
	cart := validFoodCart()
	cart.AvailablePaymentMethods = []string{"Cash"}
	provider := &fakeFoodProvider{cart: cart}
	service := NewService(provider)
	review := validFoodReview()
	review.PaymentMethod = "Card"

	_, err := service.PlaceOrder(context.Background(), PlaceOrderInput{AddressID: "address-1", PaymentMethod: "Card", Confirmed: true, ReviewedCart: &review})
	if !errors.Is(err, ErrPaymentMethodUnavailable) {
		t.Fatalf("expected ErrPaymentMethodUnavailable, got %v", err)
	}
	if provider.placeCalled {
		t.Fatal("provider order must not be called with unavailable payment method")
	}
}

func TestPlaceOrderSucceedsWithFreshCartBelowLimit(t *testing.T) {
	provider := &fakeFoodProvider{cart: validFoodCart(), orderResult: domainfood.FoodOrderResult{Status: "CONFIRMED", PaymentMethod: "Cash", CartTotal: 160}}
	service := NewService(provider)
	review := validFoodReview()

	result, err := service.PlaceOrder(context.Background(), PlaceOrderInput{AddressID: " address-1 ", RestaurantName: " Pizza Place ", PaymentMethod: "Cash", Confirmed: true, ReviewedCart: &review})
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	if result.Status != "CONFIRMED" {
		t.Fatalf("unexpected order result: %#v", result)
	}
	if !reflect.DeepEqual(provider.calls, []string{"get_food_cart", "place_food_order"}) {
		t.Fatalf("expected fresh cart then order, got %v", provider.calls)
	}
	if provider.placeAddressID != "address-1" || provider.placePaymentMethod != "Cash" {
		t.Fatalf("unexpected order args: address=%q payment=%q", provider.placeAddressID, provider.placePaymentMethod)
	}
	if provider.getCartAddressID != "address-1" || provider.getCartRestaurantName != "Pizza Place" {
		t.Fatalf("unexpected fresh cart args: address=%q restaurant=%q", provider.getCartAddressID, provider.getCartRestaurantName)
	}
}

func TestTrackOrderPassesSelectedOrderID(t *testing.T) {
	provider := &fakeFoodProvider{trackStatus: domainfood.FoodTrackingStatus{OrderID: "order-1", StatusMessage: "Preparing"}}
	service := NewService(provider)

	status, err := service.TrackOrder(context.Background(), TrackOrderInput{OrderID: " order-1 "})
	if err != nil {
		t.Fatalf("track order: %v", err)
	}
	if provider.trackOrderID != "order-1" || status.StatusMessage != "Preparing" {
		t.Fatalf("unexpected tracking: orderID=%q status=%#v", provider.trackOrderID, status)
	}
}

func validFoodReview() domainfood.FoodCartReviewSnapshot {
	return domainfood.FoodCartReviewSnapshot{
		AddressID:     "address-1",
		RestaurantID:  "restaurant-1",
		Items:         []domainfood.FoodCartUpdateItem{{MenuItemID: "item-1", Quantity: 1}},
		ToPayRupees:   160,
		PaymentMethod: "Cash",
	}
}

func validFoodCart() domainfood.FoodCart {
	return domainfood.FoodCart{
		RestaurantID: "restaurant-1",
		AddressID:    "address-1",
		Items: []domainfood.FoodCartItem{
			{MenuItemID: "item-1", Name: "Margherita Pizza", Quantity: 1, Price: 109, FinalPrice: 160},
		},
		Bill: domainfood.BillBreakdown{
			ToPayLabel:  "To Pay",
			ToPayValue:  "Rs 160",
			ToPayRupees: 160,
		},
		TotalRupees:             160,
		AvailablePaymentMethods: []string{"Cash"},
	}
}
