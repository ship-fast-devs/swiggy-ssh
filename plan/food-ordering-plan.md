# Food Ordering Plan

## Goal

Build Swiggy Food ordering inside `swiggy-ssh`.

Do it like Instamart. Same Clean Architecture shape. Same TUI discipline. No clever rewrite.

## Session Address Assumption

Food does not own address selection.

Food receives a selected session address from the root Home/session flow. See `plan/home.md` for auth state, auto-selecting the first saved address, and manual address switching.

Food rules:

- Use the selected session address ID for Food provider calls.
- Do not guess or invent address IDs.
- Do not start Food with an address picker.
- If selected address is missing, show `address required` and send user back to Home/address switch.

## Food Domain

Add `internal/domain/food`.

Types:

- `Restaurant`
- `RestaurantAvailability`
- `MenuCategory`
- `MenuItem`
- `MenuVariant`
- `MenuAddon`
- `FoodCart`
- `FoodCartItem`
- `FoodBill`
- `FoodCoupon`
- `FoodOrder`
- `FoodTrackingStatus`

Errors:

- `ErrAddressRequired`
- `ErrRestaurantUnavailable`
- `ErrCartEmpty`
- `ErrCartAmountLimit`
- `ErrCheckoutRequiresReview`
- `ErrCheckoutRequiresConfirmation`
- `ErrPaymentMethodUnavailable`
- `ErrCancellationUnsupported`

Ports:

- One `Provider` interface is fine at first.
- Split later only if it gets ugly.

Provider methods:

- `SearchRestaurants`
- `GetRestaurantMenu`
- `SearchMenu`
- `UpdateCart`
- `GetCart`
- `FetchCoupons`
- `ApplyCoupon`
- `PlaceOrder`
- `GetOrders`
- `GetOrderDetails`
- `TrackOrder`
- `FlushCart`

Domain imports stdlib only.

## Food Application

Add `internal/application/food`.

Use cases or one service:

- `SearchRestaurants`
- `GetRestaurantMenu`
- `SearchMenu`
- `UpdateCart`
- `GetCart`
- `FetchCoupons`
- `ApplyCoupon`
- `PlaceOrder`
- `GetOrders`
- `GetOrderDetails`
- `TrackOrder`
- `FlushCart`

Keep it boring.

Application rules:

- Address ID required for restaurant/menu/cart/order calls.
- Restaurant must be `OPEN` before ordering from it.
- After `UpdateCart`, caller must refresh with `GetCart`.
- Checkout requires fresh reviewed cart.
- Checkout requires explicit confirmation.
- Cart total must be below Rs 1000.
- Compare the final payable total, not item subtotal.
- Payment method must come from `GetCart` available payment methods.
- Cancellation does not call provider.
- Cancellation returns `To cancel your order, please call Swiggy customer care at 080-67466729.`
- Successful order message is shown as provider returned it.

## Food MCP Adapter

Add Food MCP adapter under infrastructure.

Likely files:

- `internal/infrastructure/provider/swiggy/food_client.go`
- `internal/infrastructure/provider/swiggy/food_mapping.go`
- `internal/infrastructure/provider/swiggy/food_client_test.go`

Endpoint:

- POST to `/food` endpoint.
- Tool names are unqualified.
- Use `search_restaurants`, not `food.search_restaurants`.

Tools:

- `get_addresses`
- `search_restaurants`
- `get_restaurant_menu`
- `search_menu`
- `update_food_cart`
- `get_food_cart`
- `fetch_food_coupons`
- `apply_food_coupon`
- `place_food_order`
- `get_food_orders`
- `get_food_order_details`
- `track_food_order`
- `flush_food_cart`

Mapping rules:

- Keep MCP JSON in infrastructure.
- Application gets clean domain structs.
- Do not leak raw MCP blobs into TUI.
- Do not log tokens.
- Do not log full address or phone.

## Food TUI

Add a Food flow similar to `instamartflow`.

Likely package:

- `internal/presentation/tui/foodflow`

Screens:

- Food home
- Restaurant search input
- Restaurant results
- Restaurant menu browse
- Menu item search
- Item customization
- Cart review
- Coupon list/apply
- Checkout confirm
- Order result
- Order history
- Tracking
- Help

Food home actions:

- search restaurants
- search dish
- staged cart
- coupons
- tail active order
- order history
- cancel help
- back home

Food should receive selected session address from SSH/root state.

Food should not start with address selection.

If selected address is missing:

- Show `address required`.
- Tell user to pick address from Home.

## Menu And Cart Rules

Restaurant rules:

- Show availability.
- Let user browse closed restaurants only if useful, but do not order from them.
- Ordering requires `OPEN`.

Menu rules:

- Use `get_restaurant_menu` to browse categories.
- Use `search_menu` before adding exact item.
- Preserve returned variant shape.
- If item has `variants`, send `variants`.
- If item has `variantsV2`, send `variantsV2`.
- Never send both for one item.

Addon rules:

- Add item with variants first.
- Call `get_food_cart`.
- Read `valid_addons`.
- Add only valid addons.
- Call `get_food_cart` again.

Cart rules:

- Show real cart after every mutation.
- Show selected address.
- Show bill lines.
- Show payment methods returned by cart.
- Do not invent COD.
- Do not say coupon applied unless discount is greater than zero.
- Recommend coupons only when response metadata clearly matches the selected payment flow.
- If COD/Cash compatibility is unclear, show coupon as informational only.

Checkout rules:

- Show final items.
- Show final bill.
- Use final payable total for the Rs 1000 block.
- Show final delivery address.
- Show selected payment method.
- Ask explicit confirmation.
- Then call place order.
- Block Rs 1000 or more.

## Main Wiring

Only `cmd/swiggy-ssh/main.go` wires concrete things.

Wire:

- Food service
- Swiggy Food MCP client
- SSH server dependencies

Presentation must not import infrastructure.

Application must not import infrastructure.

## Tests

Application tests:

- Food search requires address.
- Restaurant ordering blocks when not `OPEN`.
- Cart update requires address.
- Checkout requires reviewed cart.
- Checkout requires confirmation.
- Checkout blocks Rs 1000 or more.
- Checkout rejects unavailable payment method.
- Cancellation returns customer care message and does not call provider.

TUI tests:

- Food starts without address picker when selected address exists.
- Food blocks with `address required` when selected address missing.
- Food checkout requires explicit confirmation.

Infrastructure tests:

- Food client calls `/food` tools by unqualified names.
- Search restaurants sends `addressId`, `query`, `offset`.
- Cart sends only returned variant format.
- Cart mapping includes available payment methods.
- Coupons with zero discount are not real savings.

Boundary checks:

- Presentation does not import infrastructure.
- Application does not import infrastructure.
- Domain imports stdlib only.

Final check:

- `go test ./...`

## Build Order

1. Add Food domain.
2. Add Food application service.
3. Add mock Food provider.
4. Add Food MCP adapter.
5. Add Food TUI shell that accepts selected session address.
6. Add restaurant search.
7. Add menu browse/search.
8. Add item customization.
9. Add cart review.
10. Add coupons.
11. Add checkout confirm.
12. Add orders/tracking.
13. Wire Food in `main.go` and SSH routing.
14. Run tests.
15. Manual SSH smoke test.

## Done Means

- Food uses selected address.
- Food can search restaurants.
- Food can browse/search menu.
- Food can add item to cart.
- Food can review cart.
- Food can place order only after explicit confirmation.
- Food blocks big carts.
- Tests pass.
