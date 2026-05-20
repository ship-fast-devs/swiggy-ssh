package swiggy

import (
	"context"
	"encoding/json"
	"strings"

	domainfood "swiggy-ssh/internal/domain/food"
)

var _ domainfood.Provider = (*MCPFoodClient)(nil)

func (c *MCPFoodClient) GetAddresses(ctx context.Context) ([]domainfood.Address, error) {
	var data mcpAddressesData
	if err := c.callTool(ctx, "get_addresses", nil, &data); err != nil {
		return nil, err
	}
	return mapFoodAddresses(data), nil
}

func (c *MCPFoodClient) SearchRestaurants(ctx context.Context, addressID, query string, offset int) (domainfood.RestaurantSearchResult, error) {
	var data mcpRestaurantSearchData
	if err := c.callTool(ctx, "search_restaurants", map[string]any{
		"addressId": addressID,
		"query":     query,
		"offset":    offset,
	}, &data); err != nil {
		return domainfood.RestaurantSearchResult{}, err
	}
	return mapRestaurantSearch(data), nil
}

func (c *MCPFoodClient) GetRestaurantMenu(ctx context.Context, addressID, restaurantID string, page, pageSize int) (domainfood.MenuPage, error) {
	var data mcpMenuPage
	if err := c.callTool(ctx, "get_restaurant_menu", map[string]any{
		"addressId":    addressID,
		"restaurantId": restaurantID,
		"page":         page,
		"pageSize":     pageSize,
	}, &data); err != nil {
		return domainfood.MenuPage{}, err
	}
	return mapMenuPage(data), nil
}

func (c *MCPFoodClient) SearchMenu(ctx context.Context, addressID, query, restaurantID string, vegFilter, offset int) (domainfood.MenuSearchResult, error) {
	var data mcpMenuSearchData
	if err := c.callTool(ctx, "search_menu", map[string]any{
		"addressId":               addressID,
		"query":                   query,
		"restaurantIdOfAddedItem": restaurantID,
		"vegFilter":               vegFilter,
		"offset":                  offset,
	}, &data); err != nil {
		return domainfood.MenuSearchResult{}, err
	}
	return mapMenuSearch(data), nil
}

func (c *MCPFoodClient) UpdateCart(ctx context.Context, restaurantID, restaurantName, addressID string, items []domainfood.FoodCartUpdateItem) (domainfood.FoodCart, error) {
	cartItems := make([]map[string]any, 0, len(items))
	for _, fi := range items {
		item := map[string]any{
			"menu_item_id": fi.MenuItemID,
			"quantity":     fi.Quantity,
			"variants":     []map[string]any{},
			"variantsV2":   []map[string]any{},
			"addons":       []map[string]any{},
		}
		if len(fi.VariantsV2) > 0 {
			v2 := make([]map[string]any, 0, len(fi.VariantsV2))
			for _, v := range fi.VariantsV2 {
				v2 = append(v2, map[string]any{
					"group_id":     v.GroupID,
					"variation_id": v.VariationID,
				})
			}
			item["variantsV2"] = v2
		} else if len(fi.Variants) > 0 {
			v := make([]map[string]any, 0, len(fi.Variants))
			for _, variant := range fi.Variants {
				v = append(v, map[string]any{
					"group_id":     variant.GroupID,
					"variation_id": variant.VariationID,
				})
			}
			item["variants"] = v
		}
		if len(fi.Addons) > 0 {
			addons := make([]map[string]any, 0, len(fi.Addons))
			for _, a := range fi.Addons {
				addons = append(addons, map[string]any{
					"group_id":  a.GroupID,
					"choice_id": a.ChoiceID,
				})
			}
			item["addons"] = addons
		}
		cartItems = append(cartItems, item)
	}

	var data mcpFoodCartData
	if err := c.callTool(ctx, "update_food_cart", map[string]any{
		"restaurantId":   restaurantID,
		"restaurantName": restaurantName,
		"addressId":      addressID,
		"cartItems":      cartItems,
	}, &data); err != nil {
		return domainfood.FoodCart{}, err
	}
	return mapFoodCart(data), nil
}

func (c *MCPFoodClient) GetCart(ctx context.Context, addressID, restaurantName string) (domainfood.FoodCart, error) {
	var data mcpFoodCartData
	if err := c.callTool(ctx, "get_food_cart", map[string]any{
		"addressId":      addressID,
		"restaurantName": restaurantName,
	}, &data); err != nil {
		return domainfood.FoodCart{}, err
	}
	return mapFoodCart(data), nil
}

func (c *MCPFoodClient) FetchCoupons(ctx context.Context, restaurantID, addressID string) (domainfood.FoodCouponsResult, error) {
	var data mcpFoodCouponsData
	if err := c.callTool(ctx, "fetch_food_coupons", map[string]any{
		"restaurantId": restaurantID,
		"addressId":    addressID,
	}, &data); err != nil {
		return domainfood.FoodCouponsResult{}, err
	}
	return mapFoodCoupons(data), nil
}

func (c *MCPFoodClient) ApplyCoupon(ctx context.Context, couponCode, addressID string) error {
	return c.callTool(ctx, "apply_food_coupon", map[string]any{
		"couponCode": couponCode,
		"addressId":  addressID,
	}, nil)
}

func (c *MCPFoodClient) PlaceOrder(ctx context.Context, addressID, paymentMethod string) (domainfood.FoodOrderResult, error) {
	envelope, err := c.callToolEnvelope(ctx, "place_food_order", map[string]any{
		"addressId":     addressID,
		"paymentMethod": paymentMethod,
	})
	if err != nil {
		return domainfood.FoodOrderResult{}, err
	}
	var data mcpFoodOrderResult
	if len(envelope.Data) != 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return domainfood.FoodOrderResult{}, err
		}
	}
	return mapFoodOrderResult(data, envelope.Message), nil
}

func (c *MCPFoodClient) GetOrders(ctx context.Context, addressID string, activeOnly bool) (domainfood.FoodOrderHistory, error) {
	var data mcpFoodOrdersData
	if err := c.callTool(ctx, "get_food_orders", map[string]any{
		"addressId":  addressID,
		"activeOnly": activeOnly,
	}, &data); err != nil {
		return domainfood.FoodOrderHistory{}, err
	}
	return mapFoodOrders(data), nil
}

func (c *MCPFoodClient) GetOrderDetails(ctx context.Context, orderID string) (domainfood.FoodOrderDetails, error) {
	var data mcpFoodOrderDetails
	if err := c.callTool(ctx, "get_food_order_details", map[string]any{
		"orderId": orderID,
	}, &data); err != nil {
		return domainfood.FoodOrderDetails{}, err
	}
	return mapFoodOrderDetails(data), nil
}

func (c *MCPFoodClient) TrackOrder(ctx context.Context, orderID string) (domainfood.FoodTrackingStatus, error) {
	var args map[string]any
	if orderID = strings.TrimSpace(orderID); orderID != "" {
		args = map[string]any{"orderId": orderID}
	}
	var data mcpFoodTrackingData
	if err := c.callTool(ctx, "track_food_order", args, &data); err != nil {
		return domainfood.FoodTrackingStatus{}, err
	}
	return mapFoodTracking(data), nil
}

func (c *MCPFoodClient) FlushCart(ctx context.Context) error {
	return c.callTool(ctx, "flush_food_cart", nil, nil)
}
