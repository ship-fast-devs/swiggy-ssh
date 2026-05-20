//go:build foodlive

package swiggy

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	domainauth "swiggy-ssh/internal/domain/auth"
	domainfood "swiggy-ssh/internal/domain/food"
	appcrypto "swiggy-ssh/internal/infrastructure/crypto"
	"swiggy-ssh/internal/infrastructure/persistence/postgres"
	"swiggy-ssh/internal/platform/config"
)

func TestLiveFoodMCPClient(t *testing.T) {
	databaseURL := os.Getenv("LIVE_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://swiggy:swiggy@localhost:5432/swiggy_ssh?sslmode=disable"
	}
	userID := os.Getenv("LIVE_USER_ID")
	if userID == "" {
		t.Fatal("LIVE_USER_ID is required")
	}
	keyText := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if keyText == "" {
		keyText = config.DefaultDevTokenEncryptionKey
	}
	key, err := base64.RawURLEncoding.DecodeString(keyText)
	if err != nil {
		t.Fatalf("decode token key: %v", err)
	}
	encryptor, err := appcrypto.NewAESGCMEncryptor(key)
	if err != nil {
		t.Fatalf("create encryptor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := postgres.NewPostgresStore(ctx, databaseURL, encryptor)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	httpClient := http.DefaultClient
	if os.Getenv("LIVE_FOOD_DEBUG_HTTP") == "1" {
		httpClient = &http.Client{Transport: foodLiveDebugTransport{t: http.DefaultTransport, tb: t}}
	}
	client := NewMCPFoodClient("https://mcp.swiggy.com/food", httpClient, NewOAuthAccountAuthorizer(store))
	ctx = domainauth.ContextWithUserID(ctx, userID)
	defer func() {
		if err := client.FlushCart(ctx); err != nil {
			t.Logf("cleanup flush food cart failed: %+v", err)
		}
	}()

	addresses, err := client.GetAddresses(ctx)
	if err != nil {
		t.Fatalf("get food addresses: %+v", err)
	}
	if len(addresses) == 0 {
		t.Fatal("get food addresses returned none")
	}
	t.Logf("food addresses: count=%d first_id=%s first_label=%s", len(addresses), addresses[0].ID, addresses[0].Label)

	result, err := client.SearchRestaurants(ctx, addresses[0].ID, "burger king", 0)
	if err != nil {
		t.Fatalf("search food restaurants: %+v", err)
	}
	if len(result.Restaurants) == 0 {
		t.Fatal("search food restaurants returned none")
	}
	restaurant := firstOpenRestaurant(result.Restaurants)
	if restaurant.ID == "" {
		t.Fatalf("search food restaurants returned no open restaurants: %#v", result.Restaurants)
	}
	t.Logf("restaurants: count=%d selected_id=%s selected_name=%s", len(result.Restaurants), restaurant.ID, restaurant.Name)

	menu, err := client.GetRestaurantMenu(ctx, addresses[0].ID, restaurant.ID, 1, 3)
	if err != nil {
		t.Fatalf("get food restaurant menu: %+v", err)
	}
	if len(menu.Categories) == 0 {
		t.Fatal("get food restaurant menu returned no categories")
	}
	t.Logf("menu: restaurant=%s categories=%d first_category=%s", menu.RestaurantName, len(menu.Categories), menu.Categories[0].Name)

	menuSearch, err := client.SearchMenu(ctx, addresses[0].ID, "whopper", restaurant.ID, 0, 0)
	if err != nil {
		t.Fatalf("search food menu: %+v", err)
	}
	if len(menuSearch.Items) == 0 {
		t.Fatal("search food menu returned no items")
	}
	item := firstSimpleFoodItem(menuSearch.Items)
	if item.ID == "" {
		item = menuSearch.Items[0]
	}
	t.Logf("menu search: items=%d selected_item_id=%s selected_item=%s addons=%d variants=%d variants_v2=%d", len(menuSearch.Items), item.ID, item.Name, len(item.Addons), len(item.Variants), len(item.VariantsV2))

	updatedCart, err := client.UpdateCart(ctx, restaurant.ID, restaurant.Name, addresses[0].ID, []domainfood.FoodCartUpdateItem{{MenuItemID: item.ID, Quantity: 1}})
	if err != nil {
		t.Fatalf("update food cart: %+v", err)
	}
	if len(updatedCart.Items) == 0 {
		t.Fatal("update food cart returned empty cart")
	}
	t.Logf("update cart: items=%d total=%d", len(updatedCart.Items), updatedCart.TotalRupees)

	cart, err := client.GetCart(ctx, addresses[0].ID, restaurant.Name)
	if err != nil {
		t.Fatalf("get food cart: %+v", err)
	}
	if len(cart.Items) == 0 {
		t.Fatal("get food cart returned empty cart after update")
	}
	t.Logf("get cart: items=%d to_pay=%d payment_methods=%v", len(cart.Items), cart.Bill.ToPayRupees, cart.AvailablePaymentMethods)

	coupons, err := client.FetchCoupons(ctx, restaurant.ID, addresses[0].ID)
	if err != nil {
		t.Fatalf("fetch food coupons: %+v", err)
	}
	t.Logf("coupons: count=%d applicable=%d", len(coupons.Coupons), coupons.Applicable)

	history, err := client.GetOrders(ctx, addresses[0].ID, false)
	if err != nil {
		t.Fatalf("get food orders: %+v", err)
	}
	t.Logf("orders: count=%d has_more=%v", len(history.Orders), history.HasMore)
	if len(history.Orders) > 0 {
		details, detailErr := client.GetOrderDetails(ctx, history.Orders[0].OrderID)
		if detailErr != nil {
			t.Fatalf("get food order details: %+v", detailErr)
		}
		t.Logf("order details: order_id=%s restaurant=%s items=%d", details.OrderID, details.RestaurantName, len(details.Items))
	}

	active, err := client.GetOrders(ctx, addresses[0].ID, true)
	if err != nil {
		t.Fatalf("get active food orders: %+v", err)
	}
	t.Logf("active orders: count=%d", len(active.Orders))
	if len(active.Orders) > 0 {
		tracking, trackErr := client.TrackOrder(ctx, active.Orders[0].OrderID)
		if trackErr != nil {
			t.Fatalf("track food order: %+v", trackErr)
		}
		t.Logf("tracking: order_id=%s status=%s eta=%s", tracking.OrderID, tracking.StatusMessage, tracking.ETAText)
	}
}

func firstOpenRestaurant(restaurants []domainfood.Restaurant) domainfood.Restaurant {
	for _, restaurant := range restaurants {
		if strings.EqualFold(strings.TrimSpace(restaurant.Availability), "OPEN") {
			return restaurant
		}
	}
	if len(restaurants) == 0 {
		return domainfood.Restaurant{}
	}
	return restaurants[0]
}

type foodLiveDebugTransport struct {
	t  http.RoundTripper
	tb testing.TB
}

func (t foodLiveDebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		t.tb.Logf("food mcp request body: %s", string(body))
	}
	resp, err := t.t.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		t.tb.Logf("food mcp response read error: %v", readErr)
	} else {
		t.tb.Logf("food mcp response status=%d body=%s", resp.StatusCode, string(body))
	}
	return resp, nil
}

func firstSimpleFoodItem(items []domainfood.MenuItemDetail) domainfood.MenuItemDetail {
	for _, item := range items {
		if len(item.Variants) == 0 && len(item.VariantsV2) == 0 && len(item.Addons) == 0 {
			return item
		}
	}
	return domainfood.MenuItemDetail{}
}
