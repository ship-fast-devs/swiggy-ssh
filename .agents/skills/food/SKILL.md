---
name: food
description: Use when working with Swiggy MCP /food tools, food delivery restaurant search, menus, food cart, coupons, order placement, history, or tracking flows.
---

# Swiggy MCP `/food` Tool Usage

Generated and verified on 2026-05-16 against `https://mcp.swiggy.com/food`.

This file covers Swiggy Food delivery only. Tool names are unqualified in MCP requests: POST to `/food`, then call tool name `search_restaurants`, not `food.search_restaurants`.

## MCP Assumption

Connection and session setup is out of scope for this skill. Assume the caller already has a working MCP client wired to:

```text
POST https://mcp.swiggy.com/food
```

This skill only documents tool names, arguments, workflows, response shapes, and safety gates.

## swiggy-ssh Session Address Model

In `swiggy-ssh`, saved address selection is session-level, not Food-specific.

Home behavior:

- If the user is unauthenticated, Home must show `auth required` instead of generic connected SSH state.
- After auth succeeds, fetch saved addresses for the terminal session and cache them in session state.
- Auto-select the first returned saved address as the session delivery target.
- Home must show `deploying to <label>` for the selected delivery address.
- Food and Instamart must reuse the selected session address by default.
- Do not force address selection inside each vertical when a session address already exists.
- Manual address switching remains available from the app shell/session UI and may re-fetch addresses before showing the chooser.
- Never guess, invent, hard-code, or synthesize address IDs. The selected address ID must come from `get_addresses`.

The selected address should be carried as session state with at least:

```text
addressId
label/display text
full address details needed for checkout confirmation
```

Checkout still must display the final delivery address and require explicit confirmation before placing an order.

List tools:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
```

Call a tool:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "search_restaurants",
    "arguments": {
      "addressId": "<addressId>",
      "query": "pizza",
      "offset": 0
    }
  }
}
```

## Workflow Tree

1. Ensure the user is authenticated.
2. If unauthenticated, render Home as `auth required` and route the user through auth before Food actions.
3. After auth succeeds, call `get_addresses` for the terminal session and cache the returned addresses.
4. Auto-select the first returned address as the session delivery target. Never invent an `addressId`.
5. Render Home with `deploying to <label>`.
6. Food uses the session-selected `addressId` for restaurant search, menu search, cart, coupons, order history, and checkout.
7. `search_restaurants` with session `addressId`, `query`, optional `offset`.
8. Pick an `OPEN` restaurant and persist `restaurantId` and `restaurantName`.
9. Browse with `get_restaurant_menu` or search a dish with `search_menu`.
10. For customized items, choose variants/addons from `search_menu`.
11. `update_food_cart`.
12. Immediately call `get_food_cart`.
13. Optional: `fetch_food_coupons` then `apply_food_coupon` if the response explicitly shows a COD-compatible valid coupon.
14. Show full cart, available payment methods, bill total, and final delivery address.
15. Require explicit user confirmation.
16. `place_food_order`.
17. `track_food_order` and/or `get_food_orders`.

Manual address switching:

1. User explicitly opens address switcher.
2. Reuse cached addresses or call `get_addresses` again if the cache is missing/stale.
3. Let the user select one.
4. Update the session delivery target.
5. Reuse the updated session address for Food and Instamart.

Important: each food menu item uses either legacy `variants` or `variantsV2`. Use the same format returned by `search_menu`; do not mix both.

Addon flow for customized items:

1. Add item with variants only.
2. Call `get_food_cart`.
3. Read `valid_addons` for the chosen variant.
4. Add only valid addon choices and respect min/max constraints.
5. Call `get_food_cart` again immediately after the addon update so the caller has the real cart state.

## swiggy-ssh Architecture Placement

Keep Food ordering client-agnostic and follow the repo's Clean Architecture boundaries.

Recommended placement:

| Concern | Package |
|---|---|
| Food entities, errors, and ports | `internal/domain/food` |
| Food orchestration use cases | `internal/application/food` |
| Swiggy Food MCP/provider adapter | `internal/infrastructure/provider/swiggy` or a Food-specific provider subpackage |
| SSH session routing | `internal/presentation/ssh` |
| Bubbletea screens and input handling | `internal/presentation/tui` |
| Concrete wiring | `cmd/swiggy-ssh/main.go` |

Session address ownership:

- The authenticated terminal session owns the selected delivery address.
- Food and Instamart should receive the selected address from session/application state.
- Food use cases should accept an `addressId` from trusted session state, not ask the vertical UI to rediscover or invent one.
- Presentation may render address labels and collect manual switch input.
- Application code should enforce checkout sequencing: cart summary, final address, payment method, explicit confirmation, then order placement.

Boundary rules:

- `internal/presentation/*` must not import `internal/infrastructure/*`.
- `internal/application/*` must depend only on domain ports.
- The Swiggy MCP client is infrastructure.
- `cmd/swiggy-ssh/main.go` is the wiring point for concrete Food provider implementations.

## Tool Catalog

| Tool | Description | Required args | Optional args | How to use |
|---|---|---|---|---|
| `get_addresses` | Gets saved delivery addresses. | - | - | First call for Food delivery flows. |
| `search_restaurants` | Searches restaurants for delivery. | `addressId`, `query` | `offset` | Persist `restaurantId`, `restaurantName`, and availability. |
| `search_menu` | Searches menu items/dishes and returns customization ids. | `addressId`, `query` | `restaurantIdOfAddedItem`, `vegFilter`, `offset` | Use before adding specific dishes; returns variant/addon ids. |
| `get_restaurant_menu` | Browses restaurant menu by category page. | `addressId`, `restaurantId` | `page`, `pageSize` | Use for discovery and pagination; use `search_menu` before cart add. |
| `get_food_cart` | Gets current Food cart and payment methods. | `addressId` | `restaurantName` | Call after cart mutation and before `place_food_order`. |
| `update_food_cart` | Adds/updates cart items, variants, and addons. | `restaurantId`, `cartItems`, `addressId` | `restaurantName` | `cartItems[]` requires `menu_item_id`, `quantity`; variants/addons depend on `search_menu`. |
| `flush_food_cart` | Clears the Food cart. | - | - | Use for cleanup or explicit empty-cart action. |
| `place_food_order` | Places a real Food delivery order. | `addressId` | `paymentMethod` | Only after `get_food_cart`, address/payment/total display, and explicit confirmation. |
| `fetch_food_coupons` | Fetches coupons/offers for a restaurant/address. | `restaurantId`, `addressId` | `couponCode` | Use before checkout; only recommend coupons the response marks as compatible with the selected payment flow. |
| `apply_food_coupon` | Applies a coupon to Food cart. | `couponCode`, `addressId` | `cartId` | Use only with a valid coupon code. |
| `get_food_orders` | Gets Food order history or active orders. | `addressId` | `activeOnly` | Leave `activeOnly=false` for generic history. Set `activeOnly=true` only for active/current/ongoing orders. |
| `get_food_order_details` | Gets detailed Food order information. | `orderId` | - | Use with an order id from `get_food_orders`. |
| `track_food_order` | Tracks active Food delivery orders. | - | `orderId` | Use for active/in-progress Food order ETA/status. |
| `report_error` | Generates MCP team error report. | `tool`, `errorMessage` | `domain`, `flowDescription`, `toolContext`, `userNotes` | Include IDs like `orderId`, `restaurantId`, `couponCode`, `menu_item_id`. |

## Real Response Examples

### `get_addresses`

Use after auth to populate session address state. The response contains saved address IDs and display details, but no coordinates. In `swiggy-ssh`, auto-select the first returned address for the session and show `deploying to <label>` in Home. Only show the list when the user explicitly opens address switching.

Do not guess, invent, or hard-code `addressId`.

### `search_restaurants`

Call used:

```json
{
  "addressId": "<addressId>",
  "query": "pizza",
  "offset": 0
}
```

Observed response text excerpt:

```text
Found 10 restaurants for "pizza":
1. Olio - The Wood Fired Pizzeria (Ad) — Pizzas, Pastas, Italian, Fast Food, Snacks, Beverages, Desserts | 4.1★ | 43 min | ₹300 for two (ID: 603191)
2. La Pino'z Pizza (Ad) — Pizzas, Pastas, Italian, Desserts, Beverages | 4.1★ | 34 min | ₹250 for two (ID: 211192)
...
8. Domino's Pizza (Ad) — Pizzas, Italian, Pastas, Desserts | 4.5★ | 20 min | ₹400 for two (ID: 23788)
```

Persist restaurant id, name, ETA, distance if present, and availability status. Do not proceed with closed/unavailable restaurants.

### `get_restaurant_menu`

Call used:

```json
{
  "addressId": "<addressId>",
  "restaurantId": "23788",
  "page": 1,
  "pageSize": 3
}
```

Observed response text excerpt:

```text
Menu for Domino's Pizza (ID: 23788)
Page 1 — 3 of 29 categories. Use page=2 for more.

## Minimum 50% off
  - Onion Fresh Pan — ₹99 | Veg, has variants (ID: 192044804)
...
## Recommended
  - Chicken Dominator Pizza — ₹369 | Non-veg, has variants, has addons (ID: 25933457)
  - Veg Extravaganza Pizza — ₹319 | Veg, has variants, has addons (ID: 110240705)
```

Use `page`/`pageSize` for category pagination. To add an item, use `search_menu` for full customization details.

### `search_menu`

Call used:

```json
{
  "addressId": "<addressId>",
  "query": "margherita pizza",
  "restaurantIdOfAddedItem": "23788",
  "vegFilter": 1,
  "offset": 0
}
```

Observed response text excerpt:

```text
Found 10 menu items for "margherita pizza":
1. Margherita Pizza Regular — ₹109 | Veg | 4.6★ (81) (ID: 163982423)
2. Margherita Pizza — ₹109 | Veg | 4.6★ (2.2K+) (ID: 17857751)
   Variants (Crust): [New Hand Tossed (group:36843806, var:114791995), ...]
   Variants (Size): [Regular (group:36843808, var:114792005), Medium (...), Large (...)]
   Addons (Extra Cheese Regular): [Extra Cheese ₹50 (group:132388512, choice:100001287)]
```

Persist `menu_item_id`, variant `group_id`/`variation_id`, and addon `group_id`/`choice_id`.

### `update_food_cart`

Call used with a simple item without variants/addons:

```json
{
  "restaurantId": "23788",
  "restaurantName": "Domino's Pizza",
  "addressId": "<addressId>",
  "cartItems": [
    {
      "menu_item_id": "163982423",
      "quantity": 1,
      "addons": []
    }
  ]
}
```

Observed response text:

```text
Cart updated.
Restaurant: Domino's Pizza
Items (1):
  - Margherita Pizza Regular — ₹109 (ID: 163982423)

Item total: ₹109
Delivery: FREE
Taxes & charges: ₹51.38
TO PAY: ₹160
```

### `get_food_cart`

Observed response text:

```text
Restaurant: Domino's Pizza
Items (1):
  - Margherita Pizza Regular — ₹109 (ID: 163982423)

Item total: ₹109
Delivery: FREE
Taxes & charges: ₹51.38
TO PAY: ₹160

Payment methods: Cash
```

Checkout precondition: display items, final payable total, payment methods, and delivery address before `place_food_order`.

Use only payment methods returned by `get_food_cart`. Do not mention or assume payment methods not present in the response.

If cart offers show `coupon_applied` with `coupon_discount=0`, treat it as an auto-suggested coupon, not an applied discount. Do not claim savings unless discount is greater than zero.

Only recommend a coupon when its applicability/payment metadata clearly matches the selected payment flow. If COD/Cash compatibility is unclear, show it as informational only and do not auto-apply it.

### `fetch_food_coupons`

Call used:

```json
{
  "restaurantId": "23788",
  "addressId": "<addressId>",
  "couponCode": ""
}
```

Observed response:

```text
Found 0 coupons (0 applicable):

To apply a coupon, use apply_food_coupon with the coupon code and addressId.
```

### `get_food_orders`

Observed response text excerpt:

```text
Found 5 orders:
1. Order <orderId> — Popeyes | Delivered | ₹720
2. Order <orderId> — Subway | Delivered | ₹512
...
```

### `get_food_order_details`

Observed response:

```text
Retrieved details for order <orderId> - Delivered (Popeyes)
```

### `track_food_order`

Observed response:

```text
Tracking 1 active order
```

### `flush_food_cart`

Observed response:

```text
Flushed Food cart successfully
```

## Tools Not Executed And Why

| Tool | Reason |
|---|---|
| `apply_food_coupon` | `fetch_food_coupons` returned zero applicable coupons for the tested restaurant/address. |
| `place_food_order` | Would place a real Food order; no Food order confirmation was requested. |
| `report_error` | Only for reporting failures to Swiggy MCP team; no report was requested. |

## Safety Rules

- Never store or log full phone numbers, full addresses, precise coordinates, or real order IDs unless required and protected.
- Session auto-selection may choose only the first address returned by `get_addresses`; it must not create, infer, or hard-code an address ID.
- Food and Instamart may reuse the session-selected address, but checkout must still show the final address before order placement.
- Manual address switching must use IDs returned by `get_addresses`.
- If `get_addresses` returns no addresses, do not continue into Food ordering. Tell the user they need to add a saved Swiggy address first.
- If the selected session address is missing or stale, call `get_addresses` again and require a valid returned address before Food actions.
- `place_food_order` creates a real Food delivery order. Call it only after the user has seen final address, items, bill total, and payment method, then explicitly confirms.
- Only use payment methods returned by `get_food_cart`.
- Do not place Food orders with final payable cart value `₹1000` or more; direct the user to the Swiggy app.
- On successful order placement, show the returned message as-is. Do not rephrase Swiggy-branded success text.
- For cancellation requests, do not call MCP tools. Tell the user exactly: "To cancel your order, please call Swiggy customer care at 080-67466729."

## Coverage Check

| Endpoint | Tools in live catalog | Tools listed here | Missing |
|---|---:|---:|---:|
| `/food` | 14 | 14 | 0 |
