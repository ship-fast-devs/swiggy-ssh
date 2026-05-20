package food

import (
	"context"
	"fmt"
	"strings"

	domainfood "swiggy-ssh/internal/domain/food"
)

// Type aliases so the TUI can import from appfood cleanly.
type Provider = domainfood.Provider

type Address = domainfood.Address
type Restaurant = domainfood.Restaurant
type RestaurantSearchResult = domainfood.RestaurantSearchResult
type MenuPage = domainfood.MenuPage
type MenuCategory = domainfood.MenuCategory
type MenuItem = domainfood.MenuItem
type MenuItemDetail = domainfood.MenuItemDetail
type MenuSearchResult = domainfood.MenuSearchResult
type FoodCart = domainfood.FoodCart
type FoodCartItem = domainfood.FoodCartItem
type FoodCartUpdateItem = domainfood.FoodCartUpdateItem
type FoodCartReviewSnapshot = domainfood.FoodCartReviewSnapshot
type FoodCoupon = domainfood.FoodCoupon
type FoodCouponsResult = domainfood.FoodCouponsResult
type FoodOrderResult = domainfood.FoodOrderResult
type FoodOrderSummary = domainfood.FoodOrderSummary
type FoodOrderHistory = domainfood.FoodOrderHistory
type FoodOrderDetails = domainfood.FoodOrderDetails
type FoodTrackingStatus = domainfood.FoodTrackingStatus
type CartVariant = domainfood.CartVariant
type CartVariantV2 = domainfood.CartVariantV2
type CartAddon = domainfood.CartAddon

// Error aliases.
var ErrAddressRequired = domainfood.ErrAddressRequired
var ErrRestaurantRequired = domainfood.ErrRestaurantRequired
var ErrRestaurantUnavailable = domainfood.ErrRestaurantUnavailable
var ErrCartEmpty = domainfood.ErrCartEmpty
var ErrCartAmountLimit = domainfood.ErrCartAmountLimit
var ErrCheckoutRequiresReview = domainfood.ErrCheckoutRequiresReview
var ErrCheckoutRequiresConfirmation = domainfood.ErrCheckoutRequiresConfirmation
var ErrPaymentMethodUnavailable = domainfood.ErrPaymentMethodUnavailable
var ErrCancellationUnsupported = domainfood.ErrCancellationUnsupported

// Input types.

func (s *Service) GetAddresses(ctx context.Context) ([]domainfood.Address, error) {
	addresses, err := s.provider.GetAddresses(ctx)
	if err != nil {
		return nil, fmt.Errorf("get food addresses: %w", err)
	}
	return addresses, nil
}

type SearchRestaurantsInput struct {
	AddressID string
	Query     string
	Offset    int
}

type GetMenuInput struct {
	AddressID    string
	RestaurantID string
	Page         int
	PageSize     int
}

type SearchMenuInput struct {
	AddressID    string
	Query        string
	RestaurantID string
	VegFilter    int
	Offset       int
}

type UpdateCartInput struct {
	RestaurantID   string
	RestaurantName string
	AddressID      string
	Items          []domainfood.FoodCartUpdateItem
}

type GetCartInput struct {
	AddressID      string
	RestaurantName string
}

type FetchCouponsInput struct {
	RestaurantID string
	AddressID    string
}

type ApplyCouponInput struct {
	CouponCode string
	AddressID  string
}

type PlaceOrderInput struct {
	AddressID      string
	RestaurantName string
	PaymentMethod  string
	Confirmed      bool
	ReviewedCart   *domainfood.FoodCartReviewSnapshot
}

type GetOrdersInput struct {
	AddressID  string
	ActiveOnly bool
}

type GetOrderDetailsInput struct {
	OrderID string
}

type TrackOrderInput struct {
	OrderID string
}

// Service holds the food provider and implements all use cases.
type Service struct {
	provider domainfood.Provider
}

func NewService(provider domainfood.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) SearchRestaurants(ctx context.Context, input SearchRestaurantsInput) (domainfood.RestaurantSearchResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.RestaurantSearchResult{}, domainfood.ErrAddressRequired
	}

	result, err := s.provider.SearchRestaurants(ctx, addressID, input.Query, input.Offset)
	if err != nil {
		return domainfood.RestaurantSearchResult{}, fmt.Errorf("search food restaurants: %w", err)
	}
	return result, nil
}

func (s *Service) GetRestaurantMenu(ctx context.Context, input GetMenuInput) (domainfood.MenuPage, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.MenuPage{}, domainfood.ErrAddressRequired
	}
	restaurantID := strings.TrimSpace(input.RestaurantID)
	if restaurantID == "" {
		return domainfood.MenuPage{}, domainfood.ErrRestaurantRequired
	}

	page, err := s.provider.GetRestaurantMenu(ctx, addressID, restaurantID, input.Page, input.PageSize)
	if err != nil {
		return domainfood.MenuPage{}, fmt.Errorf("get food restaurant menu: %w", err)
	}
	return page, nil
}

func (s *Service) SearchMenu(ctx context.Context, input SearchMenuInput) (domainfood.MenuSearchResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.MenuSearchResult{}, domainfood.ErrAddressRequired
	}

	result, err := s.provider.SearchMenu(ctx, addressID, input.Query, input.RestaurantID, input.VegFilter, input.Offset)
	if err != nil {
		return domainfood.MenuSearchResult{}, fmt.Errorf("search food menu: %w", err)
	}
	return result, nil
}

func (s *Service) UpdateCart(ctx context.Context, input UpdateCartInput) (domainfood.FoodCart, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.FoodCart{}, domainfood.ErrAddressRequired
	}
	restaurantID := strings.TrimSpace(input.RestaurantID)
	if restaurantID == "" {
		return domainfood.FoodCart{}, domainfood.ErrRestaurantRequired
	}

	cart, err := s.provider.UpdateCart(ctx, restaurantID, input.RestaurantName, addressID, input.Items)
	if err != nil {
		return domainfood.FoodCart{}, fmt.Errorf("update food cart: %w", err)
	}
	return cart, nil
}

func (s *Service) GetCart(ctx context.Context, input GetCartInput) (domainfood.FoodCart, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.FoodCart{}, domainfood.ErrAddressRequired
	}

	cart, err := s.provider.GetCart(ctx, addressID, strings.TrimSpace(input.RestaurantName))
	if err != nil {
		return domainfood.FoodCart{}, fmt.Errorf("get food cart: %w", err)
	}
	return cart, nil
}

func (s *Service) FetchCoupons(ctx context.Context, input FetchCouponsInput) (domainfood.FoodCouponsResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.FoodCouponsResult{}, domainfood.ErrAddressRequired
	}
	restaurantID := strings.TrimSpace(input.RestaurantID)
	if restaurantID == "" {
		return domainfood.FoodCouponsResult{}, domainfood.ErrRestaurantRequired
	}

	result, err := s.provider.FetchCoupons(ctx, restaurantID, addressID)
	if err != nil {
		return domainfood.FoodCouponsResult{}, fmt.Errorf("fetch food coupons: %w", err)
	}
	return result, nil
}

func (s *Service) ApplyCoupon(ctx context.Context, input ApplyCouponInput) error {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.ErrAddressRequired
	}

	if err := s.provider.ApplyCoupon(ctx, strings.TrimSpace(input.CouponCode), addressID); err != nil {
		return fmt.Errorf("apply food coupon: %w", err)
	}
	return nil
}

func (s *Service) PlaceOrder(ctx context.Context, input PlaceOrderInput) (domainfood.FoodOrderResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.FoodOrderResult{}, domainfood.ErrAddressRequired
	}
	if input.ReviewedCart == nil {
		return domainfood.FoodOrderResult{}, domainfood.ErrCheckoutRequiresReview
	}
	if len(input.ReviewedCart.Items) == 0 {
		return domainfood.FoodOrderResult{}, domainfood.ErrCartEmpty
	}
	if !input.Confirmed {
		return domainfood.FoodOrderResult{}, domainfood.ErrCheckoutRequiresConfirmation
	}

	freshCart, err := s.provider.GetCart(ctx, addressID, strings.TrimSpace(input.RestaurantName))
	if err != nil {
		return domainfood.FoodOrderResult{}, fmt.Errorf("get fresh food cart before order: %w", err)
	}
	if len(freshCart.Items) == 0 {
		return domainfood.FoodOrderResult{}, domainfood.ErrCartEmpty
	}
	if !reviewedCartStillMatches(*input.ReviewedCart, freshCart, addressID, input.PaymentMethod) {
		return domainfood.FoodOrderResult{}, domainfood.ErrCheckoutRequiresReview
	}
	if !paymentMethodAvailable(freshCart.AvailablePaymentMethods, input.PaymentMethod) {
		return domainfood.FoodOrderResult{}, domainfood.ErrPaymentMethodUnavailable
	}
	if cartToPayRupees(freshCart) >= 1000 {
		return domainfood.FoodOrderResult{}, domainfood.ErrCartAmountLimit
	}

	result, err := s.provider.PlaceOrder(ctx, addressID, strings.TrimSpace(input.PaymentMethod))
	if err != nil {
		return domainfood.FoodOrderResult{}, fmt.Errorf("place food order: %w", err)
	}
	return result, nil
}

func (s *Service) GetOrders(ctx context.Context, input GetOrdersInput) (domainfood.FoodOrderHistory, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domainfood.FoodOrderHistory{}, domainfood.ErrAddressRequired
	}

	history, err := s.provider.GetOrders(ctx, addressID, input.ActiveOnly)
	if err != nil {
		return domainfood.FoodOrderHistory{}, fmt.Errorf("get food orders: %w", err)
	}
	return history, nil
}

func (s *Service) GetOrderDetails(ctx context.Context, input GetOrderDetailsInput) (domainfood.FoodOrderDetails, error) {
	orderID := strings.TrimSpace(input.OrderID)
	if orderID == "" {
		return domainfood.FoodOrderDetails{}, fmt.Errorf("food: order ID required")
	}

	details, err := s.provider.GetOrderDetails(ctx, orderID)
	if err != nil {
		return domainfood.FoodOrderDetails{}, fmt.Errorf("get food order details: %w", err)
	}
	return details, nil
}

func (s *Service) TrackOrder(ctx context.Context, input TrackOrderInput) (domainfood.FoodTrackingStatus, error) {
	status, err := s.provider.TrackOrder(ctx, strings.TrimSpace(input.OrderID))
	if err != nil {
		return domainfood.FoodTrackingStatus{}, fmt.Errorf("track food order: %w", err)
	}
	return status, nil
}

func (s *Service) FlushCart(ctx context.Context) error {
	if err := s.provider.FlushCart(ctx); err != nil {
		return fmt.Errorf("flush food cart: %w", err)
	}
	return nil
}

func (s *Service) HandleCancellation(ctx context.Context) error {
	return domainfood.ErrCancellationUnsupported
}

// reviewedCartStillMatches verifies the snapshot matches the current live cart state.
func reviewedCartStillMatches(review domainfood.FoodCartReviewSnapshot, cart domainfood.FoodCart, checkoutAddressID, paymentMethod string) bool {
	reviewAddressID := strings.TrimSpace(review.AddressID)
	if reviewAddressID != strings.TrimSpace(checkoutAddressID) {
		return false
	}
	freshAddressID := strings.TrimSpace(cart.AddressID)
	if freshAddressID != "" && reviewAddressID != freshAddressID {
		return false
	}
	reviewRestaurantID := strings.TrimSpace(review.RestaurantID)
	freshRestaurantID := strings.TrimSpace(cart.RestaurantID)
	if reviewRestaurantID != "" && freshRestaurantID != "" && reviewRestaurantID != freshRestaurantID {
		return false
	}
	if strings.TrimSpace(review.PaymentMethod) != strings.TrimSpace(paymentMethod) {
		return false
	}
	if review.ToPayRupees != cartToPayRupees(cart) {
		return false
	}
	return sameItems(review.Items, cart.Items)
}

func sameItems(reviewItems []domainfood.FoodCartUpdateItem, cartItems []domainfood.FoodCartItem) bool {
	reviewed := make(map[string]int, len(reviewItems))
	for _, item := range reviewItems {
		id := strings.TrimSpace(item.MenuItemID)
		if id == "" {
			return false
		}
		reviewed[id] += item.Quantity
	}

	fresh := make(map[string]int, len(cartItems))
	for _, item := range cartItems {
		id := strings.TrimSpace(item.MenuItemID)
		if id == "" {
			return false
		}
		fresh[id] += item.Quantity
	}

	if len(reviewed) != len(fresh) {
		return false
	}
	for id, quantity := range reviewed {
		if fresh[id] != quantity {
			return false
		}
	}
	return true
}

func paymentMethodAvailable(available []string, selected string) bool {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return false
	}
	for _, method := range available {
		if strings.TrimSpace(method) == selected {
			return true
		}
	}
	return false
}

func cartToPayRupees(cart domainfood.FoodCart) int {
	if cart.Bill.ToPayRupees > 0 {
		return cart.Bill.ToPayRupees
	}
	return cart.TotalRupees
}
