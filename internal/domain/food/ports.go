package food

import "context"

type Provider interface {
	GetAddresses(ctx context.Context) ([]Address, error)
	SearchRestaurants(ctx context.Context, addressID, query string, offset int) (RestaurantSearchResult, error)
	GetRestaurantMenu(ctx context.Context, addressID, restaurantID string, page, pageSize int) (MenuPage, error)
	SearchMenu(ctx context.Context, addressID, query, restaurantID string, vegFilter, offset int) (MenuSearchResult, error)
	UpdateCart(ctx context.Context, restaurantID, restaurantName, addressID string, items []FoodCartUpdateItem) (FoodCart, error)
	GetCart(ctx context.Context, addressID, restaurantName string) (FoodCart, error)
	FetchCoupons(ctx context.Context, restaurantID, addressID string) (FoodCouponsResult, error)
	ApplyCoupon(ctx context.Context, couponCode, addressID string) error
	PlaceOrder(ctx context.Context, addressID, paymentMethod string) (FoodOrderResult, error)
	GetOrders(ctx context.Context, addressID string, activeOnly bool) (FoodOrderHistory, error)
	GetOrderDetails(ctx context.Context, orderID string) (FoodOrderDetails, error)
	TrackOrder(ctx context.Context, orderID string) (FoodTrackingStatus, error)
	FlushCart(ctx context.Context) error
}
