---
name: swiggy-ssh-clean-architecture
description: Use when making architecture, package-boundary, refactor, naming, or new feature placement decisions in swiggy-ssh. Covers the repo's Clean Architecture layers, dependency rules, package layout, ports/repositories/use-case naming, adapters, and Instamart/cart/checkout/provider placement.
license: MIT
metadata:
  author: swiggy-ssh team
---

# swiggy-ssh Clean Architecture Rules

This repo uses Ports & Adapters / Clean Architecture. Keep business behavior client-agnostic and wire concrete dependencies only at the edge.

Load this skill for:
- package moves or new packages
- new feature placement, especially Instamart/cart/checkout/provider work
- refactors that cross package boundaries
- dependency-boundary or import questions
- ports, repositories, use-case, adapter, or handler naming
- architecture review before/after implementation

---

## Layer Vocabulary

```
cmd/swiggy-ssh/main.go                  composition root: wires concrete adapters
internal/domain/<feature>/              entities, value objects, errors, port interfaces
internal/application/<feature>/         use cases and orchestration using domain ports
internal/presentation/ssh|http|tui/     SSH, HTTP, and Bubbletea adapters only
internal/infrastructure/<tech>/         Postgres, Redis, crypto, provider clients
internal/platform/config|logging/       app-wide config/logging primitives
```

Examples already in the repo:
- `internal/domain/auth`: `OAuthAccount`, login-code/token errors, `Repository`, `LoginCodeService`, `TokenEncryptor`
- `internal/application/auth`: `EnsureValidAccountUseCase.Execute(ctx, input)` OAuth account use case
- `internal/domain/identity`: `SSHIdentity`, `TerminalSession`, `Repository`, `SessionRepository`
- `internal/application/identity`: `ResolveSSHIdentityUseCase`, `StartTerminalSessionUseCase`, `EndTerminalSessionUseCase`
- `internal/presentation/ssh`: SSH listener/session routing and TUI launch
- `internal/infrastructure/persistence/postgres`: `PostgresStore` implementing domain repositories
- `internal/platform/config`: environment parsing and config constants

---

## Strict Dependency Rules

Imports must point inward or sideways only where allowed:

| Package | May import | Must not import |
|---|---|---|
| `internal/domain/*` | stdlib only | any repo package, SSH/HTTP/TUI, Postgres, Redis, config |
| `internal/application/*` | stdlib, `internal/domain/*`, external pure libraries when already used | `internal/infrastructure/*`, `internal/presentation/*`, `internal/platform/*` |
| `internal/presentation/*` | stdlib, `internal/application/*`, `internal/domain/*`, UI/transport libs | `internal/infrastructure/*` |
| `internal/infrastructure/*` | stdlib, `internal/domain/*`, driver/client libs | `internal/presentation/*` |
| `internal/platform/*` | stdlib only or low-level config/logging deps | feature packages: domain/application/presentation/infrastructure |
| `cmd/swiggy-ssh/main.go` | all layers | n/a: this is the wiring point |

Hard rule: presentation adapters must never import infrastructure packages. If SSH or HTTP needs persistence/provider behavior, inject an application service or domain port implementation from `main.go`.

---

## Naming Conventions

Prefer names that say what role the type plays in this architecture.

### Domain
- Entities/value objects: nouns from the business: `SSHIdentity`, `OAuthAccount`, `TerminalSession`, future `Product`, `Cart`, `CheckoutSession`.
- Domain errors: sentinel `Err...` values in the feature domain package: `ErrOAuthAccountNotFound`, `ErrTokenRevoked`.
- Ports:
  - Persistence boundary: `Repository` when there is one primary feature repository in the package.
  - Multiple persistence boundaries: specific names like `SessionRepository`, future `CartRepository`, `OrderRepository`.
  - External capability boundary: capability names like `LoginCodeService`, `TokenEncryptor`, future `ProductProvider`, `CheckoutProvider`.
- Domain comments must describe business concepts, not adapter details. Do not mention Bubbletea, HTTP handlers, SQL tables, Redis keys, or concrete clients unless the domain concept itself is protocol-specific (e.g. SSH public key identity).

### Application
- Use cases own orchestration: `EnsureValidAccountUseCase`, `ResolveSSHIdentityUseCase`, `StartTerminalSessionUseCase`, `EndTerminalSessionUseCase`, future `SearchProductsUseCase`, `AddCartItemUseCase`, `PlaceCheckoutUseCase`.
- Public use-case methods should be `Execute(ctx, input) (output, error)` or `Execute(ctx, input) error` for command-only workflows.
- Constructors are `New<Type>UseCase(ports...)` and accept interfaces/ports, not concrete infrastructure.
- Services with time logic keep `now func() time.Time`; service methods should use `s.now()`, not direct `time.Now()`.
- Application packages may alias domain types when useful for API ergonomics, as current auth/identity packages do.

### Infrastructure
- Concrete adapters use technology/provider names: `PostgresStore`, `RedisLoginCodeService`, `AESEncryptor`, future `SwiggyClient`.
- Adapter methods should implement domain ports exactly and translate concrete errors into domain errors.
- Avoid vague names like `Store`, `Service`, or `Client` when the package/type role is unclear outside its package. `PostgresStore` and `SwiggyClient` are acceptable concrete adapter names.

### Presentation
- Transport entry points are handlers/servers/views: `SSHServer`, HTTP handlers, `HomeView`, `InstamartView`.
- Presentation may render, collect input, manage transport/session mechanics, and call application services.
- Presentation must not own business rules such as token validity, cart pricing, checkout state transitions, or product-provider fallback rules.

---

## Feature Placement Examples

### New Instamart product search
```
internal/domain/instamart/
  product.go              Product, SearchQuery, ErrProductNotFound
  ports.go                ProductProvider interface

internal/application/instamart/
  product_search.go       ProductSearchService.Search(ctx, sshIdentityID, query)

internal/infrastructure/provider/swiggy/
  client.go               SwiggyClient implements instamart.ProductProvider

internal/presentation/tui/
  instamart view renders search input/results

cmd/swiggy-ssh/main.go
  construct SwiggyClient, ProductSearchService, SSH/HTTP adapters
```

### Cart and checkout
```
internal/domain/cart/ or internal/domain/instamart/
  Cart, CartItem, CheckoutSession, Order, CartRepository, CheckoutProvider

internal/application/cart/ or internal/application/instamart/
  CartService: add/remove/update items
  CheckoutService: validate account/address/cart, place order through provider port

internal/infrastructure/persistence/postgres/
  implements CartRepository if cart is persisted locally

internal/infrastructure/provider/swiggy/
  implements CheckoutProvider/ProductProvider via real Swiggy API

internal/presentation/ssh|tui|http/
  input, rendering, routing, login callbacks only
```

Choose one feature package (`instamart` vs split `cart`) based on cohesion. If the concepts share the same lifecycle and provider boundary, keep them together; split when repositories/use cases become independently meaningful.

---

## Workflow for New Feature or Refactor Work

1. Identify the business concept and write/adjust domain types/errors/ports first.
2. Add application use cases that depend only on domain ports.
3. Implement concrete adapters in infrastructure or presentation.
4. Wire concrete adapters to services in `cmd/swiggy-ssh/main.go` only.
5. Keep SSH/TUI/HTTP code as a thin adapter: translate user input to use-case calls and render results.
6. Add tests at the layer where behavior lives: domain/application for business rules, infrastructure for adapter translation, presentation for rendering/session mechanics.
7. Check imports before finishing; no layer should learn about an outer layer.

---

## Common Mistakes in This Repo

- Presentation importing `internal/infrastructure/*` directly instead of receiving an application service.
- Domain types or comments knowing about HTTP handlers, Bubbletea views, Postgres rows, Redis TTL implementation, or concrete Swiggy clients.
- Vague `Store`/`Service` names for new abstractions when `CartRepository`, `ProductProvider`, or `CheckoutService` would communicate intent.
- SSH/TUI orchestration owning business logic such as auth state transitions, token validation, cart rules, checkout sequencing, or provider retry/fallback decisions.
- Application services importing `config` for env values instead of receiving explicit constructor parameters.
- Wiring infrastructure in package `init` functions or presentation constructors instead of `main.go`.
- Calling `time.Now()` inside application service methods instead of the service's injectable clock.
- Adding broad fallback paths to cross boundaries; prefer a single explicit port and adapter.
