package swiggy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	domainfood "swiggy-ssh/internal/domain/food"
)

func TestMCPFoodClientUpdateCartUsesCartItemsArgumentAndMapping(t *testing.T) {
	var args map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "update_food_cart" {
			t.Fatalf("expected update_food_cart, got %#v", params["name"])
		}
		args = params["arguments"].(map[string]any)
		writeRPCResult(t, w, map[string]any{"success": true, "data": foodCartResponseData()})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL+"/food", server.Client(), fakeAuthorizer{})
	cart, err := client.UpdateCart(context.Background(), "restaurant-1", "Pizza Place", "address-1", []domainfood.FoodCartUpdateItem{{
		MenuItemID: "item-1",
		Quantity:   2,
		VariantsV2: []domainfood.CartVariantV2{{GroupID: "group-1", VariationID: "variation-1"}},
		Addons:     []domainfood.CartAddon{{GroupID: "addon-group-1", ChoiceID: "choice-1"}},
	}})
	if err != nil {
		t.Fatalf("update food cart: %v", err)
	}
	if args["restaurantId"] != "restaurant-1" || args["restaurantName"] != "Pizza Place" || args["addressId"] != "address-1" {
		t.Fatalf("unexpected update args: %#v", args)
	}
	if _, ok := args["items"]; ok {
		t.Fatalf("update_food_cart must use cartItems, got legacy items key: %#v", args)
	}
	items := args["cartItems"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one cart item, got %#v", items)
	}
	item := items[0].(map[string]any)
	if item["menu_item_id"] != "item-1" || item["quantity"] != float64(2) {
		t.Fatalf("unexpected cart item args: %#v", item)
	}
	if _, ok := item["variants"].([]any); !ok {
		t.Fatalf("expected explicit variants array, got %#v", item["variants"])
	}
	variants := item["variantsV2"].([]any)
	if variants[0].(map[string]any)["group_id"] != "group-1" || variants[0].(map[string]any)["variation_id"] != "variation-1" {
		t.Fatalf("unexpected variantsV2 args: %#v", variants)
	}
	addons := item["addons"].([]any)
	if addons[0].(map[string]any)["group_id"] != "addon-group-1" || addons[0].(map[string]any)["choice_id"] != "choice-1" {
		t.Fatalf("unexpected addon args: %#v", addons)
	}
	if cart.RestaurantID != "restaurant-1" || cart.AddressID != "address-1" || cart.Bill.ToPayRupees != 160 || !reflect.DeepEqual(cart.AvailablePaymentMethods, []string{"Cash"}) {
		t.Fatalf("unexpected cart mapping: %#v", cart)
	}
}

func TestMCPFoodClientTrackOrderPassesSelectedOrderID(t *testing.T) {
	var args map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "track_food_order" {
			t.Fatalf("expected track_food_order, got %#v", params["name"])
		}
		args = params["arguments"].(map[string]any)
		writeRPCResult(t, w, map[string]any{"success": true, "data": map[string]any{
			"orderId":          "order-1",
			"statusMessage":    "Preparing",
			"subStatusMessage": "Kitchen accepted your order",
			"etaText":          "20 mins",
			"etaMinutes":       20,
		}})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL, server.Client(), fakeAuthorizer{})
	status, err := client.TrackOrder(context.Background(), " order-1 ")
	if err != nil {
		t.Fatalf("track food order: %v", err)
	}
	if args["orderId"] != "order-1" {
		t.Fatalf("expected selected order id, got args %#v", args)
	}
	if status.OrderID != "order-1" || status.StatusMessage != "Preparing" || status.ETAMinutes != 20 {
		t.Fatalf("unexpected tracking mapping: %#v", status)
	}
}

func TestMCPFoodClientGetCartPassesRestaurantName(t *testing.T) {
	var args map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "get_food_cart" {
			t.Fatalf("expected get_food_cart, got %#v", params["name"])
		}
		args = params["arguments"].(map[string]any)
		writeRPCResult(t, w, map[string]any{"success": true, "data": foodCartResponseData()})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL, server.Client(), fakeAuthorizer{})
	if _, err := client.GetCart(context.Background(), "address-1", "Pizza Place"); err != nil {
		t.Fatalf("get food cart: %v", err)
	}
	if args["addressId"] != "address-1" || args["restaurantName"] != "Pizza Place" {
		t.Fatalf("unexpected get_food_cart args: %#v", args)
	}
}

func TestMCPFoodClientMapsStandardMCPStructuredContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "get_addresses" {
			t.Fatalf("expected get_addresses, got %#v", params["name"])
		}
		writeRPCResult(t, w, map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "Found 1 saved address"}},
			"structuredContent": map[string]any{
				"addresses": []any{map[string]any{
					"id":              "food-address-1",
					"addressLine":     "Test Area",
					"phoneNumber":     "9876543210",
					"addressCategory": "Home",
					"addressTag":      "Home",
				}},
			},
		})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL, server.Client(), fakeAuthorizer{})
	addresses, err := client.GetAddresses(context.Background())
	if err != nil {
		t.Fatalf("get food addresses: %v", err)
	}
	if len(addresses) != 1 || addresses[0].ID != "food-address-1" || addresses[0].PhoneMasked != "****3210" {
		t.Fatalf("unexpected addresses: %#v", addresses)
	}
}

func TestMCPFoodClientMapsStructuredContentEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeRequest(t, r)
		writeRPCResult(t, w, map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "Found 1 restaurant"}},
			"structuredContent": map[string]any{
				"success": true,
				"data": map[string]any{
					"restaurants": []any{map[string]any{
						"id":           "restaurant-1",
						"name":         "Pizza Place",
						"availability": "OPEN",
					}},
				},
			},
		})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.SearchRestaurants(context.Background(), "address-1", "pizza", 0)
	if err != nil {
		t.Fatalf("search restaurants: %v", err)
	}
	if len(result.Restaurants) != 1 || result.Restaurants[0].ID != "restaurant-1" || result.Restaurants[0].Name != "Pizza Place" {
		t.Fatalf("unexpected restaurants: %#v", result.Restaurants)
	}
}

func TestMCPFoodClientMapsPlainTextRestaurantSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeRequest(t, r)
		writeRPCResult(t, w, map[string]any{"content": []any{map[string]any{"type": "text", "text": `Found 2 restaurants for "burger king":
1. Burger King — Burgers, American | 4★ | 22 min | ₹350 for two (ID: 1336699)
2. McDonald's (Ad) — Burgers, Beverages, Cafe, Desserts | 4.3★ | 26 min | ₹400 for two (ID: 23683)`}}})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.SearchRestaurants(context.Background(), "address-1", "burger king", 0)
	if err != nil {
		t.Fatalf("search restaurants: %v", err)
	}
	if len(result.Restaurants) != 2 {
		t.Fatalf("expected two restaurants, got %#v", result.Restaurants)
	}
	if result.Restaurants[0].ID != "1336699" || result.Restaurants[0].Name != "Burger King" || result.Restaurants[0].Availability != "OPEN" {
		t.Fatalf("unexpected first restaurant: %#v", result.Restaurants[0])
	}
	if !result.Restaurants[1].IsAd || result.Restaurants[1].Name != "McDonald's" {
		t.Fatalf("unexpected ad restaurant mapping: %#v", result.Restaurants[1])
	}
}

func TestMCPFoodClientMapsStructuredRestaurantSearchWithArrayCuisines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeRequest(t, r)
		writeRPCResult(t, w, map[string]any{"content": []any{map[string]any{"type": "text", "text": "Found restaurants"}}, "structuredContent": map[string]any{
			"restaurants": []any{map[string]any{
				"id":                 1336699,
				"name":               "Burger King",
				"cuisines":           []any{"Burgers", "American"},
				"rating":             4,
				"eta":                "22 min",
				"priceForTwo":        "₹350 for two",
				"availabilityStatus": "OPEN",
			}},
		}})
	}))
	defer server.Close()

	client := NewMCPFoodClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.SearchRestaurants(context.Background(), "address-1", "burger king", 0)
	if err != nil {
		t.Fatalf("search restaurants: %v", err)
	}
	if len(result.Restaurants) != 1 {
		t.Fatalf("expected one restaurant, got %#v", result.Restaurants)
	}
	restaurant := result.Restaurants[0]
	if restaurant.ID != "1336699" || restaurant.Cuisines != "Burgers, American" || restaurant.Rating != "4" || restaurant.Availability != "OPEN" {
		t.Fatalf("unexpected restaurant mapping: %#v", restaurant)
	}
}

func TestParseFoodCartTextParsesQuantityMarkers(t *testing.T) {
	data := parseFoodCartText(`Restaurant: Burger King
Items (3):
  - 2x Original Whopper Chicken — ₹418 (ID: item-1)
  - Crispy Veg Burger x 3 — ₹210 (ID: item-2)
  - Regular Fries (4 qty) — ₹360 (ID: item-3)

TO PAY: ₹988
Payment methods: Cash`)

	if len(data.Items) != 3 {
		t.Fatalf("expected three cart items, got %#v", data.Items)
	}
	tests := []struct {
		idx      int
		name     string
		quantity int
	}{
		{idx: 0, name: "Original Whopper Chicken", quantity: 2},
		{idx: 1, name: "Crispy Veg Burger", quantity: 3},
		{idx: 2, name: "Regular Fries", quantity: 4},
	}
	for _, tt := range tests {
		item := data.Items[tt.idx]
		if item.Name != tt.name || item.Quantity.Int() != tt.quantity {
			t.Fatalf("item %d: expected %q x%d, got %#v", tt.idx, tt.name, tt.quantity, item)
		}
	}
	if data.BillBreakdown.ToPay.Value.Int() != 988 || !reflect.DeepEqual(data.AvailablePaymentMethods, []string{"Cash"}) {
		t.Fatalf("unexpected cart summary: %#v", data)
	}
}

func foodCartResponseData() map[string]any {
	return map[string]any{
		"restaurantId":            "restaurant-1",
		"restaurantName":          "Pizza Place",
		"addressId":               "address-1",
		"addressLabel":            "Home",
		"cartTotalAmount":         "Rs 160",
		"availablePaymentMethods": []any{"Cash"},
		"items": []any{map[string]any{
			"menu_item_id": "item-1",
			"name":         "Margherita Pizza",
			"quantity":     2,
			"price":        80,
			"finalPrice":   160,
		}},
		"billBreakdown": map[string]any{
			"lineItems": []any{map[string]any{"label": "Item Total", "value": "Rs 160"}},
			"toPay":     map[string]any{"label": "To Pay", "value": "Rs 160"},
		},
	}
}
