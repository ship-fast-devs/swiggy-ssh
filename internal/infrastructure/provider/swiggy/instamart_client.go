package swiggy

import (
	"context"
	"encoding/json"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

var _ domaininstamart.Provider = (*MCPInstamartClient)(nil)

func (c *MCPInstamartClient) GetAddresses(ctx context.Context) ([]domaininstamart.Address, error) {
	var data mcpAddressesData
	if err := c.callTool(ctx, "get_addresses", nil, &data); err != nil {
		return nil, err
	}
	return mapAddresses(data), nil
}

func (c *MCPInstamartClient) SearchProducts(ctx context.Context, addressID, query string, offset int) (domaininstamart.ProductSearchResult, error) {
	return c.searchProducts(ctx, "search_products", map[string]any{
		"addressId": addressID,
		"query":     query,
		"offset":    offset,
	})
}

func (c *MCPInstamartClient) YourGoToItems(ctx context.Context, addressID string, offset int) (domaininstamart.ProductSearchResult, error) {
	return c.searchProducts(ctx, "your_go_to_items", map[string]any{
		"addressId": addressID,
		"offset":    offset,
	})
}

func (c *MCPInstamartClient) searchProducts(ctx context.Context, toolName string, arguments map[string]any) (domaininstamart.ProductSearchResult, error) {
	var data mcpProductSearchData
	if err := c.callTool(ctx, toolName, arguments, &data); err != nil {
		return domaininstamart.ProductSearchResult{}, err
	}
	return mapProductSearch(data), nil
}

func (c *MCPInstamartClient) GetCart(ctx context.Context) (domaininstamart.Cart, error) {
	var data mcpCartData
	if err := c.callTool(ctx, "get_cart", nil, &data); err != nil {
		return domaininstamart.Cart{}, err
	}
	return mapCart(data), nil
}

func (c *MCPInstamartClient) UpdateCart(ctx context.Context, selectedAddressID string, items []domaininstamart.CartUpdateItem) (domaininstamart.Cart, error) {
	requestItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		requestItems = append(requestItems, map[string]any{"spinId": item.SpinID, "quantity": item.Quantity})
	}
	var data mcpCartData
	if err := c.callTool(ctx, "update_cart", map[string]any{
		"selectedAddressId": selectedAddressID,
		"items":             requestItems,
	}, &data); err != nil {
		return domaininstamart.Cart{}, err
	}
	return mapCart(data), nil
}

func (c *MCPInstamartClient) ClearCart(ctx context.Context) error {
	return c.callTool(ctx, "clear_cart", nil, nil)
}

func (c *MCPInstamartClient) Checkout(ctx context.Context, addressID, paymentMethod string) (domaininstamart.CheckoutResult, error) {
	envelope, err := c.callToolEnvelope(ctx, "checkout", map[string]any{
		"addressId":     addressID,
		"paymentMethod": paymentMethod,
	})
	if err != nil {
		return domaininstamart.CheckoutResult{}, err
	}
	var data mcpCheckoutData
	if len(envelope.Data) != 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return domaininstamart.CheckoutResult{}, err
		}
	}
	return mapCheckout(data, envelope.Message), nil
}

func (c *MCPInstamartClient) GetOrders(ctx context.Context, input domaininstamart.OrderHistoryQuery) (domaininstamart.OrderHistory, error) {
	orderType := input.OrderType
	if orderType == "" {
		orderType = domaininstamart.DefaultOrderType
	}
	var data mcpOrdersData
	if err := c.callTool(ctx, "get_orders", map[string]any{
		"count":      input.Count,
		"orderType":  orderType,
		"activeOnly": input.ActiveOnly,
	}, &data); err != nil {
		return domaininstamart.OrderHistory{}, err
	}
	return mapOrders(data), nil
}

func (c *MCPInstamartClient) TrackOrder(ctx context.Context, orderID string, location domaininstamart.Location) (domaininstamart.TrackingStatus, error) {
	var data mcpTrackingData
	if err := c.callTool(ctx, "track_order", map[string]any{
		"orderId": orderID,
		"lat":     location.Lat,
		"lng":     location.Lng,
	}, &data); err != nil {
		return domaininstamart.TrackingStatus{}, err
	}
	return mapTracking(data), nil
}

func (c *MCPInstamartClient) callToolEnvelope(ctx context.Context, name string, arguments any) (instamartToolEnvelope, error) {
	return c.callToolEnvelopeHTTP(ctx, name, arguments)
}
