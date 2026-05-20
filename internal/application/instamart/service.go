package instamart

import (
	"context"
	"fmt"
	"strings"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

type Provider = domaininstamart.Provider

type Address = domaininstamart.Address
type Cart = domaininstamart.Cart
type CartReviewSnapshot = domaininstamart.CartReviewSnapshot
type CartUpdateItem = domaininstamart.CartUpdateItem
type CheckoutResult = domaininstamart.CheckoutResult
type Location = domaininstamart.Location
type OrderHistory = domaininstamart.OrderHistory
type ProductSearchResult = domaininstamart.ProductSearchResult
type TrackingStatus = domaininstamart.TrackingStatus

var ErrAddressRequired = domaininstamart.ErrAddressRequired
var ErrVariantRequired = domaininstamart.ErrVariantRequired
var ErrCartEmpty = domaininstamart.ErrCartEmpty
var ErrCheckoutRequiresReview = domaininstamart.ErrCheckoutRequiresReview
var ErrCheckoutRequiresConfirmation = domaininstamart.ErrCheckoutRequiresConfirmation
var ErrCheckoutAmountLimit = domaininstamart.ErrCheckoutAmountLimit
var ErrPaymentMethodUnavailable = domaininstamart.ErrPaymentMethodUnavailable
var ErrTrackingLocationUnavailable = domaininstamart.ErrTrackingLocationUnavailable
var ErrCancellationUnsupported = domaininstamart.ErrCancellationUnsupported

type SearchProductsInput struct {
	AddressID string
	Query     string
	Offset    int
}

type GetGoToItemsInput struct {
	AddressID string
	Offset    int
}

type UpdateCartInput struct {
	SelectedAddressID string
	Items             []domaininstamart.CartUpdateItem
}

type CheckoutInput struct {
	AddressID     string
	PaymentMethod string
	Confirmed     bool
	ReviewedCart  *domaininstamart.CartReviewSnapshot
}

type GetOrdersInput struct {
	Count      int
	OrderType  string
	ActiveOnly bool
}

type TrackOrderInput struct {
	OrderID  string
	Location *domaininstamart.Location
}

type Service struct {
	provider domaininstamart.Provider
}

func NewService(provider domaininstamart.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) GetAddresses(ctx context.Context) ([]domaininstamart.Address, error) {
	addresses, err := s.provider.GetAddresses(ctx)
	if err != nil {
		return nil, fmt.Errorf("get instamart addresses: %w", err)
	}
	return addresses, nil
}

func (s *Service) SearchProducts(ctx context.Context, input SearchProductsInput) (domaininstamart.ProductSearchResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domaininstamart.ProductSearchResult{}, domaininstamart.ErrAddressRequired
	}

	result, err := s.provider.SearchProducts(ctx, addressID, input.Query, input.Offset)
	if err != nil {
		return domaininstamart.ProductSearchResult{}, fmt.Errorf("search instamart products: %w", err)
	}
	return result, nil
}

func (s *Service) GetGoToItems(ctx context.Context, input GetGoToItemsInput) (domaininstamart.ProductSearchResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domaininstamart.ProductSearchResult{}, domaininstamart.ErrAddressRequired
	}

	result, err := s.provider.YourGoToItems(ctx, addressID, input.Offset)
	if err != nil {
		return domaininstamart.ProductSearchResult{}, fmt.Errorf("get instamart go-to items: %w", err)
	}
	return result, nil
}

func (s *Service) GetCart(ctx context.Context) (domaininstamart.Cart, error) {
	cart, err := s.provider.GetCart(ctx)
	if err != nil {
		return domaininstamart.Cart{}, fmt.Errorf("get instamart cart: %w", err)
	}
	return cart, nil
}

func (s *Service) UpdateCart(ctx context.Context, input UpdateCartInput) (domaininstamart.Cart, error) {
	addressID := strings.TrimSpace(input.SelectedAddressID)
	if addressID == "" {
		return domaininstamart.Cart{}, domaininstamart.ErrAddressRequired
	}
	for _, item := range input.Items {
		if strings.TrimSpace(item.SpinID) == "" {
			return domaininstamart.Cart{}, domaininstamart.ErrVariantRequired
		}
	}

	cart, err := s.provider.UpdateCart(ctx, addressID, input.Items)
	if err != nil {
		return domaininstamart.Cart{}, fmt.Errorf("update instamart cart: %w", err)
	}
	return cart, nil
}

func (s *Service) ClearCart(ctx context.Context) error {
	if err := s.provider.ClearCart(ctx); err != nil {
		return fmt.Errorf("clear instamart cart: %w", err)
	}
	return nil
}

func (s *Service) Checkout(ctx context.Context, input CheckoutInput) (domaininstamart.CheckoutResult, error) {
	addressID := strings.TrimSpace(input.AddressID)
	if addressID == "" {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrAddressRequired
	}
	if input.ReviewedCart == nil {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrCheckoutRequiresReview
	}
	if len(input.ReviewedCart.Items) == 0 {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrCartEmpty
	}
	if !input.Confirmed {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrCheckoutRequiresConfirmation
	}

	freshCart, err := s.provider.GetCart(ctx)
	if err != nil {
		return domaininstamart.CheckoutResult{}, fmt.Errorf("get fresh instamart cart before checkout: %w", err)
	}
	if len(freshCart.Items) == 0 {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrCartEmpty
	}
	if !reviewedCartStillMatches(*input.ReviewedCart, freshCart, addressID, input.PaymentMethod) {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrCheckoutRequiresReview
	}
	if !paymentMethodAvailable(freshCart.AvailablePaymentMethods, input.PaymentMethod) {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrPaymentMethodUnavailable
	}
	if cartToPayRupees(freshCart) >= 1000 {
		return domaininstamart.CheckoutResult{}, domaininstamart.ErrCheckoutAmountLimit
	}

	result, err := s.provider.Checkout(ctx, addressID, strings.TrimSpace(input.PaymentMethod))
	if err != nil {
		return domaininstamart.CheckoutResult{}, fmt.Errorf("checkout instamart cart: %w", err)
	}
	if len(freshCart.StoreIDs) > 1 {
		result.MultiStore = true
	}
	return result, nil
}

func (s *Service) GetOrders(ctx context.Context, input GetOrdersInput) (domaininstamart.OrderHistory, error) {
	orderType := strings.TrimSpace(input.OrderType)
	if orderType == "" {
		orderType = domaininstamart.DefaultOrderType
	}

	history, err := s.provider.GetOrders(ctx, domaininstamart.OrderHistoryQuery{
		Count:      input.Count,
		OrderType:  orderType,
		ActiveOnly: input.ActiveOnly,
	})
	if err != nil {
		return domaininstamart.OrderHistory{}, fmt.Errorf("get instamart orders: %w", err)
	}
	return history, nil
}

func (s *Service) TrackOrder(ctx context.Context, input TrackOrderInput) (domaininstamart.TrackingStatus, error) {
	if !validLocation(input.Location) {
		return domaininstamart.TrackingStatus{}, domaininstamart.ErrTrackingLocationUnavailable
	}

	status, err := s.provider.TrackOrder(ctx, input.OrderID, *input.Location)
	if err != nil {
		return domaininstamart.TrackingStatus{}, fmt.Errorf("track instamart order: %w", err)
	}
	return status, nil
}

func (s *Service) HandleCancellation(ctx context.Context) error {
	return domaininstamart.ErrCancellationUnsupported
}

func reviewedCartStillMatches(review domaininstamart.CartReviewSnapshot, cart domaininstamart.Cart, checkoutAddressID, paymentMethod string) bool {
	reviewAddressID := strings.TrimSpace(review.AddressID)
	if reviewAddressID != strings.TrimSpace(checkoutAddressID) {
		return false
	}
	freshAddressID := strings.TrimSpace(cart.AddressID)
	if freshAddressID != "" && reviewAddressID != freshAddressID {
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

func sameItems(reviewItems []domaininstamart.CartUpdateItem, cartItems []domaininstamart.CartItem) bool {
	reviewed := make(map[string]int, len(reviewItems))
	for _, item := range reviewItems {
		spinID := strings.TrimSpace(item.SpinID)
		if spinID == "" {
			return false
		}
		reviewed[spinID] += item.Quantity
	}

	fresh := make(map[string]int, len(cartItems))
	for _, item := range cartItems {
		spinID := strings.TrimSpace(item.SpinID)
		if spinID == "" {
			return false
		}
		fresh[spinID] += item.Quantity
	}

	if len(reviewed) != len(fresh) {
		return false
	}
	for spinID, quantity := range reviewed {
		if fresh[spinID] != quantity {
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

func cartToPayRupees(cart domaininstamart.Cart) int {
	if cart.Bill.ToPayRupees > 0 {
		return cart.Bill.ToPayRupees
	}
	return cart.TotalRupees
}

func validLocation(location *domaininstamart.Location) bool {
	if location == nil {
		return false
	}
	if location.Lat == 0 && location.Lng == 0 {
		return false
	}
	return location.Lat >= -90 && location.Lat <= 90 && location.Lng >= -180 && location.Lng <= 180
}
