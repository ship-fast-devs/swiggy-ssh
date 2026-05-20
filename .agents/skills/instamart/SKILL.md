---
name: instamart
description: Use when working with Swiggy MCP /im Instamart tools, grocery product search, Instamart cart, checkout, order history, or tracking flows.
---

# Swiggy MCP `/im` Tool Usage

Generated and verified on 2026-05-16 against `https://mcp.swiggy.com/im`.

This file covers Instamart only. Tool names are unqualified in MCP requests: POST to `/im`, then call tool name `get_addresses`, not `im.get_addresses`.

## MCP Assumption

Connection and session setup is out of scope for this skill. Assume the caller already has a working MCP client wired to:

```text
POST https://mcp.swiggy.com/im
```

This skill only documents tool names, arguments, workflows, response shapes, and safety gates.

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
    "name": "search_products",
    "arguments": {
      "addressId": "<addressId>",
      "query": "diet coke",
      "offset": 0
    }
  }
}
```

## Workflow Tree

1. `get_addresses`
2. Select `addressId` from `data.addresses[].id`
3. `search_products` with `addressId`, `query`, optional `offset`
4. Present returned product variations and require the user to choose the exact pack/variant
5. Select `spinId` from the chosen `data.products[].variations[]`
6. `update_cart` with `selectedAddressId` and `items[]`
7. `get_cart`
8. Show address, items, bill breakdown, and `availablePaymentMethods`
9. If cart contains items from multiple stores, show that the system will create separate orders and report each result separately
10. Require explicit confirmation
11. `checkout` with `addressId` and selected/available `paymentMethod`
12. `track_order` with `orderId`, `lat`, `lng`
13. `get_orders` for active/history lookup

Important: `update_cart` replaces the whole Instamart cart with the passed `items[]`. Preserve existing items yourself if you need additive behavior.

## Tool Catalog

| Tool | Description | Required args | Optional args | How to use |
|---|---|---|---|---|
| `get_addresses` | Gets saved delivery addresses. | - | - | First call for Instamart flows requiring `addressId`. |
| `search_products` | Searches products available at the selected address. | `addressId`, `query` | `offset` | Present product variations to the user; only use the `spinId` for the exact selected pack/variant. Response `nextOffset` is string-shaped. |
| `your_go_to_items` | Fetches frequently/recently ordered Instamart items. | `addressId` | `offset` | Use for personalized reorder suggestions. Present variations and use the chosen `spinId`. Response `nextOffset` is string-shaped. |
| `get_cart` | Gets current Instamart cart, bill breakdown, and payment methods. | - | - | Call after cart mutation and before checkout. |
| `update_cart` | Replaces cart with provided grocery items. | `selectedAddressId`, `items` | - | `items[]` requires `spinId` and `quantity`. |
| `clear_cart` | Clears Instamart cart. | - | - | Use for cleanup or explicit empty-cart action. |
| `checkout` | Places and confirms Instamart order. | `addressId` | `paymentMethod` | Only after `get_cart`, address/payment/bill display, and explicit confirmation. |
| `get_orders` | Gets Instamart order history or active orders. | - | `count`, `orderType`, `activeOnly` | Use `activeOnly=true` for current orders. |
| `track_order` | Tracks Instamart order status in real time. | `orderId`, `lat`, `lng` | - | Use order id from `checkout`/`get_orders` and delivery coordinates. |
| `report_error` | Generates MCP team error report. | `tool`, `errorMessage` | `domain`, `flowDescription`, `toolContext`, `userNotes` | Include IDs like `orderId`, `spinId`, `addressId`, `paymentMethod`. |

## Real Response Examples

### `get_addresses`

Observed shape:

```json
{
  "success": true,
  "data": {
    "addresses": [
      {
        "id": "<addressId>",
        "addressLine": "<redacted office address>",
        "phoneNumber": "****6438",
        "addressCategory": "Other",
        "addressTag": "Pratilipi - Office"
      }
    ]
  }
}
```

Persist `data.addresses[].id`. Initial `get_addresses` returns address summary fields (`id`, `addressLine`, `phoneNumber`, `addressCategory`, `addressTag`); do not assume `area` or `city` are present here. `area`/`city` may appear later in cart address details.

### `search_products`

Call used:

```json
{
  "addressId": "<addressId>",
  "query": "diet coke",
  "offset": 0
}
```

Observed response excerpt:

```json
{
  "success": true,
  "data": {
    "nextOffset": "1",
    "products": [
      {
        "displayName": "Coca-Cola Diet Coke Can",
        "brand": "Coca Cola",
        "inStock": true,
        "isAvail": true,
        "productId": "SKLNYNX61O",
        "parentProductId": "ECARNG1K2X",
        "isPromoted": false,
        "variations": [
          {
            "spinId": "6W66L24IMW",
            "quantityDescription": "300 ml x 2",
            "displayName": "Coca-Cola Diet Coke Can",
            "brandName": "Coca Cola",
            "price": {"mrp": 80, "offerPrice": 80},
            "isInStockAndAvailable": true,
            "imageUrl": "https://media-assets.swiggy.com/..."
          }
        ]
      }
    ]
  }
}
```

Persist `variations[].spinId`, variation name, pack size, stock flags, and price. Decode `data.nextOffset` as a string or string/number tolerant value.

If `products[].isPromoted` is `true`, clearly mark the product as featured/sponsored in the UI.

### `update_cart`

Call used for two Diet Coke cans at Pratilipi office. This is quantity `1` of a `300 ml x 2` variation.

```json
{
  "selectedAddressId": "<addressId>",
  "items": [
    {"spinId": "6W66L24IMW", "quantity": 1}
  ]
}
```

Observed response excerpt:

```json
{
  "success": true,
  "data": {
    "selectedAddress": "<addressId>",
    "selectedAddressDetails": {
      "annotation": "Pratilipi - Office",
      "area": "Sona Towers",
      "city": "Bangalore",
      "lat": "<redacted>",
      "lng": "<redacted>",
      "name": "<redacted>",
      "mobile": "<redacted>",
      "flatNo": "<redacted>"
    },
    "cartTotalAmount": "₹150",
    "items": [
      {
        "spinId": "6W66L24IMW",
        "itemName": "Coca-Cola Diet Coke Can 2 Pieces",
        "quantity": 1,
        "storeId": 1402444,
        "isInStockAndAvailable": true,
        "mrp": 80,
        "discountedFinalPrice": 80
      }
    ],
    "billBreakdown": {
      "lineItems": [
        {"label": "Item Total", "value": "₹80.00"},
        {"label": "Handling Fee", "value": "₹11.00"},
        {"label": "Small Cart Fee", "value": "₹20.00"},
        {"label": "Delivery Partner Fee", "value": "₹30.00"},
        {"label": "GST and Charges", "value": "₹9.00"}
      ],
      "toPay": {"label": "To Pay", "value": "₹150"}
    }
  }
}
```

### `get_cart`

Observed additional payment field after `update_cart`:

```json
{
  "success": true,
  "data": {
    "cartTotalAmount": "₹150",
    "availablePaymentMethods": ["Cash"]
  }
}
```

Checkout precondition: display `selectedAddressDetails`, `items`, `billBreakdown`, and `availablePaymentMethods` before calling `checkout`.

### `checkout`

Call used after explicit user confirmation:

```json
{
  "addressId": "<addressId>",
  "paymentMethod": "Cash"
}
```

Observed real order response:

```json
{
  "success": true,
  "data": {
    "orderId": "<orderId>",
    "status": "CONFIRMED",
    "paymentMethod": "Cash",
    "cartTotal": 150
  },
  "message": "Instamart order placed successfully! Sit back and enjoy! Order ID: <orderId>"
}
```

Persist `data.orderId`, `data.status`, `data.paymentMethod`, and `data.cartTotal`.

### `track_order`

Call shape:

```json
{
  "orderId": "<orderId>",
  "lat": "<delivery-lat>",
  "lng": "<delivery-lng>"
}
```

Observed response immediately after checkout:

```json
{
  "success": true,
  "data": {
    "orderId": "<orderId>",
    "orderTitle": "Instamart order",
    "orderSubtitle": "05:40 PM • 1 items",
    "status": {
      "statusMessage": "Order is getting packed!",
      "subStatusMessage": "We'll assign a delivery partner soon",
      "etaMinutes": 5,
      "etaText": "5 mins"
    },
    "storeInfo": {"name": "Instamart", "address": "<redacted store address>"},
    "deliveryInfo": {"addressLabel": "Pratilipi - Office", "fullAddress": "<redacted office address>"},
    "items": [{"name": "1 x Coca-Cola Diet Coke Can", "quantity": 1, "price": "₹80"}],
    "pollingIntervalSeconds": 30
  },
  "message": "Order is getting packed! - ETA: 5 mins"
}
```

Observed response after delivery completed:

```json
{
  "success": true,
  "data": {
    "orderId": "<orderId>",
    "status": {"statusMessage": "Order Delivered"},
    "deliveryInfo": {"addressLabel": "Delivered to Pratilipi - Office", "fullAddress": "<redacted office address>"},
    "items": [{"name": "1 x Coca-Cola Diet Coke Can", "quantity": 1, "price": "₹80"}],
    "pollingIntervalSeconds": -1
  },
  "message": "Order Delivered"
}
```

### `get_orders`

Observed active-order response excerpt:

```json
{
  "success": true,
  "data": {
    "orders": [
      {
        "orderId": "<orderId>",
        "status": "CONFIRMED",
        "estimatedDeliveryTime": "5 mins",
        "itemCount": 1,
        "totalAmount": 150,
        "paymentMethod": "Cash",
        "orderType": "DASH",
        "isActive": true,
        "currentStatus": "Order Confirmed!",
        "storeName": "Instamart",
        "items": [{"name": "Coca-Cola Diet Coke Can", "quantity": 1, "itemId": "<itemId>"}],
        "billDetails": {"itemTotal": 80, "deliveryFee": 30, "packagingFee": 11, "grandTotal": 141},
        "paymentStatus": "SUCCESS",
        "refundStatus": "NO_REFUND"
      }
    ],
    "hasMore": false
  }
}
```

Note: this observed order-history response reported `totalAmount` as `150` while nested `billDetails.grandTotal` was `141`. Treat `checkout.data.cartTotal` / cart `billBreakdown.toPay` as the checkout amount and preserve both order-history fields if you log order history.

### `your_go_to_items`

Observed response shape:

```json
{
  "success": true,
  "data": {
    "nextOffset": "0",
    "products": [
      {
        "displayName": "Coca-Cola Diet Coke Can",
        "brand": "Coca Cola",
        "inStock": true,
        "isAvail": true,
        "variations": [
          {"spinId": "6W66L24IMW", "quantityDescription": "300 ml x 2", "price": {"mrp": 80, "offerPrice": 80}}
        ]
      }
    ]
  }
}
```

Decode `data.nextOffset` as a string or string/number tolerant value.

### `get_cart` Failure Example

Observed before a cart existed:

```json
{
  "success": false,
  "error": {
    "message": "Cart not found or session expired. This can happen if the cart was cleared or the session timed out. Please add items to your cart again using update_cart with a valid addressId."
  },
  "reportLink": "mailto:mcp-support@swiggy.in?...",
  "reportHint": "Something went wrong. You can report this error to the Swiggy MCP team using the link above, or ask me to report this error for a detailed report."
}
```

## Safety Rules

- Never store or log full phone numbers, full addresses, precise coordinates, or real order IDs unless required and protected.
- `checkout` creates a real Instamart order. Call it only after the user has seen final address, items, bill breakdown, and payment method, then explicitly confirms.
- Only use payment methods returned by `get_cart`.
- Check cart value before checkout. If cart is `₹1000` or more, ask the user to use the Swiggy Instamart app.
- If the cart has items from multiple stores, inform the user before checkout and report each resulting order separately after checkout.
- For cancellation requests, do not call MCP tools. Tell the user exactly: "To cancel your order, please call Swiggy customer care at 080-67466729."

## Coverage Check

| Endpoint | Tools in live catalog | Tools listed here | Missing |
|---|---:|---:|---:|
| `/im` | 10 | 10 | 0 |
