---
name: go-doc
description: >-
  Go documentation patterns — Go 1.19+ doc comment format, a what-to-document
  decision tree, package-level docs (doc.go template), function/type/constant
  and error-sentinel documentation, symbol links, lists, headings, and
  doc-comment anti-patterns.
when_to_use: >-
  Use when writing or reviewing doc comments, adding package documentation or
  a doc.go file, deciding whether an exported or unexported symbol needs a
  comment, documenting sentinel errors or constant groups, or when doc
  comment formatting (links, lists, headings, godoc rendering) is unclear.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Documentation Patterns

## What to Document Decision Tree

```
Is the symbol exported?
├─ No  → Document only if the logic is non-obvious
│   Short comment explaining WHY, not WHAT
├─ Yes → MUST have a doc comment (go-philosophy.md rule)
│   ├─ Is it a package declaration?
│   │   └─ Yes → Package doc in doc.go or main file (see below)
│   ├─ Is it a type?
│   │   └─ "<TypeName> represents/defines/holds..."
│   ├─ Is it a function/method?
│   │   └─ "<FuncName> returns/creates/processes..."
│   ├─ Is it a constant/variable group?
│   │   └─ Document the group, then individual items if non-obvious
│   └─ Is it an error sentinel?
│       └─ "<ErrName> is returned when..."
```

## Doc Comment Format (Go 1.19+)

### Basic Rules

```go
// Order represents a customer order with line items and metadata.
//
// An Order transitions through statuses: pending → confirmed → shipped → delivered.
// Use [NewOrder] to create an Order with validated fields.
type Order struct { ... }

// NewOrder returns an Order with the given parameters after validation.
// It returns [ErrInvalidInput] if total is negative.
func NewOrder(userID string, total int) (*Order, error) { ... }

// ErrNotFound is returned when a requested order does not exist.
var ErrNotFound = errors.New("not found")
```

**Rules**:
- First sentence starts with the symbol name
- First sentence is a complete sentence ending with a period
- Use `[SymbolName]` to link to other symbols (Go 1.19+)
- Blank comment line (`//`) creates a paragraph break

### Links to Symbols

```go
// Store provides order persistence backed by PostgreSQL.
//
// Use [NewStore] to create a Store. The Store implements
// the [OrderReader] and [OrderWriter] interfaces.
//
// See also: [order.Status] for valid status values.
type Store struct { ... }
```

Link syntax:
- `[FuncName]` — function in same package
- `[TypeName]` — type in same package
- `[pkg.Symbol]` — symbol in another package
- `[TypeName.Method]` — method on a type

### Headings, Lists, and Code Blocks

```go
// Package order provides order management for the e-commerce platform.
//
// # Architecture
//
// The package follows the project's package-by-feature structure:
//   - order.go: types, constants, sentinel errors
//   - handler.go: HTTP handlers
//   - store.go: PostgreSQL operations
//
// # Usage
//
// Create a store and wire it to handlers:
//
//	store := order.NewStore(pool)
//	mux.HandleFunc("GET /orders/{id}", order.GetHandler(store, logger))
//
// # Status Transitions
//
// Orders follow a strict state machine:
//
//	pending → confirmed → shipped → delivered
//	    │         │
//	    └─────────┴──→ cancelled
package order
```

**Format rules**:
- Headings: `// # Heading` (Go 1.19+)
- Lists: `//   - item` (indented with spaces)
- Code blocks: indented with tab (`//\t code`)
- Blank `//` line before headings and code blocks

## Package Documentation

### Where to Put It

| Situation | Location |
|-----------|----------|
| Small package (1-3 files) | Top of the main `.go` file |
| Large package (4+ files) | Separate `doc.go` file |

### doc.go Template

```go
// Package order provides order management including creation,
// retrieval, and status tracking.
//
// # Store
//
// [Store] handles all database operations. Create one with [NewStore]:
//
//	store := order.NewStore(pool)
//
// # HTTP Handlers
//
// Handler functions follow the closure pattern, accepting dependencies
// and returning [http.HandlerFunc]:
//
//	mux.HandleFunc("GET /orders/{id}", order.GetHandler(store, logger))
//
// # Errors
//
// The package defines sentinel errors for common failure cases:
//   - [ErrNotFound]: order does not exist
//   - [ErrConflict]: duplicate order ID
package order
```

## Function Documentation

### Decision: What to Include

```
Does the function have non-obvious behavior?
├─ No  → Single sentence: "<Name> does X."
├─ Yes → Add:
│   ├─ What it returns on error (which sentinel errors?)
│   ├─ What side effects it has (writes to DB, sends notification?)
│   ├─ What the zero value means (if returning struct)
│   └─ Any concurrency safety notes
```

### Examples

```go
// Order returns the order with the given ID.
// It returns [ErrNotFound] if no order exists with that ID.
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {

// CreateOrder inserts a new order and returns it with a generated ID.
// It returns [ErrConflict] if an order with the same idempotency key exists.
func (s *Store) CreateOrder(ctx context.Context, p CreateParams) (*Order, error) {

// Close releases all resources held by the server.
// It blocks until all in-flight requests complete or ctx is cancelled.
func (s *Server) Close(ctx context.Context) error {
```

## Constant and Variable Groups

```go
// Order statuses track the lifecycle of an order.
const (
    StatusPending   Status = "pending"   // initial state after creation
    StatusConfirmed Status = "confirmed" // payment verified
    StatusShipped   Status = "shipped"   // handed to carrier
    StatusDelivered Status = "delivered" // customer received
    StatusCancelled Status = "cancelled" // order cancelled
)

// Sentinel errors for order operations.
var (
    // ErrNotFound is returned when the requested order does not exist.
    ErrNotFound = errors.New("not found")

    // ErrConflict is returned when a unique constraint is violated.
    ErrConflict = errors.New("conflict")
)
```

## Anti-Patterns

### Restating the Name

```go
// ❌ WRONG — adds no information
// Order is an order.
type Order struct { ... }

// ✅ CORRECT — explains what it represents
// Order represents a customer purchase with line items and shipping details.
type Order struct { ... }
```

### Documenting Implementation

```go
// ❌ WRONG — describes HOW (implementation detail)
// Order queries the database using the orders table and joins with
// order_items, then maps the rows to an Order struct.
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {

// ✅ CORRECT — describes WHAT and WHEN
// Order returns the order with the given ID.
// It returns [ErrNotFound] if no order exists with that ID.
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {
```

### Missing Error Documentation

```go
// ❌ WRONG — caller doesn't know what errors to handle
// CreateOrder creates an order.
func (s *Store) CreateOrder(ctx context.Context, p CreateParams) (*Order, error) {

// ✅ CORRECT — documents error conditions
// CreateOrder inserts a new order and returns it with a generated ID.
// It returns [ErrConflict] if an order with the same idempotency key exists.
// It returns [ErrInvalidInput] if required fields are missing.
func (s *Store) CreateOrder(ctx context.Context, p CreateParams) (*Order, error) {
```

### Over-Documentation

```go
// ❌ WRONG — documenting obvious getter
// ID returns the order's ID.
func (o *Order) ID() string { return o.id }

// ✅ CORRECT — skip doc for trivial accessors, or keep minimal
func (o *Order) ID() string { return o.id }
```

### Wrong Comment Format

```go
// ❌ WRONG — not starting with symbol name
// This function creates a new order store.
func NewStore(pool *pgxpool.Pool) *Store {

// ❌ WRONG — using /* */ for doc comments
/* NewStore returns a Store backed by the given pool. */
func NewStore(pool *pgxpool.Pool) *Store {

// ✅ CORRECT — // comment starting with symbol name
// NewStore returns a Store backed by the given database pool.
func NewStore(pool *pgxpool.Pool) *Store {
```

See: go-philosophy.md rule for the doc comment requirement on exported symbols.
See: naming.md rule for naming conventions that make docs clearer.
