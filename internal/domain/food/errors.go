package food

import "errors"

var ErrAddressRequired = errors.New("food: address required")
var ErrRestaurantRequired = errors.New("food: restaurant required")
var ErrRestaurantUnavailable = errors.New("food: restaurant is not OPEN")
var ErrCartEmpty = errors.New("food: cart is empty")
var ErrCartAmountLimit = errors.New("food: cart total must be below Rs 1000")
var ErrCheckoutRequiresReview = errors.New("food: checkout requires fresh cart review")
var ErrCheckoutRequiresConfirmation = errors.New("food: checkout requires explicit confirmation")
var ErrPaymentMethodUnavailable = errors.New("food: payment method unavailable")
var ErrCancellationUnsupported = errors.New("food: cancellation unsupported")
