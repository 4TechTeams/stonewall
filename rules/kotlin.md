---
id: kotlin
name: Kotlin
extensions: [kt, kts]
---

# Kotlin Ruleset

## Exhaustiveness

**ID: `kotlin/exhaustiveness`**

Domain states are modeled as sealed hierarchies or enums and consumed through `when` expressions without an `else`
branch. The compiler then reports every consumption site when a new subtype appears, which turns a modeling change into
a set of compile errors instead of a silent runtime fallthrough.

### How to validate

- [ ] Search for `when` blocks over sealed types or enums that contain `else ->` and produce a default value, log a
  warning, or throw a generic exception.
- [ ] Flag `when` used as a statement rather than an expression over a sealed type, since statement position does not
  enforce exhaustiveness in older compilation setups and hides missing branches from review.
- [ ] Look for `is` chains combined with `if (x is A) ... else if (x is B)` over a closed hierarchy, which bypasses
  exhaustiveness entirely.
- [ ] Flag sealed interfaces or classes whose subtypes are declared in modules where the compiler cannot see them all,
  for example public sealed hierarchies intended for external implementation.
- [ ] Check that boolean flags or `String` constants are not used where a closed set of states exists, for example
  `status: String` with values checked by equality.

### Mitigation

Replace open representations such as strings, integers, and boolean pairs with sealed interfaces whose subtypes carry
exactly the data valid in that state. Assign the result of `when` to a value or return it directly so the compiler
demands full coverage. When an `else` branch appears necessary, that usually means the type is too wide for the call
site, so narrow the parameter type instead of adding a fallback.

## Nullability as Design

**ID: `kotlin/nullability-as-design`**

Nullability expresses a real domain possibility, not an implementation convenience. A nullable type is a contract
stating that absence is meaningful, so nullable declarations that only exist to satisfy initialization order or
framework lifecycles push a runtime failure into every consumer.

### How to validate

- [ ] Flag every `!!` operator. Each occurrence is an assertion that the type system is wrong and needs an explanation
  in code or removal.
- [ ] Flag `lateinit var` outside of test fixtures and framework injected fields, especially where the field is written
  more than once.
- [ ] Look for nullable properties in data classes that are never null after construction, indicated by constructors or
  factories that always pass a non-null value.
- [ ] Check for `?: return`, `?: emptyList()`, or `?: 0` chains that swallow absence without distinguishing "no data"
  from "failed to load".
- [ ] Flag platform types crossing into domain code, meaning values from Java APIs assigned without an explicit nullable
  or non-null type annotation.
- [ ] Look for `requireNotNull` or `checkNotNull` at points deep inside call chains rather than at the system boundary.

### Mitigation

Validate at the boundary where external data enters the system and convert to non-null domain types immediately, so
internal code never re-checks. Use constructor injection and immutable properties rather than `lateinit`, and where a
value genuinely arrives later, model the two phases as two types. When absence carries information, use a sealed result
type rather than null so the reason survives.

## Immutability by Default

**ID: `kotlin/immutability-by-default`**

Values are declared `val`, collections are exposed through read-only interfaces, and state changes produce new instances
through `copy`. Shared mutable state is the precondition for data races and for action at a distance, both of which are
absent by construction when instances cannot change.

### How to validate

- [ ] Flag `var` properties in classes where no mutation site requires them, and `var` locals that are assigned exactly
  once.
- [ ] Look for `MutableList`, `MutableMap`, or `MutableSet` in public function signatures, return types, or public
  properties.
- [ ] Flag a private `MutableList` backing a public `List` property without `toList()` or an unmodifiable wrapper, since
  the exposed reference still aliases the mutable instance.
- [ ] Check data classes for mutable constructor parameters, for example `val items: MutableList<T>`, which allows
  external mutation after construction.
- [ ] Look for arrays used as fields where a `List` would do, since arrays are always mutable.
- [ ] Flag mutation of collection elements inside `forEach` where `map` or `fold` expresses the same transformation.

### Mitigation

Default every declaration to `val` and widen to `var` only when a measured need appears. Return `List` and construct
defensive copies when handing out internal collections, or build results with `buildList` so the mutable phase ends
before publication. Model state transitions as functions returning a new state object, which also makes the transition
testable in isolation.

## Errors as Values

**ID: `kotlin/errors-as-values`**

Expected failures appear in the return type. Exceptions are reserved for programming errors and unrecoverable
conditions. A signature that returns `Either`, a sealed `Result` type, or `kotlin.Result` documents its failure modes at
compile time, whereas a thrown checked condition is invisible to the caller in Kotlin because the language has no
checked exceptions.

### How to validate

- [ ] Look for `try/catch` blocks that catch `Exception` or `Throwable` broadly and return a default value or null.
- [ ] Flag `catch` blocks with empty bodies or bodies containing only a log statement, which discard the failure while
  allowing execution to continue in an undefined state.
- [ ] Check that `CancellationException` is not caught and swallowed inside coroutine code, since that breaks structured
  concurrency cancellation.
- [ ] Look for functions whose KDoc mentions throwing but whose signature returns a plain domain type.
- [ ] Flag `runCatching` used around large blocks where only one call can fail, since it captures unrelated failures
  including cancellation.
- [ ] Check that domain errors are typed rather than represented as `String` messages or generic `Exception` instances.

### Mitigation

Define a sealed error hierarchy per bounded context and return it from operations that can fail for domain reasons.
Reserve `require`, `check`, and `error` for invariant violations that indicate a bug, and let those propagate. When
integrating a throwing library, wrap it once at the adapter layer and translate exceptions into the typed error at that
single point.

## Type-Driven Domain Modeling

**ID: `kotlin/type-driven-modeling`**

Domain concepts are distinct types rather than primitives. A function taking `UserId` and `OrderId` cannot receive them
in the wrong order, while a function taking two `String` parameters can, and the compiler will not notice.

### How to validate

- [ ] Look for function signatures with two or more adjacent parameters of the same primitive type, for example
  `fun transfer(from: String, to: String, amount: Long)`.
- [ ] Flag identifiers, money amounts, quantities, and durations represented as `String`, `Int`, `Long`, or `Double` in
  domain code.
- [ ] Check for `Double` or `Float` used for monetary values, which introduces representation error in arithmetic.
- [ ] Look for value classes or data classes wrapping a primitive whose constructor performs no validation, meaning
  invalid instances are constructible.
- [ ] Flag boolean parameters at call sites, for example `process(order, true, false)`, where an enum would name the
  intent.
- [ ] Check that validation logic for a concept is not duplicated across call sites rather than living in the type.

### Mitigation

Introduce `@JvmInline value class` wrappers for identifiers and quantities so the abstraction costs no allocation on the
JVM. Make constructors private and expose a factory returning a typed result that rejects invalid input, so an instance
existing is proof of validity. Replace boolean parameters with small enums whose names appear at the call site.

## Structured Concurrency

**ID: `kotlin/structured-concurrency`**

Every coroutine runs in a scope with a defined lifetime and a defined parent. Coroutines launched outside a
lifecycle-bound scope outlive the work that started them, which leaks memory, continues after cancellation, and produces
failures with no owner to report them.

### How to validate

- [ ] Flag `GlobalScope` usage anywhere in production code.
- [ ] Look for `CoroutineScope(...)` constructed inline inside a function body rather than held by a component with a
  defined shutdown.
- [ ] Check that every manually created scope has a corresponding `cancel()` call in a teardown path.
- [ ] Flag `runBlocking` in production code paths, particularly inside suspend functions or on request-handling threads.
- [ ] Look for suspend functions that switch dispatchers internally with `withContext(Dispatchers.IO)` on every call,
  and verify dispatchers are injected rather than hardcoded, which otherwise blocks testing with a test dispatcher.
- [ ] Check that `async` results are always awaited, since an un-awaited `Deferred` hides its exception.
- [ ] Flag `SupervisorJob` used where failure of a child should actually fail the parent, which silently converts a
  fatal error into a partial result.
- [ ] Look for shared mutable state accessed from multiple coroutines without a mutex, actor, or confinement to a single
  dispatcher thread.

### Mitigation

Bind scopes to components that already have a lifecycle and cancel them in the same place the component is disposed.
Inject `CoroutineDispatcher` as a constructor parameter so tests substitute a deterministic dispatcher. Use
`coroutineScope` and `supervisorScope` inside suspend functions for local parallelism, which guarantees children
complete or cancel before the function returns.

## Explicit API Surface

**ID: `kotlin/explicit-api-surface`**

Public declarations are a commitment. Visibility is narrowed to the smallest scope that works, and modules expose
interfaces rather than implementations, so internal refactoring cannot break consumers.

### How to validate

- [ ] Check whether shared library modules enable `explicitApi()` in their Kotlin compiler options.
- [ ] Flag public classes and functions used only within their own module that lack the `internal` modifier.
- [ ] Look for public properties exposing implementation types, for example returning a concrete `ArrayList` or a
  framework-specific type from a domain module.
- [ ] Flag `open` classes and methods without a documented extension contract, since inheritance is a stronger
  commitment than composition.
- [ ] Check for public constructors on classes with validation requirements where a factory should control
  instantiation.
- [ ] Look for public extension functions on common types like `String` or `Any` in shared modules, which pollute
  completion and collide across dependencies.
- [ ] Flag public API returning nullable types where the nullability is not part of the contract.

### Mitigation

Enable explicit API mode in library modules so the compiler requires visibility modifiers and explicit return types on
public declarations. Default new declarations to `private`, widen to `internal` when another file in the module needs
them, and make them public only when an external consumer exists. Prefer sealed interfaces plus internal
implementations, which allows adding behavior without exposing structure.

## Deterministic and Injected Effects

**ID: `kotlin/injected-effects`**

Time, randomness, environment access, file systems, and network clients are dependencies passed into a component, never
reached for statically. Code calling `System.currentTimeMillis()` or `Random.nextInt()` directly cannot be tested
deterministically and behaves differently under load, in other time zones, and across environments.

### How to validate

- [ ] Flag direct calls to `System.currentTimeMillis()`, `Instant.now()`, `LocalDate.now()`, and `LocalDateTime.now()`
  outside of a clock abstraction.
- [ ] Look for `Random`, `UUID.randomUUID()`, and `Math.random()` called inside domain logic rather than injected.
- [ ] Flag `System.getenv` and `System.getProperty` outside of a single configuration loading component.
- [ ] Check for object declarations holding mutable state or performing I/O, since singletons cannot be substituted in
  tests.
- [ ] Look for `Dispatchers.Default` and `Dispatchers.IO` referenced directly inside classes rather than injected.
- [ ] Flag static factory calls to HTTP clients, database connections, or file handles inside business logic.
- [ ] Check tests for `Thread.sleep` or arbitrary timeouts, which indicate a non-injected time source.

### Mitigation

Inject `java.time.Clock` and read time through it, which makes tests use `Clock.fixed`. Wrap identifier generation
behind a small interface so tests produce predictable values. Keep configuration reading in one component that produces
a validated immutable configuration object, and pass that object rather than letting arbitrary code query the
environment.

## Sequence and Collection Discipline

**ID: `kotlin/collection-discipline`**

Collection pipelines allocate an intermediate list at every step. Chains over large inputs use `Sequence` or
`asSequence()` to fuse operations, and pipelines that terminate early use lazy evaluation so the remaining elements are
never touched.

### How to validate

- [ ] Look for chains of three or more collection operators such as `map { }.filter { }.map { }` over inputs whose size
  is unbounded or externally controlled.
- [ ] Flag `asSequence()` on small fixed-size collections, where the wrapper overhead exceeds the benefit.
- [ ] Check for `sortedBy { }.first()` where `minByOrNull` performs the same work in linear time.
- [ ] Look for `count() > 0` instead of `isNotEmpty()`, and `filter { }.size` instead of `count { }`.
- [ ] Flag `toList()` calls in the middle of a pipeline that force materialization without a reason.
- [ ] Check for `contains` in a loop over a `List` where a `Set` would give constant time lookup.
- [ ] Look for `flatMap` producing large intermediates where `flatMapTo` or a sequence would avoid them.
- [ ] Flag terminal operations inside loops, which turns a linear pipeline quadratic.

### Mitigation

Choose the data structure by access pattern before choosing the operators, using `Set` for membership and `Map` for
keyed lookup. Convert to a sequence when the chain has multiple stages and the input can grow, and stay with eager
collections for small known-size data. Use the dedicated operators such as `any`, `none`, `firstOrNull`, `minByOrNull`,
and `sumOf`, which express the intent and avoid a full traversal.

## Boundary Validation and Safe Serialization

**ID: `kotlin/boundary-validation`**

Data crossing a trust boundary is parsed into domain types with validation, not deserialized directly into objects the
rest of the system treats as trusted. Deserialization that binds unvalidated input to domain models allows invalid
state, and reflective or polymorphic deserialization of untrusted payloads permits instantiation of unintended types.

### How to validate

- [ ] Look for external payloads deserialized directly into domain entities rather than into dedicated transport
  objects.
- [ ] Flag polymorphic deserialization configured with type information taken from the payload, especially Jackson
  `enableDefaultTyping` or equivalent, when the input is not fully trusted.
- [ ] Check that `@Serializable` classes used for input have their invariants enforced in an `init` block, since
  kotlinx.serialization bypasses secondary construction logic but does execute `init`.
- [ ] Look for nullable fields in transport objects mapped to non-null domain fields with `!!` rather than a validating
  conversion.
- [ ] Flag string interpolation used to build SQL, shell commands, file paths, or HTML from external input.
- [ ] Check that secrets are not held in `String` fields, logged, or included in `data class` output, since the
  generated `toString` prints every property.
- [ ] Look for size, range, and format limits missing on collections and strings coming from the network.

### Mitigation

Define transport types separate from domain types and write an explicit conversion that returns a typed validation
result. Enforce invariants in `init` blocks so no instance exists in an invalid state regardless of construction path.
Override `toString` on any type holding credentials or personal data, and use parameterized queries and process argument
arrays rather than string concatenation for anything sent to an interpreter.
