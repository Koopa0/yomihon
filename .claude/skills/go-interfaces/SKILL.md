---
name: go-interfaces
description: >-
  Go interface design patterns -- consumer-side interfaces, interface
  size decisions, composition, implicit satisfaction, testing with
  interfaces, and common pitfalls.
when_to_use: >-
  Use when defining an interface, deciding where to place it, choosing
  between interface and concrete type, designing cross-package
  boundaries, or when interface satisfaction behavior is unclear.
  Triggers: "define an interface", "interface vs struct", "where should
  this interface live", "accept interfaces return structs", "mock/fake
  for testing", "does X satisfy Y".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Interface Design Patterns

## Interface Size Decision Tree

```
How many methods does the consumer need?
├─ 1 method → single-method interface (name: method + "-er")
│   Examples: Reader, Writer, Handler, Stringer
├─ 2-3 methods → small interface (name: descriptive noun)
│   Examples: OrderReader, UserStore
├─ 4+ methods → STOP. Do you really need all of them?
│   ├─ Yes, in a single call site → split into 2-3 smaller interfaces
│   └─ No, different call sites need different subsets → one interface per call site
```

**The alarm threshold is 4 methods.** If an interface has 4+ methods, it almost
certainly violates the Interface Segregation Principle. Split it.

## Where to Define Interfaces

### Consumer-Side (CORRECT)

```go
// internal/notification/handler.go — CONSUMER defines what it needs
package notification

// OrderReader is the subset of order operations this package needs.
type OrderReader interface {
    Order(ctx context.Context, id string) (*order.Order, error)
}

func sendNotification(reader OrderReader, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        o, err := reader.Order(r.Context(), r.PathValue("id"))
        // ...
    }
}
```

```go
// internal/order/store.go — PRODUCER returns concrete type
package order

// Store provides order database operations.
type Store struct {
    q *db.Queries
}

// NewStore returns a Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
    return &Store{q: db.New(dbtx)}
}

// Order returns a single order by ID.
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {
    // ...
}
```

### Producer-Side (WRONG)

```go
// ❌ WRONG — producer defines interface for its own type
package order

type Store interface {
    Order(ctx context.Context, id string) (*Order, error)
    Orders(ctx context.Context, f Filter) ([]Order, error)
    CreateOrder(ctx context.Context, p CreateParams) (*Order, error)
}

type storeImpl struct { q *db.Queries }

func NewStore(dbtx db.DBTX) Store { // ❌ returns interface
    return &storeImpl{q: db.New(dbtx)}
}
```

Why this is wrong:
- Producer can't predict what consumers need
- Forces all consumers to depend on methods they don't use
- Hides concrete type — prevents direct method calls, struct embedding
- Java pattern, not Go

### Decision: Where Does This Interface Go?

```
Who USES the interface?
├─ One consumer package → define in that package
├─ Multiple consumer packages, same methods → define in the most common consumer
├─ Standard library interface (io.Reader, etc.) → use directly, don't redefine
├─ Cross-cutting (middleware, logging) → define nearest to usage
```

## Accept Interfaces, Return Structs

```go
// ✅ Function ACCEPTS interface — flexible for callers
func ProcessOrder(ctx context.Context, reader OrderReader) error {
    // ...
}

// ✅ Function RETURNS concrete struct — callers decide what interface to use
func NewStore(dbtx db.DBTX) *Store {
    return &Store{q: db.New(dbtx)}
}

// ❌ WRONG — returning interface hides concrete type
func NewStore(dbtx db.DBTX) OrderReader {
    return &Store{q: db.New(dbtx)}
}
```

### Exceptions (Rare)

Returning an interface is acceptable when:
- The function is a factory for multiple concrete types (rare in this project)
- Standard library convention: `errors.New` returns `error` interface
- The concrete type is unexported by design

## Implicit Interface Satisfaction

Go interfaces are satisfied **implicitly** — no `implements` keyword.

```go
// store.go — no declaration that Store implements OrderReader
type Store struct{ q *db.Queries }

func (s *Store) Order(ctx context.Context, id string) (*Order, error) { ... }

// handler.go — Store satisfies OrderReader automatically
type OrderReader interface {
    Order(ctx context.Context, id string) (*Order, error)
}

// Compile-time verification (optional but recommended)
var _ OrderReader = (*Store)(nil)
```

### Method Set and Interface Satisfaction

```
*Store has method set: all methods (value + pointer receivers)
Store  has method set: only value receiver methods
```

```go
type Saver interface { Save() error }

type Doc struct{}
func (d *Doc) Save() error { return nil } // pointer receiver

var s Saver
s = &Doc{} // ✅ *Doc satisfies Saver
s = Doc{}  // ❌ compile error: Doc does not implement Saver
```

See: go-types skill § Receiver Decision Tree for when to use pointer vs value receiver.

## Interface Composition

### Embedding Small Interfaces

```go
// ✅ Compose larger interfaces from smaller ones
type OrderReader interface {
    Order(ctx context.Context, id string) (*Order, error)
}

type OrderWriter interface {
    CreateOrder(ctx context.Context, p CreateParams) (*Order, error)
    UpdateOrder(ctx context.Context, id string, p UpdateParams) (*Order, error)
}

// Only use when a consumer truly needs both
type OrderReadWriter interface {
    OrderReader
    OrderWriter
}
```

### stdlib Examples

```go
io.ReadWriter    = io.Reader + io.Writer
io.ReadCloser    = io.Reader + io.Closer
io.ReadWriteCloser = io.Reader + io.Writer + io.Closer
```

### When NOT to Compose

```go
// ❌ WRONG — composing unrelated interfaces
type UserOrderStore interface {
    UserReader
    OrderReader
    OrderWriter
    PaymentReader
}
// This is a God interface. Each consumer should declare only what it needs.
```

## Testing and Interfaces

Tests are NEVER a reason to introduce an interface. Interfaces are
DISCOVERED (a second production implementation exists, or a cross-feature
consumer boundary appears) — tests then use whatever already exists.
See `.claude/rules/interfaces.md` and `.claude/rules/testing.md` § Test Doubles.

### Test Doubles: Real First, Fakes Last

The DEFAULT is no double at all — wire the real dependency. A consumer that
needs order data gets a real `*order.Store` backed by real PostgreSQL via
testcontainers-go. HTTP dependencies get a real `httptest.Server`.

```go
// internal/notification/integration_test.go
//go:build integration

package notification_test

func TestSendNotification(t *testing.T) {
    pool := startPostgres(t) // testcontainers-go — see testcontainers skill
    store := order.NewStore(pool)
    seedOrder(t, pool, "abc") // real INSERT: real schema, real constraints

    h := notification.NewHandler(store, slog.Default()) // *order.Store satisfies OrderReader
    req := httptest.NewRequest("GET", "/notify/abc", nil)
    w := httptest.NewRecorder()
    h.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("send notification status = %d, want %d", w.Code, http.StatusOK)
    }
}
```

A hand-written fake is the LAST resort — allowed ONLY for a
non-containerizable external (paid API, third-party SaaS). It implements an
EXISTING consumer interface (one already discovered at a cross-feature
boundary — never one invented for the test), lives in the same file as the
test using it, is a plain struct of 5-15 lines, and is asserted on OUTPUTS
only — NEVER call order or call counts.

```go
// internal/order/handler_test.go
package order

// Charger is the EXISTING consumer interface order defines over the
// payment gateway client (a paid external API — nothing to containerize).
type fakeCharger struct {
    receipt *payment.Receipt
    err     error
}

func (f *fakeCharger) Charge(_ context.Context, _ payment.Request) (*payment.Receipt, error) {
    return f.receipt, f.err
}

func TestCheckout(t *testing.T) {
    tests := []struct {
        name       string
        charger    Charger
        wantStatus int
    }{
        {
            name:       "charge accepted",
            charger:    &fakeCharger{receipt: &payment.Receipt{ID: "rcpt_1"}},
            wantStatus: http.StatusOK,
        },
        {
            name:       "charge declined",
            charger:    &fakeCharger{err: payment.ErrDeclined},
            wantStatus: http.StatusPaymentRequired,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := checkout(tt.charger, slog.Default())
            req := httptest.NewRequest("POST", "/checkout", strings.NewReader(`{"order_id":"abc"}`))
            req.Header.Set("Content-Type", "application/json")
            w := httptest.NewRecorder()
            handler.ServeHTTP(w, req)

            if w.Code != tt.wantStatus {
                t.Errorf("checkout() status = %d, want %d", w.Code, tt.wantStatus)
            }
        })
    }
}
```

The assertions are on the HTTP response — an output. "Charge was called
once with amount X" is a forbidden mock-verification test
(rules/testing.md § Low-Value Tests #5).

### Why Not Mock Frameworks

- Mock frameworks (gomock, mockery) are forbidden — depguard blocks the
  imports, check-anti-patterns.sh blocks the codegen
- They test implementation details (method call order, argument matching) —
  exactly the assertions rules/testing.md § Low-Value Tests forbids
- Real dependencies (testcontainers-go, httptest.Server) catch real bugs;
  a hand-written fake covers only the rare non-containerizable external

See: testcontainers skill for database integration testing.

### Functional Fakes (Single-Method Interfaces Only)

Same rules as struct fakes — non-containerizable externals only, EXISTING
consumer interface, same file as the test, assert outputs only:

```go
type ChargerFunc func(ctx context.Context, req payment.Request) (*payment.Receipt, error)

func (f ChargerFunc) Charge(ctx context.Context, req payment.Request) (*payment.Receipt, error) {
    return f(ctx, req)
}

// Usage in test
charger := ChargerFunc(func(_ context.Context, req payment.Request) (*payment.Receipt, error) {
    if req.Amount > 1_000_00 {
        return nil, payment.ErrDeclined
    }
    return &payment.Receipt{ID: "rcpt_1"}, nil
})
```

Only for 1-method interfaces. For 2+ methods, use a struct fake.

## Common Interface Patterns in This Project

### Handler Dependencies

A handler in the same package as its store takes the concrete `*Store`
directly — no interface. There is no second implementation and no
cross-package boundary, so neither discovery case applies:

```go
// internal/order/handler.go — same package as Store
func getOrder(store *Store, logger *slog.Logger) http.HandlerFunc { ... }
func createOrder(store *Store, logger *slog.Logger) http.HandlerFunc { ... }
func listOrders(store *Store, logger *slog.Logger) http.HandlerFunc { ... }
```

The honest consequence: any handler test that needs data goes through the
feature's single `integration_test.go` (`//go:build integration`,
`package order_test`, testcontainers) — handler + store + real SQL
exercised together. There is NO stubbed unit-test path for these handlers,
by design. White-box unit tests in `<feature>_test.go` remain only for
paths that never reach the store (bad JSON, missing path value).

### Cross-Feature Dependencies

```go
// internal/invoice/handler.go — invoice needs to read orders
package invoice

// OrderReader defines the order operations invoice needs.
type OrderReader interface {
    Order(ctx context.Context, id string) (*order.Order, error)
}

// In cmd/app/main.go — wiring
orderStore := order.NewStore(pool)
invoiceHandler := invoice.NewHandler(orderStore, logger) // *order.Store satisfies invoice.OrderReader
```

## Anti-Patterns

### Premature Interface

```go
// ❌ WRONG — interface with only one implementation, designed up front
type UserStore interface {
    User(ctx context.Context, id string) (*User, error)
}

type userStoreImpl struct { ... } // the only implementation

// ✅ CORRECT — concrete type until an interface is DISCOVERED: a second
// production implementation exists, or a cross-feature consumer boundary
// appears. Tests are NEVER a reason.
type Store struct { ... }
func (s *Store) User(ctx context.Context, id string) (*User, error) { ... }
```

### God Interface

```go
// ❌ WRONG — 7 methods, no consumer needs all of them
type OrderService interface {
    Order(ctx context.Context, id string) (*Order, error)
    Orders(ctx context.Context, f Filter) ([]Order, error)
    CreateOrder(ctx context.Context, p CreateParams) (*Order, error)
    UpdateOrder(ctx context.Context, id string, p UpdateParams) (*Order, error)
    DeleteOrder(ctx context.Context, id string) error
    CancelOrder(ctx context.Context, id string) error
    RefundOrder(ctx context.Context, id string) error
}

// ✅ CORRECT — each consumer declares 1-2 methods
type OrderReader interface {
    Order(ctx context.Context, id string) (*Order, error)
}
type OrderCanceller interface {
    CancelOrder(ctx context.Context, id string) error
}
```

### Interface Pollution

```go
// ❌ WRONG — interface for every type, Java-style
type UserRepository interface { ... }
type OrderRepository interface { ... }
type PaymentRepository interface { ... }

// ✅ CORRECT — interfaces only where needed
// Most handlers take *Store directly
// Interfaces only at cross-package boundaries
```

### Embedding Interface in Struct

```go
// ❌ WRONG — zero value has nil interface, panics on call
type TrackedWriter struct {
    io.Writer // embedded interface
    bytes int
}

w := TrackedWriter{} // Writer is nil
w.Write(data)        // PANIC

// ✅ CORRECT — named field, explicit initialization
type TrackedWriter struct {
    w     io.Writer
    bytes int
}

func NewTrackedWriter(w io.Writer) *TrackedWriter {
    return &TrackedWriter{w: w}
}
```

### Interface{} / any as Escape Hatch

```go
// ❌ WRONG — losing type safety
func process(data any) { ... }

// ✅ CORRECT — use generics or concrete types
func process(data Order) { ... }
func process[T Processable](data T) { ... }
```

Use `any` only at true boundaries (JSON decode, context values).
See: go-generics skill for when generics vs interface is appropriate.

## Naming Conventions

| Pattern | Example | When |
|---------|---------|------|
| Method + "-er" | `Reader`, `Writer`, `Closer` | Single-method interface |
| Descriptive noun | `OrderReader`, `UserStore` | Multi-method interface |
| No `I` prefix | `Store`, not `IStore` | Always |
| No stutter | `order.Reader`, not `order.OrderReader` | When in same package as type |

Exception: when the interface name would collide with a concrete type in the same
package, use a descriptive prefix: `OrderReader` in `notification` package is fine
because it reads `order.Order`.
