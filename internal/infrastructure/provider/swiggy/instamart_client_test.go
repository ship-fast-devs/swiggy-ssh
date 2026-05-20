package swiggy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

type fakeAuthorizer struct{}

func (fakeAuthorizer) AuthorizeMCPRequest(_ context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer fake-test-token")
	return nil
}

func TestMCPClientPostsJSONRPCToolCallToConfiguredEndpoint(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/im" {
			t.Fatalf("expected /im path, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("expected authorizer to decorate request")
		}
		if r.Header.Get("Accept") != "application/json, text/event-stream" {
			t.Fatalf("expected MCP streamable accept header, got %q", r.Header.Get("Accept"))
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(t, w, map[string]any{"success": true, "data": map[string]any{"addresses": []any{map[string]any{"id": "address-1", "addressLine": "Test Area", "phoneNumber": "9876543210", "addressCategory": "Home", "addressTag": "Home"}}}})
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL+"/im", server.Client(), fakeAuthorizer{})
	addresses, err := client.GetAddresses(context.Background())
	if err != nil {
		t.Fatalf("get addresses: %v", err)
	}
	if captured["method"] != "tools/call" {
		t.Fatalf("expected tools/call, got %#v", captured["method"])
	}
	params := captured["params"].(map[string]any)
	if params["name"] != "get_addresses" {
		t.Fatalf("expected unqualified get_addresses, got %#v", params["name"])
	}
	if _, ok := params["arguments"]; ok {
		t.Fatalf("get_addresses should not send arguments, got %#v", params["arguments"])
	}
	if len(addresses) != 1 || addresses[0].ID != "address-1" || addresses[0].Label != "Home" || addresses[0].PhoneMasked != "****3210" {
		t.Fatalf("unexpected address mapping: %#v", addresses)
	}
}

func TestMCPClientSearchProductsRequestAndMapping(t *testing.T) {
	var args map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "search_products" {
			t.Fatalf("expected search_products, got %#v", params["name"])
		}
		args = params["arguments"].(map[string]any)
		writeRPCResult(t, w, map[string]any{"success": true, "data": map[string]any{
			"nextOffset": "7",
			"products": []any{map[string]any{
				"productId":       "product-1",
				"parentProductId": "parent-1",
				"displayName":     "Test Milk",
				"brand":           "Test Dairy",
				"inStock":         true,
				"isAvail":         true,
				"isPromoted":      true,
				"variations": []any{map[string]any{
					"spinId":                "spin-1",
					"quantityDescription":   "1 L",
					"displayName":           "Test Milk",
					"brandName":             "Test Dairy",
					"price":                 map[string]any{"mrp": 64, "offerPrice": 60},
					"isInStockAndAvailable": true,
					"imageUrl":              "https://example.invalid/milk.png",
				}},
			}},
		}})
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.SearchProducts(context.Background(), "address-1", "milk", 3)
	if err != nil {
		t.Fatalf("search products: %v", err)
	}
	if args["addressId"] != "address-1" || args["query"] != "milk" || args["offset"] != float64(3) {
		t.Fatalf("unexpected search args: %#v", args)
	}
	if result.NextOffset != "7" || len(result.Products) != 1 {
		t.Fatalf("unexpected search result: %#v", result)
	}
	product := result.Products[0]
	if !product.Promoted || product.ID != "product-1" || !product.Available || len(product.Variations) != 1 {
		t.Fatalf("unexpected product mapping: %#v", product)
	}
	variation := product.Variations[0]
	if variation.SpinID != "spin-1" || variation.QuantityDescription != "1 L" || variation.Price.OfferPrice != 60 || !variation.InStock {
		t.Fatalf("unexpected variation mapping: %#v", variation)
	}
}

func TestMCPClientUnwrapsTextContentResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "your_go_to_items" {
			t.Fatalf("expected your_go_to_items, got %#v", params["name"])
		}
		writeRPCResult(t, w, map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "not json"},
			textEnvelope(t, map[string]any{"success": true, "data": map[string]any{"nextOffset": 0, "products": []any{}}}),
		}})
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.YourGoToItems(context.Background(), "address-1", 0)
	if err != nil {
		t.Fatalf("your go-to items: %v", err)
	}
	if result.NextOffset != "0" {
		t.Fatalf("expected numeric nextOffset to map to string 0, got %#v", result.NextOffset)
	}
}

func TestMCPClientNoArgumentToolRequests(t *testing.T) {
	var seen []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		seen = append(seen, params)
		switch params["name"] {
		case "get_cart":
			writeRPCResult(t, w, map[string]any{"success": true, "data": cartResponseData()})
		case "clear_cart":
			writeRPCResult(t, w, map[string]any{"success": true})
		default:
			t.Fatalf("unexpected tool: %#v", params["name"])
		}
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	if _, err := client.GetCart(context.Background()); err != nil {
		t.Fatalf("get cart: %v", err)
	}
	if err := client.ClearCart(context.Background()); err != nil {
		t.Fatalf("clear cart: %v", err)
	}
	for _, params := range seen {
		if _, ok := params["arguments"]; ok {
			t.Fatalf("%s should not send arguments, got %#v", params["name"], params["arguments"])
		}
	}
}

func TestMCPClientUpdateCartRequestAndCartMapping(t *testing.T) {
	var args map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "update_cart" {
			t.Fatalf("expected update_cart, got %#v", params["name"])
		}
		args = params["arguments"].(map[string]any)
		writeRPCResult(t, w, map[string]any{"success": true, "data": cartResponseData()})
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	cart, err := client.UpdateCart(context.Background(), "address-1", []domaininstamart.CartUpdateItem{{SpinID: "spin-1", Quantity: 2}, {SpinID: "spin-2", Quantity: 1}})
	if err != nil {
		t.Fatalf("update cart: %v", err)
	}
	if args["selectedAddressId"] != "address-1" {
		t.Fatalf("unexpected selected address: %#v", args)
	}
	items := args["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["spinId"] != "spin-1" || items[1].(map[string]any)["quantity"] != float64(1) {
		t.Fatalf("expected full replacement items, got %#v", items)
	}
	if cart.AddressID != "address-1" || cart.AddressLabel != "Test Home" || cart.AddressLocation == nil {
		t.Fatalf("unexpected cart address mapping: %#v", cart)
	}
	if cart.TotalRupees != 150 || cart.Bill.ToPayRupees != 150 || !reflect.DeepEqual(cart.AvailablePaymentMethods, []string{"Cash"}) {
		t.Fatalf("unexpected cart bill/payment mapping: %#v", cart)
	}
	if len(cart.Items) != 1 || cart.Items[0].StoreID != "1402444" || !reflect.DeepEqual(cart.StoreIDs, []string{"1402444"}) {
		t.Fatalf("unexpected cart item/store mapping: %#v", cart)
	}
}

func TestMCPClientCheckoutRequestAndMapping(t *testing.T) {
	var args map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		if params["name"] != "checkout" {
			t.Fatalf("expected checkout, got %#v", params["name"])
		}
		args = params["arguments"].(map[string]any)
		writeRPCResult(t, w, map[string]any{"success": true, "message": "Instamart order placed successfully!", "data": map[string]any{"orderId": "order-1", "status": "CONFIRMED", "paymentMethod": "Cash", "cartTotal": 150}})
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.Checkout(context.Background(), "address-1", "Cash")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if args["addressId"] != "address-1" || args["paymentMethod"] != "Cash" {
		t.Fatalf("unexpected checkout args: %#v", args)
	}
	if result.Message != "Instamart order placed successfully!" || result.Status != "CONFIRMED" || result.CartTotal != 150 || !reflect.DeepEqual(result.OrderIDs, []string{"order-1"}) {
		t.Fatalf("unexpected checkout mapping: %#v", result)
	}
}

func TestMCPClientCheckoutMapsMultiStoreOrderArrays(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeRequest(t, r)
		writeRPCResult(t, w, map[string]any{"success": true, "message": "Instamart order placed successfully!", "data": map[string]any{
			"results": []any{
				map[string]any{"orderId": "order-1", "status": "CONFIRMED"},
				map[string]any{"orderId": "order-2", "status": "CONFIRMED"},
			},
			"paymentMethod": "Cash",
			"cartTotal":     220,
		}})
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	result, err := client.Checkout(context.Background(), "address-1", "Cash")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if !result.MultiStore || !reflect.DeepEqual(result.OrderIDs, []string{"order-1", "order-2"}) || result.Status != "CONFIRMED" || result.CartTotal != 220 {
		t.Fatalf("unexpected multi-store checkout mapping: %#v", result)
	}
}

func TestMCPClientGetOrdersDefaultsOrderTypeAndTrackOrderArgs(t *testing.T) {
	var seen []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := decodeRequest(t, r)
		params := request["params"].(map[string]any)
		seen = append(seen, params)
		switch params["name"] {
		case "get_orders":
			writeRPCResult(t, w, map[string]any{"success": true, "data": map[string]any{"hasMore": false, "orders": []any{map[string]any{"orderId": "order-1", "status": "CONFIRMED", "itemCount": 1, "totalAmount": 150, "paymentMethod": "Cash", "isActive": true}}}})
		case "track_order":
			writeRPCResult(t, w, map[string]any{"success": true, "data": map[string]any{"orderId": "order-1", "status": map[string]any{"statusMessage": "Packing", "etaMinutes": 5, "etaText": "5 mins"}, "items": []any{map[string]any{"name": "1 x Test Milk", "quantity": 1, "price": "Rs 60"}}, "pollingIntervalSeconds": 30}})
		default:
			t.Fatalf("unexpected tool: %#v", params["name"])
		}
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	history, err := client.GetOrders(context.Background(), domaininstamart.OrderHistoryQuery{Count: 5, ActiveOnly: true})
	if err != nil {
		t.Fatalf("get orders: %v", err)
	}
	status, err := client.TrackOrder(context.Background(), "order-1", domaininstamart.Location{Lat: 10.1, Lng: 20.2})
	if err != nil {
		t.Fatalf("track order: %v", err)
	}
	orderArgs := seen[0]["arguments"].(map[string]any)
	if orderArgs["orderType"] != domaininstamart.DefaultOrderType || orderArgs["count"] != float64(5) || orderArgs["activeOnly"] != true {
		t.Fatalf("unexpected get_orders args: %#v", orderArgs)
	}
	trackArgs := seen[1]["arguments"].(map[string]any)
	if trackArgs["orderId"] != "order-1" || trackArgs["lat"] != 10.1 || trackArgs["lng"] != 20.2 {
		t.Fatalf("unexpected track_order args: %#v", trackArgs)
	}
	if len(history.Orders) != 1 || !history.Orders[0].Active {
		t.Fatalf("unexpected history mapping: %#v", history)
	}
	if status.StatusMessage != "Packing" || status.ETAMinutes != 5 || status.PollingIntervalSeconds != 30 || len(status.Items) != 1 {
		t.Fatalf("unexpected tracking mapping: %#v", status)
	}
}

func TestMCPClientMapsMCPAndToolErrorsSafely(t *testing.T) {
	tests := []struct {
		name     string
		result   any
		contains string
	}{
		{name: "json rpc error", result: map[string]any{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32000, "message": "upstream failed"}}, contains: "mcp error -32000: upstream failed"},
		{name: "tool error", result: map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"success": false, "error": map[string]any{"message": "cart unavailable"}}}, contains: "cart unavailable"},
		{name: "sensitive tool error", result: map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"success": false, "error": map[string]any{"message": "failed for phone 9876543210 at address secret"}}}, contains: "tool returned unsuccessful response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = decodeRequest(t, r)
				if err := json.NewEncoder(w).Encode(tt.result); err != nil {
					t.Fatalf("write response: %v", err)
				}
			}))
			defer server.Close()

			client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
			_, err := client.GetCart(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("expected error containing %q, got %v", tt.contains, err)
			}
		})
	}
}

func TestMCPClientRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"success":true,"data":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", maxMCPResponseBytes+1)))
		_, _ = w.Write([]byte(`"}}`))
	}))
	defer server.Close()

	client := NewMCPInstamartClient(server.URL, server.Client(), fakeAuthorizer{})
	_, err := client.GetCart(context.Background())
	if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestSafeUpstreamMessageTruncatesUTF8ByRune(t *testing.T) {
	message := strings.Repeat("界", 200)

	got := safeUpstreamMessage(message, "fallback")

	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8 after truncation, got %q", got)
	}
	if len([]rune(got)) != 160 {
		t.Fatalf("expected 160 runes, got %d", len([]rune(got)))
	}
}

func TestMCPClientRequiresAuthorizer(t *testing.T) {
	client := NewMCPInstamartClient("https://example.invalid/im", http.DefaultClient, nil)
	_, err := client.GetAddresses(context.Background())
	if !errors.Is(err, errMCPAuthorizerRequired()) {
		t.Fatalf("expected authorizer error, got %v", err)
	}
}

func decodeRequest(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var request map[string]any
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if request["jsonrpc"] != "2.0" || request["method"] != "tools/call" {
		t.Fatalf("unexpected json-rpc request: %#v", request)
	}
	return request
}

func writeRPCResult(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result}); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func writeRPCTextResult(t *testing.T, w http.ResponseWriter, envelope any) {
	t.Helper()
	writeRPCResult(t, w, map[string]any{"content": []any{textEnvelope(t, envelope)}})
}

func textEnvelope(t *testing.T, envelope any) map[string]any {
	t.Helper()
	text, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal text envelope: %v", err)
	}
	return map[string]any{"type": "text", "text": string(text)}
}

func cartResponseData() map[string]any {
	return map[string]any{
		"selectedAddress": "address-1",
		"selectedAddressDetails": map[string]any{
			"annotation": "Test Home",
			"area":       "Test Area",
			"city":       "Test City",
			"lat":        "10.1",
			"lng":        "20.2",
		},
		"cartTotalAmount": "Rs 150",
		"items": []any{map[string]any{
			"spinId":                "spin-1",
			"itemName":              "Test Milk 1 L",
			"quantity":              2,
			"storeId":               1402444,
			"isInStockAndAvailable": true,
			"mrp":                   64,
			"discountedFinalPrice":  120,
		}},
		"billBreakdown": map[string]any{
			"lineItems": []any{map[string]any{"label": "Item Total", "value": "Rs 120"}},
			"toPay":     map[string]any{"label": "To Pay", "value": "Rs 150"},
		},
		"availablePaymentMethods": []any{"Cash"},
	}
}
