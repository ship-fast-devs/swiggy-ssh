package instamart

import "context"

type Provider interface {
	GetAddresses(ctx context.Context) ([]Address, error)
	SearchProducts(ctx context.Context, addressID, query string, offset int) (ProductSearchResult, error)
	YourGoToItems(ctx context.Context, addressID string, offset int) (ProductSearchResult, error)
	GetCart(ctx context.Context) (Cart, error)
	UpdateCart(ctx context.Context, selectedAddressID string, items []CartUpdateItem) (Cart, error)
	ClearCart(ctx context.Context) error
	Checkout(ctx context.Context, addressID, paymentMethod string) (CheckoutResult, error)
	GetOrders(ctx context.Context, input OrderHistoryQuery) (OrderHistory, error)
	TrackOrder(ctx context.Context, orderID string, location Location) (TrackingStatus, error)
}
