package instamart

import "errors"

var ErrAddressRequired = errors.New("instamart: address required")
var ErrVariantRequired = errors.New("instamart: product variation required")
var ErrCartEmpty = errors.New("instamart: cart is empty")
var ErrCheckoutRequiresReview = errors.New("instamart: checkout requires fresh cart review")
var ErrCheckoutRequiresConfirmation = errors.New("instamart: checkout requires explicit confirmation")
var ErrCheckoutAmountLimit = errors.New("instamart: checkout amount limit exceeded")
var ErrPaymentMethodUnavailable = errors.New("instamart: payment method unavailable")
var ErrTrackingLocationUnavailable = errors.New("instamart: tracking location unavailable")
var ErrCancellationUnsupported = errors.New("instamart: cancellation unsupported")
