# Repository Engineering, Acceptance, and Evidence Standard

Version: 2.0  
Status: Normative  
Date: 2026-07-18  
Audience: product owners, maintainers, builders, reviewers, release owners, and coding agents  
Primary language target: Go  
Additional surfaces: HTML, CSS, JavaScript, SQL, CLI, HTTP/JSON, RPC, local applications, distributed systems, and agent-facing systems

## 0. How to use this standard

This document is a repository constitution, not a suggestion list.

A repository adopts it by:

1. placing this document, or an unmodified organization-controlled copy, at a stable path;
2. adding a project-specific `PROJECT_PROFILE.md` that resolves applicability, risk, commands,
   owners, compatibility, and performance budgets;
3. making one canonical verification command execute every mandatory automated check;
4. requiring the three acceptance gates in pull-request and release policy;
5. recording exceptions through the exception process in this standard;
6. binding final acceptance to an immutable commit or artifact digest.

A repository MAY add stricter local rules. It MUST NOT silently weaken this standard.
A local rule that conflicts with this standard is invalid unless a recorded exception explicitly
identifies the conflicting clauses, owner, risk, mitigation, and review trigger.

The project profile MUST classify every conditional requirement as one of:

- **APPLIES**: the requirement is mandatory;
- **N/A**: the capability or risk does not exist, with a concrete reason;
- **DEFERRED-BY-EXCEPTION**: a reviewed, time-bounded exception exists;
- **UNRESOLVED**: applicability is not yet settled and acceptance is blocked.

`N/A` is not a convenience escape hatch. It is a falsifiable statement about product scope.

## 1. Purpose

This standard defines when engineering work is:

- semantically correct;
- architecturally coherent;
- idiomatic and maintainable;
- safe with respect to authority, security, privacy, concurrency, and durability;
- usable by an unfamiliar human or software agent;
- operationally supportable;
- supported by evidence that will fail when the protected contract is broken.

Passing tests is not sufficient. A change is acceptable only when three independent gates pass:

1. **Architecture and open-source engineering quality**;
2. **Real-user and third-party-agent usability**;
3. **Test and evidence-system quality**.

Failure of any gate blocks a completion, merge-ready, release-ready, or production-ready claim.

## 2. Normative language and verdict vocabulary

### 2.1 Requirement words

- **MUST**: required; violation blocks acceptance.
- **MUST NOT**: prohibited; violation blocks acceptance.
- **SHOULD**: expected unless a concrete, reviewed reason proves another design is better.
- **SHOULD NOT**: normally prohibited unless a concrete, reviewed reason proves otherwise.
- **MAY**: optional; it MUST NOT be represented as required.
- **NEEDS-OWNER**: more than one reasonable product behavior remains.
- **UNVERIFIED**: evidence was not executed or directly observed on the reviewed snapshot.

### 2.2 Claim status

Every material review claim MUST be marked:

- **verified**: directly inspected, executed, or observed;
- **inferred**: uniquely derived from verified facts;
- **assumed**: temporarily accepted for reversible work, but not proven;
- **unresolved**: multiple plausible interpretations remain;
- **unverified**: evidence is absent or was not run.

Assumed, unresolved, and unverified claims MUST NOT support a `GO` verdict.

### 2.3 Check status

Every required check has exactly one status:

- **PASS**: executed successfully on the identified snapshot;
- **FAIL**: executed and failed, or inspection found a violation;
- **N/A**: demonstrably outside product scope;
- **UNVERIFIED**: not executed or not inspectable;
- **BLOCKED**: could not execute because a named prerequisite failed.

`BLOCKED` and `UNVERIFIED` are not variants of PASS.

## 3. Authority hierarchy

Engineering decisions MUST be traced to authority. When authorities conflict, the higher level wins:

1. applicable law, contractual obligations, security boundaries, privacy commitments, and
   irreversible user-safety constraints;
2. explicit product-owner rulings and accepted product canon;
3. public compatibility contracts: wire formats, storage formats, documented APIs, CLI behavior,
   migration promises, and released semantics;
4. language and platform specifications, including the Go specification, Go memory model,
   selected Go release documentation, and standard-library contracts;
5. accepted repository architecture decisions, design documents, and ownership records;
6. this engineering standard and the repository's project profile;
7. canonical language style guidance, including Effective Go, Go Code Review Comments, and the
   Google Go Style Guide and Style Decisions;
8. reference-codebase precedents;
9. tool recommendations and linter defaults;
10. individual preference.

A lower authority MUST NOT be used to overrule a higher authority.

Reference projects are case studies, not product canon. A pattern from CockroachDB, etcd,
Kubernetes, Temporal, Prometheus, containerd, NATS, OpenTelemetry Collector, Caddy, Zed, or any
other project is admissible only after its assumptions are shown to match this repository.

## 4. Core doctrine

The governing rule is:

> Use the fewest concepts necessary to describe the product world honestly. One term has one
> meaning, one material behavior has one traceable authority, one source of truth is authoritative
> at a time, and no state, error, cache, boundary, metric, or test may lie.

The quality order is:

1. correctness and honesty;
2. clarity;
3. simplicity;
4. maintainability;
5. consistency;
6. concision;
7. performance, after a requirement or measurement establishes its importance.

This order does not permit avoidable inefficiency. It prohibits sacrificing correctness or clarity
for speculative optimization.

Semantic rigor does not mean adding layers, interfaces, factories, domain theater, or patterns.
Concrete structs, ordinary functions, explicit data flow, and clearly owned packages are preferred.

Implementation convenience, test convenience, current database shape, framework defaults, or the
preferences of the author MUST NOT determine product behavior.

## 5. Risk and applicability profile

Every repository MUST declare a risk class in `PROJECT_PROFILE.md`.

### 5.1 Base classes

- **R0 — library or deterministic build-time tool**: no long-lived process, no durable user data,
  no independent network authority, and no irreversible external side effects.
- **R1 — local application or CLI**: interacts with local files, local credentials, a user session,
  subprocesses, or local databases.
- **R2 — networked service or shared control plane**: accepts remote input, performs egress,
  handles multi-user state, or exposes a supported remote API.
- **R3 — critical data, security, infrastructure, or distributed system**: correctness failures may
  cause data loss, privilege bypass, private-data exposure, split-brain behavior, broad outages,
  or irreversible publication.

### 5.2 Capability flags

The profile MUST also mark relevant flags:

- `public-api`
- `durable-storage`
- `network-ingress`
- `network-egress`
- `credentials`
- `personal-data`
- `irreversible-actions`
- `concurrent`
- `distributed`
- `plugin-platform`
- `browser-ui`
- `desktop-ui`
- `agent-facing`
- `cross-platform`
- `paid-provider`
- `regulated-data`

Requirements are activated by actual risk, not repository size. A ten-file credential tool may be
R3. A million-line code generator may remain R0.

## 6. Required repository control plane

Every active repository MUST have, directly or by an organization-level inherited policy:

- `README.md`: product purpose, supported use, quick start, limitations, support path;
- `LICENSE` or an explicit private-repository rights statement;
- `CONTRIBUTING.md`: development and review workflow;
- `PROJECT_PROFILE.md`: applicability and verification profile;
- an architecture entry point, normally `ARCHITECTURE.md` or `docs/architecture/README.md`;
- an accepted-decision location, normally `docs/decisions/`;
- `SECURITY.md` for any repository that is public, handles input, stores data, uses credentials,
  or ships artifacts;
- an explicit maintainer and ownership record;
- a canonical verification entry point;
- release and compatibility policy when artifacts or APIs are distributed;
- an agent instruction file when coding agents are used.

The agent instruction file MUST remain high-signal. It SHOULD contain traps, local commands,
non-obvious invariants, and review requirements. It SHOULD NOT duplicate rapidly changing package
maps or architecture that can be learned from source and maintained documentation.

A new root-level agent rule MUST be:

1. non-obvious to a competent contributor;
2. repeatedly encountered or demonstrably high-risk;
3. specific enough to change an action;
4. reviewed independently of the feature that discovered it.

Module-, package-, or directory-specific rules belong as close as practical to their scope.

## 7. Required concept definition

Before non-trivial implementation, the builder MUST define a concept card:

```text
Concept:
Non-concept:
User need:
User-visible promise:
Single semantic owner:
Decision authority:
Source of truth:
Captured capabilities:
Derived projections:
Legal states:
Impossible states and why:
Failure classes:
Retry semantics:
Time and freshness semantics:
Concurrency semantics:
Compatibility surface:
Irreversible side effects:
Privacy and egress implications:
Invariant that must never fail:
Acceptance evidence:
```

The card MAY be short for a small change. It MUST still settle every material item.

If two reasonable product behaviors remain, the builder MUST:

- mark the issue `NEEDS-OWNER`;
- stop implementing the disputed behavior or irreversible side effect;
- record options and consequences without disguising preference as authority;
- continue only reversible investigation, test construction, or refactoring that is valid under all
  remaining options.

A package, interface, schema, cache, flag, or API MUST NOT be created merely to postpone
understanding.

## 8. Semantic consistency

### 8.1 One term, one meaning

A core term MUST mean the same thing in:

- product canon and decisions;
- package, type, function, method, and field names;
- SQL tables and columns;
- configuration;
- CLI flags, output, and exit codes;
- JSON, HTTP, and RPC contracts;
- UI copy;
- logs, metrics, traces, and diagnostics;
- fixtures, fakes, and tests.

A state named `published`, for example, MUST NOT mean an externally committed publication in
product canon while meaning only "the user clicked Publish" in code.

When a name is dishonest, the model or name SHOULD be corrected. A comment is not a permanent
substitute for a truthful API.

### 8.2 One semantic owner

Each core behavior MUST have one authoritative semantic definition and one named owner. The same
invariant MAY be enforced redundantly at trust boundaries, type constructors, or storage constraints
when those enforcement points are mechanically consistent with that definition and do not become
independent policy.

A concept may legitimately appear in multiple packages as:

- a domain implementation;
- a read-only projection;
- a transport encoding;
- a persistence mapping;
- a UI presentation;
- test evidence.

Those forms MUST NOT independently reinterpret the concept's rules. "One owner" means one
semantic authority, not necessarily one physical file or package.

UI, CLI, SQL, migration code, and tests MUST NOT each carry a divergent copy of the same state
machine. A test fake MUST NOT become a hidden second product specification.

### 8.3 Truth, capability, and projection

Every durable or transient value MUST be classifiable as one of:

- source of truth;
- capability or authority token;
- derived state;
- disposable cache;
- read model or UI projection;
- transport envelope;
- diagnostic;
- historical audit record.

A derived value MUST NOT silently become authorization. A cache MUST NOT be presented as current
truth. A stale snapshot MUST NOT authorize an irreversible action. An audit record MUST NOT be
mutated as though it were live state.

### 8.4 Decision provenance

Every load-bearing rule MUST be classified as:

- **REAL-OBSERVED**: directly observed in the real system, protocol, or data;
- **EXPLICIT-RULING**: explicitly decided by an authorized owner;
- **CONTRACT-INHERITED**: required by an existing public contract or higher authority;
- **CANON-DERIVED**: uniquely implied by accepted authority;
- **NEEDS-OWNER**: multiple reasonable behaviors remain.

The following are prohibited:

- citing a plan sentence as authority for the plan's own invention;
- deriving authorization from a field merely existing;
- reconstructing product policy from current tests or accidental implementation;
- treating a roadmap label or issue priority as a product ruling;
- presenting personal caution, preference, or implementation ease as derived behavior;
- citing a decision that covers only an adjacent case;
- silently modifying an existing ruling in an implementation plan;
- assigning a decision an earlier date than the actual ruling.

## 9. State and failure semantics

### 9.1 Relevant distinctions

A system MUST distinguish all materially different reachable states. Common distinctions include:

- `off`: not requested or intentionally disabled;
- `not-applicable`: requested, but no operation applies to the input;
- `unavailable`: applicable, but a required capability is missing;
- `empty`: successfully answered with zero results;
- `partial`: some results are trustworthy and some work failed;
- `unanswerable`: the system cannot answer honestly;
- `invalid`: caller input or current state is illegal;
- `conflict`: a concurrent or authoritative state prevents the operation;
- `stale`: data exists but no longer represents current authority;
- `not-found`: the identified resource does not exist in the relevant authority;
- `provider-failed`: an external dependency failed;
- `cancelled`: the caller or system intentionally stopped work;
- `deadline-exceeded`: work did not complete within the promised bound;
- `internal-error`: the application violated an invariant or could not safely form the action.

A repository MUST NOT invent irrelevant states merely to satisfy this list. It MUST NOT collapse
materially distinct states into an empty list, `false`, a generic error, HTTP 500, or one ambiguous
status code.

### 9.2 Complete state matrix

Every material reachable cross-product cell MUST define:

```text
preconditions
→ state
→ result shape
→ status or exit code
→ human diagnostic
→ machine-readable reason
→ retryability
→ recovery action
→ side effects
→ authority used
→ freshness
→ acceptance evidence
```

An impossible cell MUST explain why it cannot be constructed and MUST have an invariant or type
constraint that prevents it. A comment alone is weak evidence.

### 9.3 Honest fault ownership

Every failure MUST identify, when knowable, whether the fault belongs to:

- the caller;
- local configuration;
- source authority;
- the application;
- an external provider;
- an operator-controlled dependency;
- an unknown party.

Unknown ownership MUST remain unknown. A failure MUST also state whether:

- partial results remain trustworthy;
- side effects occurred;
- retry is safe;
- retry may duplicate work;
- user intervention is required;
- rollback or compensation is available.

Diagnostics MUST NOT leak credentials, tokens, private content, complete queries, sensitive paths,
or untrusted provider response bodies.

## 10. Architecture and package ownership

### 10.1 Organize around capabilities

Packages SHOULD be organized around stable product capabilities and responsibilities, not a
preselected horizontal pattern.

Good package names usually describe what the product does:

```text
search
snapshot
lifecycle
schema
render
publish
lease
queue
```

Generic buckets such as the following require exceptional justification:

```text
services
repositories
models
handlers
adapters
infrastructure
common
utils
helpers
managers
processors
```

These names are not universally forbidden. They are presumptively suspicious because they often
hide mixed ownership or describe assembly technique instead of product meaning.

For a questionable package or type, reviewers ask:

1. What real product capability or boundary does it own?
2. Is `pkg.Name` natural at the call site?
3. Does it duplicate the package name or another concept?
4. Is it only a design-pattern label?
5. Would renaming it to `Thing` barely reduce understanding?
6. If deleted, does a named production capability disappear, or only test wiring?

### 10.2 Dependency direction

Dependency direction MUST be understandable from the product model.

- Core semantics MUST NOT import UI, transport, storage-driver, or command packages.
- Composition roots MAY import concrete implementations.
- Transport and persistence packages MAY translate; they MUST NOT redefine domain policy.
- Cycles are prohibited.
- Side-effecting capabilities MUST be passed explicitly or constructed in a visible composition root.
- Service locators and mutable global dependency registries are prohibited unless a platform-level
  plugin system is itself the reviewed product capability.

Dependency rules SHOULD be mechanically checked when the repository is large enough that review
alone is unreliable.

### 10.3 Command packages

`cmd/...` packages MUST own only process concerns:

- configuration loading and validation;
- dependency construction and hand-off;
- signal handling and graceful shutdown;
- command dispatch;
- process exit status.

They MUST NOT reimplement domain rules, persistence semantics, authorization, or error taxonomy.

### 10.4 Public and internal API surface

- Public API is a compatibility commitment and MUST be deliberately reviewed.
- Implementation details SHOULD remain unexported or under `internal/`.
- Exporting for tests is prohibited.
- A public symbol MUST have documentation that states behavior, ownership, concurrency safety,
  zero-value behavior, error semantics, and lifecycle when material.
- Public examples SHOULD compile as tests.
- Compatibility changes MUST follow the repository's versioning and deprecation policy.

### 10.5 Interfaces and abstraction

- Concrete types are the default.
- Interfaces SHOULD be defined by consumers at the smallest useful boundary.
- A single implementation does not, by itself, justify an interface.
- A test double does not, by itself, justify a production interface.
- Interfaces MUST represent substitutable behavior, not a list of every method on an implementation.
- "Future flexibility" without a named second behavior or compatibility need is not sufficient.
- A configuration struct MUST NOT become a bag of unrelated dependencies.

### 10.6 Composition and extension

A plugin or module system is justified only when third-party or separately versioned components are
a product requirement.

When a plugin system exists, it MUST define:

- namespace and identity;
- lifecycle;
- configuration schema;
- capability and permission boundary;
- supported data types;
- stability level;
- compatibility and deprecation policy;
- failure isolation;
- resource limits;
- observability;
- secure loading and provenance.

Caddy's module model and OpenTelemetry Collector's component model are useful references for real
extension platforms. They are not reasons to make an ordinary application "all plugins."

### 10.7 Generated code

Generated code MUST have:

- an owning generator and version;
- a deterministic generation command;
- clear checked-in or build-time policy;
- generated-file markers;
- drift detection in verification;
- review of the source schema or generator input, not only generated output.

Hand-editing generated output is prohibited unless the generator contract explicitly allows it.

### 10.8 Architecture decisions

A design record is required when a change:

- creates or removes a major concept;
- changes authority or source of truth;
- adds a durable format, public API, protocol, or plugin boundary;
- changes consistency, durability, privacy, or compatibility semantics;
- introduces cgo, `unsafe`, reflection-heavy infrastructure, or a major framework;
- creates a new cross-cutting dependency direction;
- changes an irreversible side-effect boundary.

The decision MUST include context, constraints, alternatives, consequences, migration, evidence,
and conditions under which it should be revisited.

## 11. Go engineering standard

This section is mandatory for Go code. Repository-local rules MAY be stricter, but MUST preserve
Go's semantics and the authority hierarchy in this standard.

### 11.1 Formatting and mechanical consistency

- All Go source MUST be `gofmt`-clean.
- Import grouping and ordering MUST be deterministic. A repository MAY standardize on `goimports`.
- Generated Go source MUST also be formatted by its generator.
- Formatting-only changes SHOULD NOT be mixed with behavioral changes unless inseparable.
- A linter's preferred spelling MUST NOT override clearer domain language or a public compatibility
  contract.

### 11.2 Packages and identifiers

- Package names MUST be short, lower-case, and describe the capability they provide.
- Package names MUST NOT contain `util`, `common`, `base`, or `misc` without an approved exception.
- Exported names MUST read naturally as `package.Symbol`; package-name stutter SHOULD be removed.
- Initialisms MUST be written consistently with Go convention, such as `ID`, `URL`, `HTTP`, and
  `JSON`, except where an external compatibility contract fixes another spelling.
- Receiver names SHOULD be short and consistent for a type. They MUST NOT be `this` or `self`.
- A name MUST communicate role or product meaning. Type information already visible from context
  SHOULD NOT be repeated in the name.
- Boolean names SHOULD read as predicates. Public APIs SHOULD avoid ambiguous boolean arguments;
  use a named type or options struct when the alternatives are not obvious at the call site.
- Filenames SHOULD describe the contained capability. A filename that needs several unrelated
  nouns is a signal that responsibilities may be mixed.

### 11.3 Comments and documentation

- Comments MUST explain contracts, invariants, ownership, non-obvious reasons, compatibility, or
  risk. They MUST NOT narrate syntax.
- Every exported identifier MUST have a useful documentation comment unless the repository records
  a narrow exception for generated or self-evident declarations.
- Package documentation MUST explain the package's purpose, ownership, and important invariants.
- Public examples SHOULD compile and run as tests.
- A comment that contradicts code is a defect. Comments MUST be updated in the same change as the
  behavior they describe.
- TODOs MUST include an owner or issue identifier and a trigger or exit condition. A vague TODO is
  not a plan and MUST NOT hide acceptance debt.

### 11.4 API shape and zero values

- The default API shape is an ordinary function or concrete type.
- A constructor is required when construction validates invariants, acquires resources, captures
  authority, or prevents an invalid zero value. It MUST NOT exist merely to mirror another
  language's class pattern.
- A useful zero value SHOULD work when it is natural and honest. An unusable zero value MUST be
  documented and guarded.
- Functional options are appropriate only for independent optional dimensions with stable
  semantics. They MUST NOT hide required dependencies, permit illegal combinations, or replace a
  small explicit configuration struct.
- Inputs and outputs MUST make nil-versus-empty semantics explicit where callers can observe the
  difference.
- Variadic parameters MUST NOT obscure ownership, allocation, or validation semantics.
- Public structs SHOULD NOT expose fields that prevent future invariant enforcement unless field
  mutability is an intentional compatibility contract.
- APIs SHOULD be synchronous unless asynchronous execution is itself a product capability. A
  caller can start a goroutine; a library cannot recover ownership clarity after hiding one.

### 11.5 Interfaces

- Interfaces MUST be defined where they are consumed unless a public provider contract requires
  otherwise.
- Interfaces SHOULD contain the smallest behavior needed by the consumer.
- An interface MUST NOT be introduced solely to mock an implementation.
- An implementation MUST NOT return an interface merely to hide a concrete type without a real
  substitution or compatibility requirement.
- Compile-time interface assertions MAY document intentional conformance.
- Interface methods MUST have coherent lifecycle, cancellation, concurrency, and error semantics.
- A broad interface used by unrelated consumers MUST be split or justified through an architecture
  decision.

### 11.6 Context and cancellation

- `context.Context` MUST be the first parameter after a receiver.
- Context MUST NOT be stored in a struct, except when a framework contract makes request-scoped
  storage unavoidable and the exception is documented.
- A caller-provided context MUST be propagated through blocking I/O and long-running work.
- Libraries MUST NOT replace a caller's context with `context.Background()` across a cancellable
  boundary.
- Context values MUST contain request-scoped metadata, not optional function parameters or mutable
  service dependencies.
- Context keys MUST use unexported, collision-safe types.
- Cancellation MUST have documented side-effect semantics: whether work stopped, whether a commit
  occurred, whether retry is safe, and whether partial results are trustworthy.
- A goroutine started for request work MUST have a termination path even when the caller cancels.

### 11.7 Error semantics

- Expected operational failures MUST be returned as errors, not panics.
- Panic is reserved for impossible internal states, violated programmer contracts, or process
  startup conditions where continuing would be dishonest. Library code SHOULD rarely panic.
- Error text SHOULD be lower-case and MUST NOT end with redundant punctuation when it will be
  wrapped.
- Errors MUST add useful context once at the layer that owns that context and preserve identity
  with `%w` when callers need `errors.Is` or `errors.As`.
- A layer MUST NOT both log and return the same error unless it owns an additional operational event
  that cannot be represented by the caller.
- Sentinel and typed errors MUST be deliberate compatibility contracts. They MUST NOT expose
  implementation details accidentally.
- Error classification MUST be stable enough for callers to choose recovery without parsing text.
- Errors MUST identify partial-result validity and retry safety when those facts are material.
- Sensitive values, credentials, private content, full SQL strings, tokens, or untrusted provider
  bodies MUST NOT appear in error text or wrapping chains.
- Swallowing an error, replacing it with success, or changing an expected result merely to make a
  test pass is prohibited.

### 11.8 Resource ownership

- The code that acquires a resource MUST make ownership and release responsibility explicit.
- Files, response bodies, rows, transactions, sockets, timers, tickers, processes, and goroutines
  MUST have a bounded lifecycle.
- `defer` SHOULD be used when it makes cleanup reliable and its cost is not material. Defers in hot
  loops MUST be assessed rather than mechanically retained or removed.
- Cleanup errors that can affect durability or correctness MUST be observed and surfaced.
- A function MUST NOT retain caller-owned mutable buffers or slices beyond the documented lifetime.
- Returning pooled or aliased memory MUST be documented and guarded by tests.

### 11.9 Goroutines, channels, and shared state

- Every goroutine MUST have a named owner, stop condition, and join or process-lifetime rationale.
- Unbounded goroutine creation is prohibited.
- Fan-out, queues, and buffering MUST have explicit limits and backpressure or shedding behavior.
- The sender or producer that owns a channel is normally responsible for closing it. Receivers MUST
  NOT close a channel they do not own.
- Channel direction SHOULD be expressed in public signatures.
- Channels SHOULD coordinate ownership transfer or event flow. A mutex is often clearer for shared
  state; channels MUST NOT be selected merely because they appear more idiomatic.
- A send that may block forever MUST have a bounded or cancellable path.
- Background errors MUST have an owner and observable path; they MUST NOT disappear in a goroutine.
- Tests MUST prove goroutine termination for lifecycle-sensitive components.

### 11.10 Memory model, races, and synchronization

- Correct concurrent code MUST be data-race free under the Go memory model.
- Synchronization MUST establish the required happens-before relationship; timing observations are
  not synchronization.
- `sync.Once`, `sync.Mutex`, `sync.RWMutex`, `sync.Cond`, atomics, and channels MUST be chosen for
  semantics, not perceived prestige.
- `RWMutex` MUST be justified by measured or clearly dominant read contention; it is not a default
  upgrade from `Mutex`.
- Atomics MUST document the invariant and memory-ordering argument they implement.
- Copying values containing mutexes, atomics, `sync.Once`, or other no-copy state is prohibited.
- Nested locks MUST have a documented acquisition order close to the code. For systems with many
  locks, the repository MUST maintain an authoritative lock-order document and tests or analysis
  where feasible.
- Race-detector coverage MUST include representative production composition. A race-free unit-only
  subset is not sufficient evidence for a concurrent application.

### 11.11 Time, timers, and randomness

- Durations MUST use `time.Duration`; APIs MUST NOT encode durations as unexplained integers.
- Deadline, timeout, interval, timestamp, and elapsed duration MUST be named distinctly.
- Elapsed-time logic SHOULD use monotonic time carried by `time.Time` where available.
- Tests MUST NOT depend on real sleeps when a deterministic clock, synchronization point, or
  `testing/synctest` can express the behavior.
- Fake clocks MUST preserve the production clock contract and MUST NOT become an alternate product
  scheduler.
- Timers and tickers MUST be stopped or allowed to expire with a documented lifecycle.
- Security-sensitive randomness MUST use `crypto/rand` or a reviewed cryptographic primitive.
- Pseudo-random sources used for deterministic tests MUST be explicitly seeded and isolated.

### 11.12 Collections, ownership, and encoding

- Map iteration order MUST NOT be relied upon.
- Deterministic artifacts, logs, snapshots, tests, and wire output MUST sort unordered data at the
  owned boundary.
- Slice capacity reuse and append aliasing MUST be understood where values cross ownership
  boundaries.
- `nil` maps and slices MUST be handled intentionally. JSON null-versus-empty output MUST be a
  product decision when observable.
- Parsers and decoders MUST place limits on size, depth, count, recursion, and allocation according
  to risk.
- Unknown fields MUST be rejected, preserved, or ignored according to an explicit compatibility
  policy; permissiveness is not automatically safer.

### 11.13 Reflection, generics, unsafe, cgo, and code generation

- Generics SHOULD remove duplication or strengthen type safety without hiding domain behavior.
- Reflection-heavy infrastructure requires a demonstrated benefit over explicit code and stronger
  tests at schema, conversion, and failure boundaries.
- `unsafe` and cgo require an architecture decision, owner, threat analysis, platform matrix,
  benchmark where performance is the rationale, and focused tests.
- A generated implementation SHOULD be preferred over runtime reflection when the schema is stable
  and generation materially improves safety or performance.
- Clever type machinery that makes ordinary control flow difficult to inspect is below this
  standard even when technically correct.

### 11.14 Initialization and global state

- Package initialization MUST be deterministic, fast, and free of network I/O, filesystem writes,
  hidden goroutines, or environment-dependent product decisions.
- `init` SHOULD be limited to unavoidable registration or validation. Explicit construction is
  preferred.
- Mutable package-global state is prohibited unless it represents a process-wide invariant with
  explicit synchronization, reset semantics for tests, and an architecture justification.
- Environment variables and process flags MUST be read at the process composition boundary, not
  opportunistically throughout domain packages.

### 11.15 HTTP clients and servers

Where HTTP applies:

- Clients MUST reuse transports and MUST NOT create an unbounded new client per request.
- Timeouts, cancellation, redirects, proxy behavior, TLS policy, retries, and response-size limits
  MUST be explicit at the owned boundary.
- Response bodies MUST be closed. Reuse requirements and draining behavior MUST be handled
  intentionally.
- Servers MUST define header, body, idle, and shutdown limits appropriate to their exposure.
- Request bodies MUST be bounded before decoding.
- Handler errors MUST map through one authoritative error-to-wire policy.
- User-controlled data MUST NOT be reflected into headers, HTML, logs, or redirects without the
  correct contextual validation or escaping.
- Graceful shutdown MUST define acceptance stop, in-flight completion, deadline, and forced-exit
  behavior.

### 11.16 Database use from Go

Where `database/sql` or an equivalent driver applies:

- Queries MUST use context-aware operations at cancellable boundaries.
- `Rows` MUST be closed and `Rows.Err()` MUST be checked.
- Transaction ownership MUST be explicit; a transaction MUST be committed or rolled back on every
  path.
- A context cancellation MUST NOT be assumed to prove rollback without checking the selected
  driver and database contract.
- Prepared statements, batching, and connection-pool settings MUST follow measured workload and
  database semantics rather than habit.
- SQL errors MUST be classified without parsing unstable text where a stable code exists.
- Database types, nullability, precision, and time-zone behavior MUST be mapped deliberately into
  Go types.

### 11.17 Modules, compatibility, and dependencies

- The supported Go version and platform matrix MUST be declared in the project profile.
- `go.mod` and `go.sum` drift MUST be checked in verification.
- Library releases MUST follow semantic import versioning and the repository's compatibility
  policy.
- Public packages MUST treat compatibility as a product promise, including behavior and error
  contracts, not only compilation.
- Dependencies MUST have a named need, active maintenance signal, acceptable license, security
  posture, compatibility policy, and removal strategy.
- A dependency MUST NOT be added for a trivial function that is clearer and safer to own locally.
- Vendoring, proxy, checksum-database, private-module, and reproducibility policy MUST be explicit.
- Dependency upgrades MUST be reviewed for behavior, generated drift, license, security, and
  transitive impact; a green compile is insufficient.

## 12. Product surfaces and contract design

### 12.1 General contract rule

Every externally observable surface MUST define:

```text
consumer
→ request or input contract
→ authority and validation
→ success shapes
→ failure shapes
→ partial-result semantics
→ compatibility policy
→ resource and size limits
→ side effects
→ observability
→ recovery
→ acceptance evidence
```

A contract MUST be understandable without reading the implementation.

### 12.2 HTTP, JSON, and RPC APIs

- API behavior MUST be versioned when incompatible evolution is plausible.
- Request validation MUST distinguish malformed, invalid, unauthorized, forbidden, conflicting,
  unavailable, and internal failures when those distinctions change recovery.
- Error responses MUST contain a stable machine-readable code, human-readable message safe for the
  caller, correlation identifier where applicable, retry guidance, and field-level details when
  useful.
- Clients MUST NOT need to parse prose to select recovery.
- Success responses MUST NOT hide partial failure. Partial results require explicit completeness,
  provenance, and warning fields.
- Idempotency semantics MUST be defined for operations that may be retried.
- Pagination MUST define ordering, cursor stability, filtering interaction, and behavior under
  concurrent mutation.
- Unknown-field behavior, duplicate keys, numeric precision, Unicode normalization, timestamps,
  time zones, and nullability MUST be deliberate.
- Request and response limits MUST be enforced before unbounded allocation.
- Authentication proves identity; authorization decides capability. They MUST NOT be collapsed.
- Desired state and observed state SHOULD be distinct for asynchronous control-plane APIs. A
  request acceptance MUST NOT be represented as completed work.
- Deprecations MUST provide detection, migration, support window, and removal criteria.

### 12.3 CLI contracts

- CLI help MUST make the primary task discoverable from a clean installation.
- Commands and flags MUST use product language and stable semantics.
- Standard output is for requested results; standard error is for diagnostics and progress.
- Exit codes MUST be documented and stable enough for automation.
- Machine-readable output MUST be schema-controlled, free of ANSI escapes and incidental logs, and
  deterministic where ordering has no product meaning.
- A single agent-facing invocation MUST reveal outcome, completeness, failure class, whether retry
  is safe, and the next supported action.
- Interactive prompts MUST have a non-interactive alternative or explicitly declare that automation
  is unsupported.
- Destructive actions MUST identify scope, support preview where feasible, and require explicit
  confirmation or an action-scoped capability. A generic `--force` MUST NOT silently combine
  unrelated waivers.
- Shell completion, examples, and error suggestions MUST not invent unsupported behavior.
- Configuration precedence and environment interaction MUST be visible in help or diagnostics.

### 12.4 Browser and desktop UI

- UI state MUST be a projection of authoritative product state, not an alternate source of truth.
- Semantic HTML is the default for web UI. CSS and platform APIs SHOULD be used before custom
  JavaScript when they satisfy the requirement.
- Every interactive operation MUST support keyboard use, visible focus, accessible naming,
  meaningful landmarks, and correct disabled and loading semantics.
- No-JavaScript behavior MUST be truthful: either the core task remains usable or the limitation is
  stated before the user commits work.
- Browser history, reload, deep links, back/forward navigation, and interrupted operations MUST be
  tested where applicable.
- Narrow viewports, zoom, long content, localization expansion, non-ASCII input, reduced motion,
  high contrast, and screen-reader semantics MUST be considered.
- Loading, empty, unavailable, stale, partial, invalid, unauthorized, offline, and internal-error
  states MUST be visibly distinguishable when relevant.
- Optimistic UI MUST have rollback and conflict semantics; it MUST NOT claim durable success before
  authority confirms it.
- Visual or interaction regressions in stable surfaces SHOULD have browser automation, screenshot,
  or structured UI tests. Screenshot tests MUST be reviewed for semantic value, not blindly
  accepted.
- Performance-critical interaction paths MUST have product-specific response budgets and traces.

### 12.5 SQL schema and durable storage

- Schema names MUST use product language and preserve one-term-one-meaning.
- SQL values MUST use parameters. Dynamic identifiers, ordering, or clauses require an allowlist,
  typed query construction, or another reviewed mechanism that cannot turn untrusted data into SQL.
- The project MUST choose and document direct SQL, generated queries, a query builder, or an ORM
  according to schema safety, reviewability, performance, and operational needs. A framework MUST
  NOT be added merely because migrations or more queries may exist someday.
- Query result shapes and scans MUST fail visibly on incompatible schema change; generated query
  code and schema inputs MUST participate in drift verification when selected.
- Primary keys, foreign keys, uniqueness, checks, nullability, defaults, and cascade behavior MUST
  encode real invariants where the datastore can enforce them.
- Every index MUST have a query, constraint, ordering, or operational reason.
- Derived columns and denormalization require an owner, recomputation rule, staleness semantics, and
  contradiction test.
- Transaction boundaries MUST match product atomicity. A transaction MUST NOT be split for
  implementation convenience when users observe an impossible intermediate state.
- Migration policy MUST define forward application, rollback or roll-forward, compatibility window,
  resumability, locking, failure recovery, and old-binary behavior.
- Destructive migration MUST require backup or rebuild semantics appropriate to data value.
- Schema evolution MUST be exercised against representative existing data, not only a fresh
  database.
- Atomic generation switching SHOULD use staging and a final activation boundary when readers must
  never observe partial publication.
- Stale writers and handles MUST be fenced when they could publish after authority changes.
- Durability claims MUST be matched to actual fsync, transaction, replication, and storage-system
  semantics.

### 12.6 Filesystem contracts

- Paths MUST be normalized and validated for the target platform without assuming Unix-only
  semantics unless the support matrix says so.
- Untrusted paths MUST be defended against traversal, symlink substitution, case-folding surprises,
  reserved names, alternate data streams where relevant, and time-of-check/time-of-use races.
- Writes that promise atomic replacement MUST use a verified same-filesystem staging and rename
  strategy, with explicit permission and sync semantics.
- Temporary files MUST have restrictive permissions and deterministic cleanup.
- File formats MUST define encoding, newline, Unicode, size, compatibility, corruption, and partial
  write behavior.
- A local cache MUST be safe to delete and rebuild. If deletion loses authority, it is not a cache.

### 12.7 Wire formats and protocols

- Protocol ownership, version negotiation, framing, limits, unknown-field policy, and downgrade
  behavior MUST be documented.
- Exact bytes that carry security, privacy, signature, compatibility, or billing meaning MUST have
  golden or recording evidence.
- Parsers MUST be fuzzed and bounded at trust boundaries.
- Encoders MUST be deterministic when signatures, hashes, caches, diffs, or reproducibility depend
  on output.
- Compatibility tests MUST use released or captured fixtures, not only two copies of the current
  implementation.
- A protocol error MUST NOT be converted to an empty successful response.

### 12.8 Configuration

- Configuration MUST have one authoritative schema and validation path.
- Defaults MUST be safe, documented, and testable.
- Precedence among flags, files, environment, remote configuration, and built-in defaults MUST be
  explicit and observable.
- Configuration parsing and normalization MUST occur at a controlled boundary. Domain packages
  SHOULD receive validated values, not repeatedly read global environment.
- Unknown keys MUST follow an explicit compatibility policy.
- Secrets MUST be distinguishable from ordinary configuration and redacted from display, logs,
  diffs, and diagnostics.
- Dynamic reload MUST define atomicity, validation, failure rollback, and which fields require
  restart.
- A generic configuration representation MAY be used at the ingress boundary, but normalized
  internal state SHOULD use typed product concepts.

### 12.9 Plugins, modules, and component stability

A repository with a real extension ecosystem MUST publish stability per component or API, such as:

- experimental;
- alpha;
- beta;
- stable;
- deprecated;
- unmaintained.

Each level MUST define compatibility, support, testing, security review, and removal obligations.
An extension MUST NOT imply that the host guarantees behavior it does not test. Experimental
components MUST be visibly experimental in documentation and machine-readable metadata.

## 13. Time, concurrency, retries, and distributed correctness

### 13.1 Event-first review

Review MUST analyze the sequence of events, not only final values:

```text
capture identity and authority
→ validate preconditions
→ begin work
→ perform cancellable computation or I/O
→ receive external result
→ revalidate identity and authority
→ enter commit boundary
→ commit durable state
→ publish or expose the result
→ emit evidence
```

For every side effect, reviewers MUST ask what happens if cancellation, process death, authority
change, duplicate delivery, timeout, or competing work occurs between each adjacent pair.

### 13.2 Final-boundary validation

- Authorization, freshness, generation identity, consent, quota, and privacy conditions MUST be
  revalidated at the final irreversible send, commit, activation, or publication boundary when they
  can change during work.
- An upstream guard is not proof if another path can reach the final boundary.
- Final boundaries SHOULD be structurally centralized so bypasses are detectable.
- A stale worker MUST be unable to commit merely because it began while authorized.
- The committed record SHOULD contain enough provenance to audit which authority and input
  generation authorized the action.

### 13.3 Cancellation and partial completion

- Cancellation MUST be treated as a request, not proof that remote or durable work stopped.
- Operations MUST define cancellation before side effects, during side effects, after commit, and
  during response delivery.
- A caller MUST be able to distinguish "not performed," "may have been performed," and "performed
  but response lost" when the distinction is knowable and material.
- Compensating actions MUST be explicit product behavior, not assumed transaction rollback.

### 13.4 Retries and duplicate work

- Retries MUST be bounded, classified by failure, cancellable, observable, and subject to a total
  deadline or attempt budget.
- Backoff SHOULD include jitter for shared remote systems.
- Permanent errors MUST NOT be retried automatically.
- Non-idempotent actions require an idempotency key, deduplication contract, transactional outbox,
  or another reviewed duplicate-control mechanism.
- Retry MUST NOT silently resend credentials, private content, or billable work beyond the user's
  authorized scope.
- Retry state MUST survive process restart when the product claims durable delivery.
- A retry library MUST NOT decide product policy; the owning capability decides which failures are
  retryable.

### 13.5 Deterministic concurrency tests

- Tests SHOULD use barriers, channels, fake clocks, hooks at owned boundaries, or `synctest` rather
  than sleeps and scheduler luck.
- A deterministic hook MUST expose a real production transition; it MUST NOT create a test-only
  alternate state machine.
- Concurrency tests MUST assert forbidden observations and side effects, not merely eventual
  success.
- Deadlock and leak-prone code SHOULD have bounded test deadlines and goroutine diagnostics.
- Flaky concurrency tests MUST be fixed or quarantined with an owner and release gate. Retrying a
  test until green is not evidence.

### 13.6 Distributed systems

Repositories with the `distributed` flag MUST define:

- the consistency model per operation;
- identity and epoch or generation model;
- leader, lease, quorum, or ownership semantics;
- read and write authority;
- clock assumptions;
- partition and message-delay behavior;
- duplicate, reordering, and replay handling;
- snapshot, log, checkpoint, and compaction semantics;
- membership and rolling-upgrade compatibility;
- recovery from crash, disk loss, and partial network failure;
- durability and data-loss envelope;
- operator-visible health and repair procedures.

A distributed correctness claim MUST have more than happy-path unit tests. Depending on risk, the
repository MUST use deterministic simulation, model or property tests, history checking,
linearizability or serializability checking, fault injection, process-level integration,
randomized robustness testing, or controlled chaos.

A test that starts several in-process objects but bypasses the real transport, persistence,
configuration, or lifecycle MUST NOT certify the full distributed system.

### 13.7 Leases, fencing, and generations

- A lease is not authority after expiry, revocation, or generation change.
- Long-running work MUST carry a generation, epoch, token, or equivalent fence through commit when
  stale publication is possible.
- Wall-clock time alone SHOULD NOT be used as a fence when monotonic sequence or authoritative
  generation is available.
- Readers MUST define whether they can observe staging, active, and previous generations.
- Purge, rebuild, activation, and rollback MUST have an explicit interleaving model.

### 13.8 Lock-order and blocking discipline

- Code MUST document whether a call may block, perform I/O, acquire another lock, invoke user code,
  or call a plugin while holding a lock.
- Untrusted callbacks, network I/O, filesystem I/O, and potentially blocking channel sends SHOULD
  NOT occur while holding a shared lock.
- A repository with nested locks MUST maintain a lock-order invariant and update it in the same
  change that adds or changes an acquisition path.
- Watched-red or static evidence SHOULD prove critical lock-order contracts where practical.

## 14. Security, privacy, and supply-chain quality

Security and privacy are product semantics. They MUST be verified at the boundary that performs the
real action, not only in an upstream helper.

### 14.1 Threat and trust model

Repositories with ingress, egress, credentials, personal data, plugins, durable storage, or
irreversible actions MUST maintain a threat and trust model that identifies:

- protected assets;
- actors and identities;
- trust boundaries;
- accepted input sources;
- authority and capability grants;
- egress destinations;
- durable and transient data locations;
- attacker-controlled values;
- abuse and denial-of-service paths;
- third-party dependencies and operators;
- detection, response, and recovery assumptions.

The model MAY be concise, but it MUST be current enough to guide design and test selection.

### 14.2 Least authority and capability ownership

- Components MUST receive the minimum authority needed for the specific operation.
- Credentials, filesystem roots, network clients, signing keys, and publication handles SHOULD be
  action-scoped or capability-scoped rather than globally available.
- A read-only operation MUST NOT receive a write-capable dependency without a reviewed reason.
- Authorization MUST be enforced at every independently reachable privileged boundary.
- UI visibility, cached metadata, caller-supplied roles, and object possession MUST NOT be treated
  as authorization.
- Privilege changes and capability delegation MUST be auditable.

### 14.3 Egress control

Every external-data send MUST have:

- an owning capability;
- a declared destination class;
- explicit user or product authority;
- final-boundary revalidation;
- size and content limits;
- retry and redirect policy;
- timeout and cancellation semantics;
- redaction and no-log policy;
- recording or test evidence of exact behavior;
- a structural test or analysis capable of detecting alternate send paths.

Generic network helpers MUST NOT become unreviewed egress escape hatches. DNS, redirects, proxies,
subprocesses, SDK telemetry, crash reporters, and plugin transports are part of the egress model.

### 14.4 Input validation and output encoding

- Input MUST be validated according to the grammar and authority of the destination operation.
- Validation and contextual output encoding are distinct. One MUST NOT be used as a substitute for
  the other.
- Parsers MUST be bounded against excessive size, depth, count, expansion, and algorithmic cost.
- Canonicalization MUST occur before security comparison when multiple representations are
  equivalent.
- HTML, SQL, shell, path, URL, header, log, and template contexts MUST use context-appropriate safe
  construction. String concatenation across a trust boundary requires exceptional justification.
- Redirects, hostnames, IP ranges, callback URLs, and archive extraction require dedicated SSRF,
  traversal, and rebinding analysis where applicable.

### 14.5 Secrets and credentials

- Secrets MUST NOT be committed to source, fixtures, generated artifacts, logs, crash reports,
  telemetry, error chains, or screenshots.
- Secret storage, retrieval, rotation, revocation, expiry, and incident response MUST be defined.
- Secret values SHOULD have types or APIs that discourage accidental formatting and logging.
- Credentials MUST NOT be copied into broader configuration structures than needed.
- Test credentials MUST be clearly non-production and constrained.
- Diagnostic modes MUST preserve redaction guarantees.
- A redaction claim MUST have mutation or recording evidence at the final serialization and logging
  boundaries.

### 14.6 Personal and sensitive data

Repositories with `personal-data` or `regulated-data` MUST maintain a data inventory covering:

- field or content class;
- purpose and legal or product authority;
- source;
- collection trigger;
- storage location;
- encryption and access boundary;
- retention and deletion policy;
- export and portability behavior;
- third-party recipients;
- logs, metrics, traces, backups, caches, and derived copies;
- owner and incident path.

The system MUST collect and retain only data required for the declared purpose. Derived artifacts,
backups, indexes, telemetry, and caches are part of deletion and retention semantics.

Privacy-sensitive behavior MUST be off or minimally invasive by default unless explicit authority
establishes another default. Consent MUST be specific enough to cover the actual data and
destination.

### 14.7 Logging, diagnostics, and telemetry safety

- Logs MUST be designed as data products with schema, severity, retention, and access expectations.
- Untrusted text MUST be bounded and encoded to prevent log injection or terminal control abuse.
- Identifiers SHOULD be pseudonymous or minimized where full identity is unnecessary.
- High-cardinality or sensitive labels MUST NOT be placed in metrics.
- Trace attributes MUST follow the same privacy rules as logs.
- Debug endpoints and profiling data MUST be authenticated or restricted according to exposure.
- Telemetry MUST not silently expand egress or data collection during an upgrade.

### 14.8 Filesystem and archive safety

- Archive extraction MUST reject paths outside the destination root after canonicalization.
- Symlink and hard-link behavior MUST be explicitly controlled.
- Permissions MUST be least-privilege and created atomically where possible.
- Temporary directories and socket paths MUST not be predictable in a way that permits substitution.
- Ownership and mode preservation during copy, install, or restore MUST be intentional.
- Cleanup MUST not follow attacker-controlled links or delete outside the owned root.

### 14.9 Dependency and build-chain security

- New dependencies MUST be reviewed for necessity, provenance, maintainership, license, known
  vulnerabilities, transitive cost, release practice, and capability surface.
- The repository MUST run a Go-aware vulnerability check, such as `govulncheck`, on the relevant
  build graph.
- Static security analysis MAY supplement review, but findings MUST be triaged semantically.
- Suppressions MUST be narrow, local, reasoned, and owned. A repository-wide blanket suppression is
  prohibited.
- CI workflows, actions, images, compilers, generators, and release tools MUST be versioned or
  pinned according to the project's threat model.
- Generated artifacts MUST be reproducible or have signed provenance sufficient to explain their
  source.
- Public R2 and R3 releases SHOULD produce dependency or SBOM information and artifact checksums or
  signatures according to their ecosystem.
- Build and release credentials MUST be scoped, short-lived where possible, and unavailable to
  untrusted pull-request code.

### 14.10 Security response

- `SECURITY.md` MUST state how to report vulnerabilities and what versions receive fixes.
- Security-sensitive repositories MUST define severity ownership, embargo handling, patch release,
  disclosure, revocation, and user-notification paths.
- A security fix MUST include a regression lock that does not publish exploit details prematurely.
- Security exceptions MUST be reviewed by the security owner and MUST NOT waive applicable law,
  explicit privacy commitments, or known critical authority boundaries.

## 15. Performance and resource efficiency

Performance is a product property when it affects usability, cost, reliability, capacity, or
safety. It MUST be measured against a declared workload, not asserted from code appearance.

### 15.1 Performance contract

Each performance-sensitive repository or capability MUST define in `PROJECT_PROFILE.md`:

```text
workload and data shape
→ user or operator objective
→ latency or throughput percentile
→ startup or interaction budget
→ memory and allocation budget
→ CPU or I/O budget
→ concurrency and queue bounds
→ environment
→ baseline commit or release
→ regression threshold
→ escalation action
```

No universal threshold is imposed by this standard. A desktop editor, CLI, control plane, and
storage engine have different budgets. Values MUST be justified by user experience, capacity,
service objectives, or measured baseline.

### 15.2 Measurement discipline

- Benchmarks MUST use stable input, controlled setup, sufficient repetitions, and recorded
  environment.
- Results MUST identify commit, Go version, OS, architecture, hardware or runner class, and relevant
  configuration.
- Before-and-after claims SHOULD use statistical comparison rather than a single run.
- Microbenchmarks MUST NOT be used to claim end-to-end improvement unless the path contribution is
  established.
- Profiling SHOULD use Go-supported tools such as CPU, heap, block, mutex, execution trace, or
  application-specific tracing as appropriate.
- Instrumentation overhead MUST be understood for the measured path.
- A performance test that is too noisy to detect the declared regression is not a gate.

### 15.3 Resource bounds

- Queues, caches, request bodies, decoded objects, goroutines, connections, retries, worker pools,
  and in-flight operations MUST be bounded or have an explicit backpressure and shedding model.
- Cache policy MUST define key cardinality, size measurement, eviction, expiry, and invalidation.
- Memory retention through slices, maps, closures, goroutines, pools, and global registries MUST be
  reviewed on long-lived paths.
- Pools MUST demonstrate benefit under representative workload and MUST not retain sensitive or
  oversized data without controls.
- Batch size MUST balance latency, memory, fairness, and failure blast radius.
- Limits MUST produce truthful, actionable errors rather than hangs or silent truncation.

### 15.4 User-perceived responsiveness

For interactive applications:

- input handling, rendering, search, command dispatch, and opening common resources MUST have
  explicit response goals;
- long work MUST not block the UI or event loop without visible progress and cancellation;
- degraded operation MUST remain understandable;
- frame, interaction, and startup analysis SHOULD include worst-case or percentile behavior, not
  only averages;
- representative large files, long lists, Unicode, slow storage, and background load MUST be part
  of acceptance when supported.

Zed's strict interaction and frame-performance discipline is a useful reference for an editor, but
its numeric budget is not automatically the right budget for another product.

### 15.5 Performance changes

- Optimization MUST begin with a requirement or measurement.
- The change MUST preserve semantics, error behavior, cancellation, security, and observability.
- Complexity introduced for performance MUST have a benchmark, profile, or capacity result that
  proves its value and a comment or decision explaining the tradeoff.
- A fast path MUST have equivalence tests against the clear path when practical.
- Performance regressions may be accepted only through the exception process with user or capacity
  impact, owner, and trigger.

## 16. Observability and operational readiness

### 16.1 Structured events

- Operational events MUST be emitted at the layer that owns the event.
- Logs SHOULD be structured when machines consume them.
- Fields, units, severity, sampling, and cardinality MUST be consistent.
- Repeated polling or retry failures MUST avoid unbounded log amplification.
- User-facing diagnostics and operator logs have different audiences; one MUST NOT substitute for
  the other.
- Correlation identifiers SHOULD connect ingress, background work, external calls, commits, and
  publication without exposing sensitive data.

### 16.2 Metrics

- Every metric MUST have a purpose, owner, unit, type, label-cardinality analysis, and expected
  interpretation.
- Counters, gauges, histograms, and summaries MUST match the measured semantics.
- Labels MUST be bounded. User IDs, raw paths, query strings, exception text, and unconstrained
  remote values MUST NOT be labels.
- Success metrics MUST distinguish complete, partial, degraded, rejected, and failed outcomes when
  operational decisions depend on them.
- SLO or alert metrics MUST be tested or exercised enough to prove they change under the intended
  condition.

### 16.3 Tracing and profiles

- Trace spans MUST follow meaningful ownership boundaries rather than every function call.
- Span status and attributes MUST preserve error classification and privacy.
- Sampling policy MUST be explicit for expensive or sensitive traces.
- Profiling and debug endpoints MUST have access control and lifecycle policy.
- Operational capture instructions MUST be safe to run during an incident and state expected cost.

### 16.4 Health and lifecycle

- Liveness, readiness, startup, and dependency health MUST be distinct when their operational
  actions differ.
- A readiness check MUST prove the process can serve the advertised capability, not merely that the
  process is alive.
- Health endpoints MUST not create expensive dependency storms.
- Graceful shutdown MUST stop new work, drain or transfer owned work, enforce a deadline, flush
  required state, and expose failure to exit status.
- Background workers MUST expose stuck, failed, and backlog conditions.

### 16.5 Runbooks and recovery

R2 and R3 repositories MUST provide runbooks for material failures, including:

- detection signal;
- affected user behavior;
- immediate containment;
- safe diagnostic commands;
- data-integrity checks;
- rollback or failover;
- repair and replay;
- credential or key rotation where relevant;
- evidence preservation;
- escalation owner;
- post-recovery validation.

A recovery procedure MUST be exercised in a representative environment before it is claimed as
supported.

## 17. Documentation, collaboration, and open-source quality

### 17.1 README and first use

The README MUST enable an unfamiliar user to determine:

- what the product is and is not;
- supported use cases and platforms;
- security or privacy implications that affect adoption;
- installation and a minimal real task;
- configuration and required external services;
- how success and failure appear;
- current limitations and maturity;
- how to obtain help or report defects.

Quick-start commands MUST be tested from a clean supported environment or explicitly marked
unverified. Screenshots and examples MUST reflect current behavior.

### 17.2 Maintainer documentation

An unfamiliar maintainer MUST be able to locate:

- product concepts and glossary;
- package and component ownership;
- dependency direction;
- source-of-truth and projection boundaries;
- data and control flow;
- storage and migration model;
- security and privacy boundaries;
- generated-code workflow;
- local development and verification;
- release and rollback process;
- known limitations with owners.

Documentation SHOULD point to source rather than duplicate volatile implementation maps.

### 17.3 Change scope

- A pull request SHOULD perform one coherent change.
- Refactoring required to make the behavior honest belongs in the change; unrelated cleanup does
  not.
- Large mechanical changes SHOULD be separated from semantic changes when separation improves
  reviewability.
- Drive-by renames, formatting churn, generated drift, and dependency upgrades MUST NOT obscure a
  risky behavior change.
- A minimal diff is not automatically a good diff. The smallest acceptable change is the smallest
  one that leaves no known semantic or structural lie in scope.

### 17.4 Feature process

Before implementation of a significant feature, the repository MUST settle:

- user problem and non-goals;
- concept and authority;
- public contract;
- data and privacy impact;
- failure and recovery semantics;
- compatibility and migration;
- performance and operational implications;
- acceptance scenarios;
- evidence plan.

A prototype MAY precede a final ruling when isolated and reversible. Prototype behavior MUST NOT
silently become product canon.

### 17.5 Review conduct

- Review comments MUST distinguish correctness, policy, evidence, maintainability, and preference.
- Blocking comments MUST cite the violated contract, authority, or concrete failure scenario.
- Preference-only comments MUST NOT masquerade as language or repository law.
- Authors MUST answer material objections with evidence or a design change, not assertion.
- Reviewers MUST inspect relevant source and contracts rather than rely only on the pull-request
  summary.
- Generated or AI-assisted code has the same authorship and review obligations as handwritten code.
- The person submitting the change remains responsible for every line and claim.

### 17.6 Compatibility, release, and support

- Supported versions, platforms, architectures, data formats, and upgrade paths MUST be declared.
- Compatibility promises MUST include behavior, wire, storage, CLI, and operational semantics as
  applicable.
- Release notes MUST describe user-visible changes, migrations, security impact, deprecations, and
  known limitations.
- Release artifacts MUST be tied to source, generated state, dependency state, and verification
  evidence.
- A release MUST have an owner, rollback or roll-forward plan, and post-release validation.
- Unsupported debugging or internal tools MUST be clearly labeled and MUST NOT be presented as
  stable public interfaces. containerd's distinction between supported APIs and its debugging
  `ctr` client is a useful example of explicit scope.

### 17.7 Licensing and attribution

- Every distributed repository and artifact MUST have an explicit license or rights statement.
- Dependency and copied-code licenses MUST be compatible with the distribution model.
- Generated bundles, web assets, fonts, data files, examples, and test fixtures are part of license
  review.
- Source provenance and required notices MUST be retained.
- A reference implementation MAY inform design; copying code requires separate license and
  attribution review.

## 18. The three independent acceptance gates

A gate is a decision boundary, not a heading in a report. Each gate has its own evidence and owner.
One gate cannot compensate for failure in another.

### 18.1 Gate 1 — Architecture and open-source engineering quality

#### PASS requires

- the concept model, authority, source of truth, and ownership are explicit;
- package and dependency structure follow product capabilities;
- public and durable contracts are intentional and compatible;
- Go code follows the language and repository authority chain;
- security, privacy, data lifecycle, concurrency, and side-effect boundaries are reviewed at their
  final enforcement points;
- dependencies, generation, licensing, CI, release, and documentation meet the project profile;
- no known structural lie or acceptance-critical debt is hidden as future cleanup;
- an unfamiliar maintainer can explain the design from names, packages, contracts, decisions, and
  documentation without reconstructing intent from tests.

#### Gate 1 fails when

- a feature is bolted on through generic layers or duplicate rules;
- names, schema, UI, wire, and docs use the same term differently;
- implementation or test convenience selected product behavior;
- authority is inferred from current code, field existence, or test expectations;
- a public contract or migration is accidental;
- security or privacy is enforced only in a bypassable upstream helper;
- an abstraction exists primarily for mocks;
- a known wrong architecture is deferred without an owned exception and closure gate.

### 18.2 Gate 2 — Real-user and third-party-agent usability

Acceptance MUST be conducted by a person or agent that did not implement the feature and uses only
supported public surfaces.

#### Required scenario classes

As applicable, the acceptance set MUST include:

- clean first use;
- experienced or repeated use;
- missing and malformed configuration;
- invalid and ambiguous input;
- offline or unreachable dependency;
- timeout, rate limit, provider failure, and partial response;
- stale local data or authority change;
- cancellation, retry, duplicate request, and recovery;
- non-ASCII, long, empty, and boundary-size input;
- narrow viewport, zoom, keyboard, reduced motion, and no JavaScript;
- supported and unsupported platform behavior;
- destructive-action preview and confirmation;
- privacy-sensitive and least-authority use;
- upgrade, downgrade, migration, or restart where relevant.

#### PASS requires

- the task and prerequisites are discoverable;
- success is distinguishable from accepted, pending, partial, stale, degraded, empty, and failed;
- diagnostics identify fault ownership without leaking sensitive data;
- recovery instructions are executable and supported;
- an agent can determine outcome, completeness, retry safety, and next action in one supported
  interaction where the surface claims agent usability;
- the user need not know internal package names, database rows, provider quirks, or undocumented
  workarounds;
- browser or desktop claims were exercised through the actual UI, not inferred from component
  tests;
- the acceptance operator records friction, ambiguity, and unsafe affordances rather than merely
  completing the happy path.

#### Gate 2 fails when

- code can perform the operation but the user cannot discover it;
- error prose requires source knowledge to recover;
- empty output hides unavailability or partial failure;
- the UI claims success before durable authority confirms it;
- automation requires scraping human text or undocumented flags;
- the tested path uses internal shortcuts unavailable to real users;
- acceptance is signed only by the builder.

### 18.3 Gate 3 — Test and evidence-system quality

#### PASS requires

- test classes are selected from the risk model rather than habit;
- load-bearing claims cross the real production composition and relevant trust boundaries;
- deterministic, race, fuzz, browser, migration, fault, or distributed tests exist where the risk
  activates them;
- every load-bearing invariant has watched-red or equivalent mutation evidence;
- privacy, egress, wire bytes, stale authority, atomic activation, and ownership boundaries have
  direct evidence when applicable;
- benchmarks have stable workloads, baselines, environments, and escalation thresholds;
- coverage is used to locate blind spots, not as a standalone certificate;
- the canonical verification chain was executed on the immutable reviewed snapshot;
- failures are named, reproducible, and point to the broken contract.

#### Gate 3 fails when

- tests prove only fake-to-fake behavior;
- a test name claims a boundary the test never crosses;
- broad mocks make production wiring unexercised;
- timing tests depend on sleeps or scheduler luck when deterministic control is available;
- fuzzing targets trivial helpers while parsers and trust boundaries remain untouched;
- a green suite has never been observed red under the defect it claims to prevent;
- checks were skipped, relaxed, quarantined, or expectations changed merely to obtain green output;
- an unexecuted check is reported as PASS.

## 19. Test and evidence-system design

### 19.1 Risk-to-test mapping

The project profile MUST map material risks to evidence. Typical mappings include:

| Risk or contract | Minimum evidence candidates |
|---|---|
| Pure deterministic rule | table-driven unit and property tests |
| Public API behavior | consumer-style tests and compatibility fixtures |
| Parser or decoder | corpus, boundary tests, fuzzing, allocation limits |
| SQL or durable state | real-database integration, migration, crash and resume tests |
| HTTP/RPC contract | real encoder/decoder, transport integration, golden wire fixtures |
| CLI | subprocess E2E, exit code, stdout/stderr, machine-output schema |
| Browser UI | real-browser task probes, accessibility and navigation checks |
| Concurrency | race detector, deterministic interleaving, leak and cancellation tests |
| Time semantics | fake clock, synchronization barrier, or synctest |
| Privacy or egress | recording transport, exact bytes, bypass mutation, no-log evidence |
| Atomic publication | crash points, stale handles, reader visibility, activation mutation |
| Distributed consistency | model/history checking, fault injection, multi-process robustness |
| Performance | benchmark or load test with baseline and threshold |
| Cross-platform behavior | compile plus runtime contract on supported targets |

The table is a decision aid, not a substitute for the repository's actual threat and state model.

### 19.2 Unit tests

- Unit tests SHOULD isolate a coherent semantic unit, not implementation trivia.
- Table-driven tests are useful when cases share one contract. They MUST NOT hide distinct behavior
  behind unreadable parameter matrices.
- Test names MUST describe the protected behavior and condition.
- Tests SHOULD assert observable results and invariant-preserving state, not private call order
  unless call order is the contract.
- Helpers MUST fail at the caller and keep diagnostics specific.
- A unit test MUST NOT duplicate the implementation algorithm so exactly that both fail together.

### 19.3 Integration tests

- Integration tests MUST use the real implementation at each boundary under claim.
- A storage claim requires the selected database or a contract-equivalent certified environment,
  not only an in-memory map.
- A transport claim requires the real serializer and client/server composition.
- Configuration and dependency construction SHOULD use the production composition path.
- Test environments MUST be reproducible and must clean up deterministically.
- External services MAY use a local emulator only when differences are known and separately
  certified against the real provider where material.

### 19.4 End-to-end tests

- E2E tests MUST begin at a supported user or operator entry point and observe supported outputs.
- They MUST NOT rely on private database mutation or internal APIs except for controlled setup and
  observation that cannot change the path under test.
- E2E fixtures MUST represent realistic data, permissions, and failure conditions.
- Tests SHOULD preserve diagnostics and artifacts needed to reproduce failure.
- A small, trustworthy E2E set is preferred over a large flaky suite.

### 19.5 Golden and data-driven tests

- Golden files are appropriate for stable, reviewable contracts such as wire output, diagnostics,
  rendering, query plans, or migrations.
- Golden updates MUST be an explicit review action and MUST show semantic diff.
- Tests MUST NOT auto-accept new goldens in normal verification.
- A golden file MUST not hide nondeterministic values; normalize only values that are truly outside
  the contract.
- Data-driven tests SHOULD provide concise commands, inputs, and named expected results. A custom
  harness must remain simpler than the behavior it tests.

### 19.6 Fuzzing and property testing

- Fuzz targets MUST focus on trust boundaries and high-state-space logic: parsers, decoders,
  Unicode, paths, protocol frames, schemas, state machines, and round trips.
- The seed corpus MUST include real and adversarial edge cases.
- Targets MUST assert semantic properties, not only absence of panic.
- Fuzzing MUST bound resource consumption and preserve minimized crashers in the repository.
- Security- or durability-sensitive fuzz failures MUST block acceptance until classified.
- A scheduled or release fuzz budget SHOULD supplement pull-request smoke fuzzing for R2 and R3
  parsers.

### 19.7 Race, synchronization, and leak tests

- `go test -race` MUST run for concurrent code on a representative supported platform.
- Race tests MUST exercise real production wiring and background lifecycles.
- Deterministic synchronization SHOULD expose critical transitions without sleeping.
- Lifecycle tests MUST prove shutdown, cancellation, timer cleanup, channel closure, and worker
  termination as applicable.
- Goroutine-count assertions MAY support leak detection but MUST account for stable runtime noise and
  SHOULD be paired with owned lifecycle evidence.

### 19.8 Browser and UI evidence

- Browser probes MUST use a supported browser engine and the production-like asset and routing path.
- They MUST verify task completion, keyboard order, focus visibility, accessible names, history,
  narrow viewport, long content, error recovery, and no-JS behavior as applicable.
- Visual regression tests SHOULD cover stable high-value surfaces and MUST be reviewed rather than
  bulk-updated.
- Component tests MAY accelerate feedback but MUST NOT replace real-browser acceptance for browser
  claims.
- Desktop UI tests MUST similarly exercise real windowing, input, platform integration, and
  persistence boundaries appropriate to the product.

### 19.9 Migration, compatibility, and upgrade tests

- Migration tests MUST begin from supported historical schemas or released fixtures.
- Rolling upgrades MUST test mixed-version interaction when supported.
- Compatibility fixtures MUST be immutable and tied to released behavior.
- Downgrade behavior MUST be defined as supported, refused safely, or destructive only with explicit
  authority.
- Old readers, old writers, stale processes, and partially migrated data MUST be included where they
  can exist.

### 19.10 Fault injection and robustness

- Fault tests SHOULD inject failures at owned boundaries: I/O, commit, fsync, transport, timeout,
  cancellation, restart, publication, leader change, and dependency degradation.
- R3 distributed systems MUST include randomized or systematic robustness testing beyond handpicked
  examples.
- Histories SHOULD be checked against the declared consistency model when feasible.
- Test-generated failures MUST be reproducible through seed, schedule, trace, or captured history.
- Fault injection MUST not rely solely on an alternate fake implementation whose behavior is
  unrelated to production.

### 19.11 Performance tests

- Benchmarks MUST state the protected workload and why it matters.
- Baselines and thresholds MUST be stored or retrievable in a durable system.
- CI performance gates MUST account for runner variance and detect regressions at the claimed size.
- Load tests MUST define warm-up, steady state, saturation, failure criteria, and cleanup.
- A benchmark result MUST NOT be generalized to unsupported hardware, data shape, concurrency, or
  provider behavior.

### 19.12 Coverage

- Coverage is a map of executed code, not proof of assertions or correctness.
- The repository MAY set a coverage policy, but MUST NOT use an arbitrary percentage as the sole
  quality gate.
- Reviewers MUST inspect uncovered branches around authority, errors, concurrency, parsing,
  migrations, and side effects.
- A coverage increase produced by weak assertions is not an improvement.
- Coverage exclusions MUST be narrow and reasoned.

### 19.13 Test doubles

- A fake, stub, mock, emulator, or recording implementation MUST have a named purpose.
- Test doubles SHOULD model a boundary contract, not recreate the entire dependency.
- They MUST NOT invent product semantics or make impossible states appear valid.
- A recording double is preferred when the evidence concerns exact calls or bytes.
- Broad mocks that assert internal call order make refactoring expensive and SHOULD be replaced by
  behavioral evidence.
- Production abstractions MUST NOT be introduced solely to accommodate a mocking framework.

### 19.14 Determinism and flake policy

- Tests MUST control time, randomness, locale, time zone, filesystem roots, environment, ports, and
  ordering where those values are not the contract.
- Parallel tests MUST not share mutable process or external state without explicit isolation.
- A flaky test is a product-quality defect.
- Automatic retry MAY collect diagnostics but MUST NOT convert a flaky failure into PASS.
- Quarantine requires an owner, issue, user risk, scope, expiry or trigger, and release policy.
- Tests that cannot fail reliably under the protected defect MUST be redesigned.

## 20. Watched-red and mutation evidence

### 20.1 Requirement

Every load-bearing invariant MUST have recorded evidence that the intended verification fails when
the invariant is deliberately broken and returns green when restored.

The evidence record MUST contain:

```text
Invariant ID:
Protected contract:
Snapshot:
Given:
When:
Then:
Forbidden side effect:
Mutation applied:
Expected failing check:
Observed red:
Failure message or artifact:
Mutation reverted:
Restored green:
Reviewer:
Date:
```

A test that has never been observed failing under the intended defect is an assertion, not a
proven lock.

### 20.2 Required mutation classes

As applicable, mutation evidence MUST cover:

- removing final authority or consent revalidation;
- accepting a stale generation, lease, identity, or capability;
- bypassing the sole egress, commit, or publication owner;
- enabling an unsafe retry or duplicate irreversible action;
- changing exact security-, privacy-, billing-, or compatibility-significant wire bytes;
- leaking sensitive values through errors, logs, traces, or metrics;
- converting unavailable, partial, invalid, or provider-failed into empty success;
- exposing staging or partial durable state to readers;
- weakening validation, size limits, path constraints, or decoder strictness;
- violating lock order, cancellation, or goroutine termination;
- skipping required migration or compatibility behavior;
- wiring a fake or alternate implementation into production composition.

### 20.3 Mutation quality

- Mutations MUST represent plausible future regressions, not arbitrary syntax damage.
- The observed failure MUST identify the protected contract clearly enough for a maintainer to act.
- A mutation that fails only an unrelated lint or compilation check does not prove the semantic
  lock, unless compilation is itself the protected contract.
- Equivalent mutation tooling MAY automate coverage, but high-risk invariants still require
  reviewed semantic mutations.
- Evidence SHOULD be retained with the review or release record, not merely described from memory.

## 21. Canonical verification chain

### 21.1 One entry point

Every repository MUST provide one documented command that runs the complete mandatory automated
verification for the current profile, for example:

```text
make verify
just verify
./tools/verify
mage verify
```

The name is repository-specific. Its meaning is not: it MUST fail if any mandatory automated check
fails or generated state drifts.

The command MUST be runnable from a clean checkout using documented prerequisites. It MUST NOT
silently skip a required tool because that tool is missing.

### 21.2 Required stages

The profile MUST resolve exact commands and applicability for this ordered set:

1. repository and policy validation;
2. formatting and forbidden-file checks;
3. generated-file drift;
4. module and dependency drift;
5. build for supported primary targets;
6. `go vet`;
7. static analysis, normally including `staticcheck`;
8. strict repository lint;
9. security analysis;
10. Go vulnerability analysis;
11. unit and package tests;
12. race tests for concurrent code;
13. integration tests;
14. end-to-end or subprocess tests;
15. fuzz smoke for selected trust boundaries;
16. browser or desktop probes;
17. migration and compatibility tests;
18. mutation locks or watched-red evidence checks;
19. cross-platform compile and runtime contracts;
20. performance smoke or regression checks;
21. license, notice, and artifact policy checks;
22. release artifact build and provenance verification.

A stage MAY be `N/A` only through the project profile with a concrete scope reason. Expensive or
credentialed certification MAY run in a separate named pipeline, but its absence MUST remain
visible and MUST block the release claims that depend on it.

### 21.3 Go baseline

A typical Go verification chain includes, adjusted to repository scope:

```sh
gofmt drift check
go generate drift check
go mod tidy drift check
go build ./...
go vet ./...
staticcheck ./...
golangci-lint run with repository-owned strict configuration
gosec or equivalent targeted security analysis
govulncheck ./...
go test ./...
go test -race ./...
go test selected integration and E2E packages
short bounded fuzz runs for selected targets
cross-compilation and supported-platform runtime jobs
```

This list is illustrative. The project profile is responsible for exact commands, tags,
environment, packages, timeouts, credentials, and artifact locations.

### 21.4 Tool configuration

- Tool versions MUST be pinned or reproducibly installed.
- Tool configuration MUST be reviewed as code.
- Enabling a new rule requires evaluating false positives, semantic value, and remediation impact.
- Disabling or suppressing a rule requires a narrow rationale and owner.
- A change MUST NOT lower lint, security, test, decoder, or compatibility strictness merely to pass.
- A generated baseline of ignored findings MUST have an owner and burn-down or review policy; it
  MUST NOT hide new findings.
- Tool output SHOULD be deterministic and machine-readable in CI where practical.

### 21.5 Evidence log

Every formal review or release MUST record:

- immutable commit and submodule or dependency-lock state;
- clean or described worktree state;
- Go and tool versions;
- operating system, architecture, and relevant environment;
- exact commands;
- PASS, FAIL, N/A, UNVERIFIED, or BLOCKED per stage;
- links or paths to logs, reports, screenshots, profiles, histories, and mutation evidence;
- credentials or real-provider certification performed, without exposing secrets;
- deviations and exception identifiers.

No check may be reported PASS unless it was actually executed successfully against the identified
snapshot.

## 22. Change, review, and release workflow

### 22.1 Phase 0 — Intake and scope

Before changing code:

- identify the user or operator need;
- state the intended outcome and non-goals;
- classify risk and capability flags;
- identify affected public, durable, security, privacy, and operational contracts;
- list required owner rulings;
- select acceptance and evidence owners separate from the builder where required.

A request phrased as an implementation is not proof that the proposed mechanism is correct.

### 22.2 Phase 1 — Comprehend

The builder and reviewer MUST read:

- product canon and explicit rulings;
- architecture and relevant decisions;
- public API, CLI, UI, schema, and wire contracts;
- relevant production composition;
- existing tests by actual boundary crossed, not test name;
- operational, security, and privacy constraints.

Initial objections MUST be recorded before the proposed solution becomes psychologically default.

### 22.3 Phase 2 — Model and challenge

For each proposed concept, state, dependency, interface, cache, or side effect, ask:

1. What user or product fact requires it?
2. Is it the source of truth, a capability, or a projection?
3. Who owns its meaning?
4. Is the behavior uniquely authorized?
5. Can the design use fewer concepts without lying?
6. Is complexity selected for implementation or test convenience?
7. What happens across time, cancellation, failure, and concurrency?
8. How will a future regression be forced red?
9. What would an unfamiliar user or maintainer misunderstand?

If two reasonable product behaviors remain, mark `NEEDS-OWNER`. Reversible investigation may
continue, but disputed behavior and irreversible implementation MUST stop.

### 22.4 Phase 3 — Plan

A non-trivial implementation plan MUST include:

- scope and non-goals;
- concept card and semantic owner;
- authority citations and unresolved decisions;
- source-of-truth and projection map;
- complete relevant state and failure matrix;
- dependency and side-effect boundaries;
- concurrency and event timeline;
- compatibility, schema, and migration impact;
- security, privacy, and data-lifecycle impact;
- performance and operational impact;
- Gate 2 acceptance scenarios;
- Gate 3 test classes and watched-red mutations;
- rollout, rollback, and contradiction-hunt plan.

A plan MUST NOT cite itself as product authority.

### 22.5 Phase 4 — Implement

- Implement the smallest coherent semantic slice.
- Preserve a working, reviewable state where feasible.
- Keep product rules in their semantic owner.
- Use production composition in integration paths from the beginning.
- Add or update docs, schema, generated inputs, migrations, help, diagnostics, and observability in
  the same change as the behavior.
- Do not introduce temporary architecture without an approved exception.
- Do not silently reinterpret an existing ruling.
- Do not broaden capability, egress, credentials, or write authority without explicit review.

### 22.6 Phase 5 — Builder falsification review

Before requesting final review, the builder MUST try to disprove the change:

- remove or invert load-bearing checks and observe red;
- search for alternate call paths and capabilities;
- inspect final bytes, queries, logs, and durable state;
- exercise cancellation, duplicate work, stale authority, and partial failure;
- compare code, docs, UI, CLI, schema, wire, metrics, and tests for semantic drift;
- run the canonical verification chain;
- review the diff for accidental public surface, dependency, permission, and generated changes;
- perform a new-contradiction hunt after the last fix.

Builder review is required but cannot issue the final certificate for R2/R3 or other profile-marked
high-risk changes.

### 22.7 Phase 6 — Independent review and red team

The independent reviewer MUST:

- review an immutable commit, not a moving worktree;
- confirm scope and authority before accepting implementation rationale;
- inspect source and production composition directly;
- reproduce high-risk evidence and selected watched-red mutations;
- rerun or independently observe required acceptance scenarios;
- recompute material numbers, coverage claims, matrices, capacity, and cost estimates;
- attack authority, privacy, egress, stale state, concurrency, recovery, and agent interpretation;
- hunt for contradictions introduced by the fix;
- record verified, inferred, unresolved, and unverified claims separately.

The independent reviewer MUST NOT be the sole author of the implementation. Tool-assisted review is
permitted, but responsibility belongs to the named reviewer.

### 22.8 Phase 7 — Merge and release decision

Merge-ready, release-ready, and production-ready are distinct claims.

- **Merge-ready** requires the repository's merge policy and all change-scope gates.
- **Release-ready** additionally requires release, compatibility, artifact, documentation, and
  rollout evidence.
- **Production-ready** additionally requires environment-specific configuration, credentials,
  operations, rollback, monitoring, capacity, and real-environment certification.

The decision MUST be bound to an immutable commit or artifact digest. A later change invalidates the
certificate unless the review policy explicitly proves the changed scope is irrelevant.

### 22.9 Phase 8 — Post-release verification

Where releases affect users or shared systems, the release owner MUST verify:

- artifact identity and configuration;
- migration and compatibility outcome;
- primary user task;
- error and rollback path;
- health, metrics, logs, and alerts;
- privacy and egress expectations;
- absence of new contradictions or undocumented behavior.

Rollback or roll-forward criteria MUST be selected before the release, not invented during failure.

## 23. Exceptions, debt, and unresolved work

### 23.1 Exception record

Any departure from a MUST or MUST NOT requires a record containing:

```text
Exception ID:
Clauses:
Scope:
Snapshot or versions:
Reason the standard cannot currently be met:
Alternatives considered:
Concrete risk:
User or operator impact:
Mitigation:
Compensating evidence:
Owner:
Approver:
Created:
Expiry date or objective review trigger:
Closure condition:
Release and merge effect:
```

An exception MUST be narrow, visible, time-bounded or trigger-bounded, and independently approved.

### 23.2 Invalid exceptions

An exception is invalid when it:

- exists only to make a deadline, test, or linter green without describing risk;
- has no owner or closure condition;
- waives applicable law, explicit privacy commitments, or a known critical authority boundary;
- suppresses an entire tool or category when one local finding is disputed;
- calls a reachable behavior `N/A`;
- hides a known defect in prose such as "follow up later";
- retroactively claims approval that did not exist at implementation time;
- remains open after its expiry or trigger without renewed review.

### 23.3 Technical debt

Known debt that is not fixed in the current scope MUST have:

- concrete description;
- affected contract and risk;
- owner;
- reason it is safe to defer;
- containment;
- issue or decision identifier;
- objective trigger or target milestone;
- gate impact.

Debt that makes the current design dishonest, unsafe, unrecoverable, or untestable is not deferrable
technical debt; it is a blocker.

### 23.4 `N/A`, deferred, and unresolved

- `N/A` means the capability or risk does not exist.
- `DEFERRED-BY-EXCEPTION` means it exists and a reviewed temporary departure is accepted.
- `UNRESOLVED` means authority or applicability is unsettled and acceptance is blocked.
- `UNVERIFIED` means evidence is absent.

These statuses MUST NOT be conflated.

## 24. Severity and verdicts

### 24.1 Severity

#### CRITICAL

Private-data egress, credential exposure, privilege or authority bypass, irreversible corruption,
unsafe publication, exploitable trust-boundary failure, split-brain safety violation, or fundamental
contradiction of a binding product contract.

Default verdict: **NO-GO**.

#### HIGH

Primary functionality is wrong; recovery is unavailable; stale, partial, or failed state is
reported as success; a destructive action is not safely bounded; production composition is
untested; compatibility or migration is materially broken; or public claims are false.

Default verdict: **NO-GO**, or **ACCEPT-WITH-GATES** only when release is blocked and a valid,
contained remediation gate exists.

#### MEDIUM

An important edge case, operational path, accessibility requirement, ownership boundary, test class,
or documentation contract is missing or misleading, without an immediate critical failure.

Default verdict: depends on affected gate and release scope.

#### LOW

No immediate correctness failure is proven, but the work falls below the repository's public,
maintainability, consistency, or evidence quality.

Default verdict: may merge only when repository policy permits and no gate-critical accumulation
exists.

### 24.2 Verdicts

- **GO**: all three gates pass on an immutable snapshot; all required checks are PASS or valid N/A;
  no unresolved blocker remains.
- **ACCEPT-WITH-GATES**: the direction is conditionally acceptable, but named gates remain open.
  The work MUST NOT be described as complete, release-ready, or production-ready.
- **NO-GO**: a semantic, authority, security, privacy, usability, durability, compatibility, or
  evidence blocker remains.

A count of findings does not determine the verdict. One load-bearing contradiction can outweigh
hundreds of passing checks.

## 25. Required review report

Every formal acceptance review MUST use this structure or a repository-equivalent structure that
preserves every field:

```markdown
# Verdict

GO / ACCEPT-WITH-GATES / NO-GO

One-sentence rationale:

# Snapshot

- Commit:
- Artifact digest:
- Worktree state:
- Generated state:
- Dependency state:
- Review environment:
- Profile and exception IDs:

# Scope and risk

- User need:
- Non-goals:
- Risk class and capability flags:
- Public, durable, security, privacy, and operational contracts affected:

# Concept model

- Concept and non-concept:
- Semantic owner:
- Authority:
- Source of truth:
- Capabilities:
- Projections and caches:
- Invariants:

# Initial objections

Objections recorded before implementation verification.

# Authority ledger

Behavior | authority class | source | status | notes

# Findings

Severity | status | file:line or surface | defect | concrete failure scenario |
required action | owner

# State and failure matrix

Input/state | result shape | status/exit | diagnostics | side effects | authority | evidence

# Event and concurrency analysis

Capture → work → revalidate → commit → publish, including cancellation, retry, stale work, crash,
and competing actions.

# Gate 1 — Architecture and open-source engineering quality

PASS / PARTIAL / FAIL

Evidence and blockers:

# Gate 2 — Real-user and third-party-agent usability

PASS / PARTIAL / FAIL

Acceptance operator, scenarios, observations, friction, and artifacts:

# Gate 3 — Test and evidence-system quality

PASS / PARTIAL / FAIL

Boundary map, production composition, missing evidence, and confidence limits:

# Watched-red evidence

Invariant | mutation | expected check | observed red | restored green | artifact

# Security, privacy, and supply chain

Threat boundaries, egress, secrets, data lifecycle, dependencies, and unresolved risk.

# Performance and operations

Workload, baseline, budgets, measurements, health, recovery, and unverified claims.

# Compatibility and release

API/wire/storage/CLI impact, migration, rollout, rollback, artifact identity, and support matrix.

# Contradiction hunt

Canon, code, package ownership, schema, wire, CLI, UI, docs, logs, metrics, tests, and release
inconsistencies found after the final repair.

# NEEDS-OWNER

Only genuine product decisions, each with options, consequences, and blocked work.

# Exceptions and debt

IDs, scope, owner, expiry or trigger, containment, and gate effect.

# Unverified and blocked checks

Checks not executed, unavailable evidence, prerequisites, and claims they prevent.

# Verification log

Exact commands, status, environment, and artifact locations.

# Independent reviewer certification

Reviewer:
Snapshot:
Date:
Scope certified:
Scope explicitly not certified:
```

## 26. Mandatory coding-agent instruction block

Repositories using coding agents MUST adapt the following block into their authoritative agent
instructions without weakening it:

```text
Treat semantic correctness as the primary gate.

Before non-trivial implementation, define the user need, concept, non-concept, semantic owner,
authority, source of truth, capabilities, projections, legal states, failure semantics,
irreversible side effects, concurrency timeline, and load-bearing invariant.

Use this authority order: law/security/privacy and explicit product commitments; product-owner
rulings; released contracts; Go specification, memory model, and standard library; repository
decisions; repository engineering standard; Effective Go, Go Code Review Comments, and Google Go
Style; reference projects; tools; personal preference.

Enforce:
- one term, one meaning across canon, code, schema, SQL, API, CLI, UI, logs, metrics, docs, and tests;
- one authoritative semantic owner per concept, with projections unable to redefine it;
- explicit separation of source truth, captured capability, derived state, cache, projection,
  transport, and diagnostics;
- complete relevant state and error semantics with honest fault ownership and recovery;
- final-boundary revalidation for authority, privacy, freshness, and irreversible effects;
- capability-oriented packages and clear dependency direction;
- concrete Go types and consumer-owned interfaces unless substitution is real;
- explicit context, resource, goroutine, cancellation, retry, and error ownership;
- no production abstraction created only for tests;
- production-composition evidence for product claims;
- watched-red mutation proof for every load-bearing invariant;
- contradiction hunting after every repair;
- immutable snapshot identity for formal acceptance;
- separate builder and independent final reviewer for profile-defined high-risk work.

Do not choose product behavior because it is easier to implement, mock, store, or test. If two
reasonable product behaviors remain, mark NEEDS-OWNER and stop disputed or irreversible work while
continuing only reversible investigation.

Do not lower lint, skip or retry-away failing tests, relax decoders, weaken validation, swallow
errors, expose secrets, broaden authority, or change expectations merely to obtain green output.
Never report a check as PASS unless it was actually executed successfully on the stated snapshot.

The three independent acceptance gates are:
1. architecture and open-source engineering quality;
2. real-user and third-party-agent usability;
3. test and evidence-system quality.

Final verdict: GO, ACCEPT-WITH-GATES, or NO-GO.
```

Agent instructions MUST also include the repository's canonical verification command, protected
paths, generated-file workflow, high-risk final boundaries, and local traps. They SHOULD NOT
attempt to restate the entire codebase.

## 27. Reference codebases: lessons and limits

Reference projects provide evidence that a practice can work under their constraints. They do not
replace local reasoning.

| Reference | Practices worth studying | Do not cargo-cult |
|---|---|---|
| Go standard library | small APIs; precise contracts; meaningful zero values; concrete types; error and resource discipline; examples as tests; compatibility awareness | internal constraints or historical APIs that do not match the product |
| CockroachDB | product-level RFCs; SQL logic and data-driven tests; roachtests; production assertions; storage, transaction, migration, and distributed failure analysis | monorepo scale, abstractions, or distributed complexity before the problem requires them |
| etcd and `etcd-io/raft` | separation of consensus core from system integration; robustness histories; fault injection; linearizability evidence; explicit persistence and snapshot semantics | treating Raft as a complete distributed-system design or copying topology without matching assumptions |
| Kubernetes and `client-go` | API conventions; desired versus observed state; reconciliation; versioning, defaulting, validation, and compatibility; generated clients; controller work queues | controllers for synchronous problems, giant API machinery, or eventual consistency where immediate transactions are required |
| Temporal | explicit service boundaries; durable histories; idempotent task execution; retry and timeout semantics; operational readiness | workflow machinery for ordinary request/response work or assuming all application code becomes deterministic automatically |
| Prometheus | narrow product model; explicit query and storage semantics; disciplined contribution checks; stable exposition contracts; operational focus | a custom data model or global labels that do not fit the product's cardinality and retention needs |
| containerd | clear supported scope; strong component boundaries; content and snapshot ownership; plugin interfaces driven by a real ecosystem; separation of supported APIs from debugging tools | plugin architecture for a closed application or presenting internal CLIs as stable user contracts |
| NATS Server | high-performance network paths; bounded protocol parsing; explicit lock ordering; clear server ownership and concurrency discipline | specialized lock or allocation techniques without measurement and equivalent invariants |
| OpenTelemetry Collector | receiver–processor–exporter pipelines; factories and lifecycle for a genuine component ecosystem; per-component stability and distribution testing | generic pipelines, factories, or component registries when the product has fixed behavior |
| Caddy | typed modules; normalized configuration; adapters at ingress; lifecycle and provisioning; strong automation around certificates and config | making every feature a module or treating JSON/config adaptation as the domain model |
| Zed | user-perceived responsiveness; feature process; focused changes; real UI and visual regression checks; cross-platform validation; disciplined coding-agent rules | Rust-specific style in Go, editor-specific frame budgets, or architecture tied to Zed's GPU and collaborative-editor constraints |

### 27.1 Cross-project deductions

The following lessons are sufficiently common to adopt as defaults:

1. **Small semantic cores beat broad framework layers.** etcd's Raft core, Go's standard library,
   and focused server packages show the value of explicit contracts around a narrow owner.
2. **Compatibility is designed, not inferred.** Kubernetes API conventions, Go's compatibility
   discipline, OpenTelemetry component stability, and containerd's supported-surface distinction
   make compatibility visible.
3. **Failure evidence must resemble production.** CockroachDB roachtests, etcd robustness testing,
   Temporal service integration, and real-browser UI checks demonstrate why unit-only green is not
   enough.
4. **Concurrency policy belongs in the repository.** NATS's explicit lock ordering and distributed
   systems' generation and lease rules cannot be left as tribal knowledge.
5. **Performance starts with user or workload budgets.** Zed's responsiveness discipline and
   Prometheus, NATS, and storage-engine measurement practices are valuable because they tie
   optimization to an observed path.
6. **Extensibility requires a real ecosystem.** Caddy, containerd, and OpenTelemetry justify module
   machinery through third-party or separately shipped components; ordinary applications should
   remain ordinary.
7. **Agent instructions are an engineered interface.** They should contain high-value local traps,
   commands, and invariants, remain reviewable, and avoid becoming a stale duplicate of the source.

## 28. Normative and reference sources

The repository's exact selected Go release documentation remains authoritative for that build. The
following sources define the baseline used by this standard:

### 28.1 Go authorities

- [The Go Programming Language Specification](https://go.dev/ref/spec)
- [The Go Memory Model](https://go.dev/ref/mem)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go Compatibility](https://go.dev/doc/go1compat)
- [Go Fuzzing](https://go.dev/doc/tutorial/fuzz)
- [`testing/synctest`](https://pkg.go.dev/testing/synctest)
- [`runtime/pprof`](https://pkg.go.dev/runtime/pprof)
- [`govulncheck`](https://go.dev/doc/tutorial/govulncheck)
- [Google Go Style Guide](https://google.github.io/styleguide/go/guide)
- [Google Go Style Decisions](https://google.github.io/styleguide/go/decisions)
- [Google Go Best Practices](https://google.github.io/styleguide/go/best-practices)

### 28.2 Reference projects

- [CockroachDB](https://github.com/cockroachdb/cockroach)
- [etcd](https://github.com/etcd-io/etcd) and [etcd Raft](https://github.com/etcd-io/raft)
- [Kubernetes](https://github.com/kubernetes/kubernetes)
- [Temporal](https://github.com/temporalio/temporal)
- [Prometheus](https://github.com/prometheus/prometheus)
- [containerd](https://github.com/containerd/containerd)
- [NATS Server](https://github.com/nats-io/nats-server)
- [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)
- [Caddy](https://github.com/caddyserver/caddy)
- [Zed](https://github.com/zed-industries/zed)

A repository MAY cite additional primary sources in its project profile. Blog posts, conference
talks, and reference implementations MAY explain a decision but MUST NOT silently outrank binding
contracts or language specifications.

## 29. Final maxims

> Do not review what the author intended to say. Review what an unfamiliar maintainer, a real user,
> an operator under failure, and a future breaker will actually encounter.

> Do not optimize for green checks. Engineer checks that are forced red by the defects the product
> cannot afford.

> A rule is hard only when applicability, authority, evidence, exception, and verdict are all
> explicit.
