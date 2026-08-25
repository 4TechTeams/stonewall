---
id: php
name: PHP
extensions: [php, phtml]
---

# PHP Ruleset

## Type Integrity at Boundaries

**ID: `php/type-integrity`**

PHP's gradual typing only pays off when types are declared everywhere and enforced at runtime. Without
`declare(strict_types=1)`, PHP coerces `"5 apples"` to `5` for a parameter declared `int`, which turns a bad request
into corrupted data. External payloads are mapped into typed structures at the edge, so nothing downstream re-checks.

### How to validate

- [ ] Check that every file opens with `declare(strict_types=1);` before any other statement.
- [ ] Flag parameters, return types, and properties with no declared type. Constructor property promotion counts.
- [ ] Flag `mixed` in domain code as a parameter or return type. Each occurrence needs a reason in a comment or a
  narrower type.
- [ ] Look for a bare `array` type with no `@param`, `@return`, or `@var` describing its shape. Where the array is
  really a value with rules, the fix is a class, not a longer annotation.
- [ ] Flag `@phpstan-ignore`, `@psalm-suppress`, and inline `/** @var */` casts used to silence an analyzer with no
  comment explaining why.
- [ ] Look for service methods taking a raw request payload as `array $data` rather than a typed command or DTO.
- [ ] Check that uploaded files are validated by inspected content rather than by the client-supplied
  `$_FILES['x']['type']` or the filename extension, and that they land outside the web root.

### Mitigation

Make `declare(strict_types=1)` part of the file template and enforce it with a linter rule. Adopt PHPStan at the level
the codebase already passes, generate a baseline, and forbid the baseline from growing. Type new code fully and add
types to legacy files whenever they are touched. Map each entry point into a typed command object so everything
downstream can assume the data is already correct.

## Value Objects over Primitives

**ID: `php/value-objects-over-primitives`**

Domain concepts that carry rules are modeled as readonly value objects, not passed around as `string` or `int`. An
`Email`, `Money`, or `UserId` validates itself in its constructor, so an invalid instance cannot exist anywhere. That
removes defensive validation at every call site and makes a swapped argument a type error rather than an incident.

### How to validate

- [ ] Look for two or more adjacent parameters of the same scalar type where the domain has distinct concepts, for
  example `function transfer(string $from, string $to, int $amount)`.
- [ ] Check whether validation of one concept, such as an email format check, is repeated in more than one file. The
  duplication is the signal that the type is missing.
- [ ] Flag money and other decimal quantities held as `float`, and any arithmetic on them, since binary floating point
  cannot represent decimal currency exactly.
- [ ] Check that value objects compare through an explicit `equals()` rather than leaving call sites to choose between
  `==` and `===`.
- [ ] Flag value object constructors that return early, return `null`, or return `false` on invalid input instead of
  rejecting it.
- [ ] Look for identifiers, quantities, and durations declared as `string`, `int`, or `float` in domain signatures.

### Mitigation

Start with the concepts that appear most often in bug reports, usually identifiers, money, and dates. Give each a final
readonly class with a private constructor and named factories such as `Money::fromCents()` or `Email::fromString()`.
Push validation into the constructor and delete the scattered checks it replaces. For money use integer minor units or
a decimal library, never floats.

## Immutability by Default

**ID: `php/immutability-by-default`**

Objects are immutable unless mutation is the point. Transitions return a new instance through `with*` methods instead
of setters, which makes state changes traceable and safe to share. Aggregates expose intent, so `$order->cancel($reason)`
rather than `$order->setStatus('cancelled')`, which puts illegal transitions in one place where they can be rejected.

### How to validate

- [ ] Flag public setters on entities, DTOs, and value objects.
- [ ] Flag a `with*` method that assigns to `$this` instead of returning a clone or a new instance.
- [ ] Look for state fields assigned from outside the owning class, for example `->status =` in a service.
- [ ] Flag closed sets of states held as string constants and compared by equality, where a backed enum belongs.
- [ ] Look for a collection property mutated in place, such as `$this->items[] =`, on a class documented as immutable.
- [ ] Check that classes are declared `readonly`, or hold `readonly` properties, wherever nothing legitimately changes
  after construction.

### Mitigation

Declare new classes `final` with `readonly` properties from the start, since relaxing that later is easy and tightening
it is not. Replace setter chains with a constructor taking everything required, plus `with*` methods for the few fields
that genuinely change. Model states as a backed enum and put the allowed transitions in one method that throws a domain
exception on an illegal move.

## Exhaustiveness

**ID: `php/exhaustiveness`**

Closed sets of states are backed enums consumed through `match`, which throws `UnhandledMatchError` when a new case
appears rather than falling through silently. A `switch` with no arm for a new case simply does nothing, and a `default`
arm turns the addition of a state into a runtime surprise instead of a visible failure at the first call site.

### How to validate

- [ ] Flag `switch` over an enum or a closed set of states where `match` would fail loudly instead of falling through.
- [ ] Flag a `default` arm on a `match` over an enum that produces a fallback value, logs a warning, or throws a
  generic exception.
- [ ] Look for `if`/`elseif` chains testing a status or a type over a closed set, which bypasses exhaustiveness
  entirely.
- [ ] Flag string or integer constants standing in for a closed set of states, for example `status: string` compared
  against literals.
- [ ] Check that a `match` over an enum lists every case explicitly rather than collapsing several into a catch-all
  arm that will absorb future cases too.

### Mitigation

Replace string constants with a backed enum so the set of states is a type. Consume it with `match` and assign or
return the result, so every case must be covered. When a `default` arm feels necessary, that usually means the
parameter type is wider than the call site needs, so narrow the type instead of adding a fallback.

## Exceptions Model Domain Failure

**ID: `php/exception-discipline`**

Failure modes are typed exceptions in a domain hierarchy, not `false`, `null`, `-1`, or an array with an `error` key.
Each bounded context has a base exception interface so callers can catch at the right granularity. Catch blocks are
narrow and never empty, and expected outcomes such as "no user found" are modeled as a nullable return or a result
type rather than as an exception.

### How to validate

- [ ] Flag empty catch blocks, and catches whose body is only a comment or a bare `return null;` with no logging.
- [ ] Flag the error suppression operator, for example `@file_get_contents(` or `@unlink(`.
- [ ] Flag `catch (\Exception)` or `catch (\Throwable)` anywhere but a top-level handler, a worker loop, or a
  documented boundary. Each one must rethrow, wrap, or log with the original attached.
- [ ] Look for an exception thrown inside a `catch` without passing the caught exception as `previous`, which discards
  the original stack trace.
- [ ] Flag `throw new \Exception(` and `throw new \RuntimeException(` in domain code where a named type belongs.
- [ ] Flag a return type that mixes a value with a failure sentinel, such as `Order|false`.
- [ ] Check that the bootstrap installs `set_error_handler` to convert notices and warnings into `ErrorException`, so a
  warning cannot pass unnoticed.

### Mitigation

Define one interface per module, for example `BillingException extends \Throwable`, and let concrete exceptions
implement it. Convert boolean or null failure returns into exceptions where the caller can never sensibly proceed, and
into an explicit result object where both outcomes are normal. Keep one top-level handler that logs with full stack
traces and renders a safe response, then delete the defensive catches scattered below it.

## Dependencies Are Injected, Never Reached For

**ID: `php/explicit-dependencies`**

Every dependency arrives through the constructor and is held in a typed readonly property. Service locators, static
facades, `global`, singletons, and `new` inside business logic hide the real dependency graph and make a class
untestable without booting the framework. The clock, randomness, and the environment are dependencies too.

### How to validate

- [ ] Flag `new` of an infrastructure or service class inside a method body. Constructing value objects and DTOs is
  fine.
- [ ] Flag static access to services, including `::getInstance(`, `global $`, `$GLOBALS[`, and framework facades used
  inside domain classes.
- [ ] Flag container access outside the composition root, such as `$container->get(`, `app(`, or `resolve(` inside an
  application or domain service.
- [ ] Flag time read directly in business logic: `new \DateTimeImmutable()` with no argument, `time()`, `date(`,
  `strtotime('now')`, anywhere but a clock implementation.
- [ ] Flag identifier and randomness generation reached for statically, such as `rand(`, `mt_rand(`, `uniqid(`, or a
  UUID factory called inside domain logic.
- [ ] Flag superglobals read outside the HTTP boundary, including `$_GET`, `$_POST`, `$_SERVER`, `$_SESSION`, and
  `getenv(` in domain, application, or repository code.
- [ ] Flag mutable static state, for example a public static property or a static assignment used as a cache with no
  stated invalidation.

### Mitigation

Use constructor promotion with readonly properties so wiring stays short. Move instantiation into the container
configuration, which becomes the single description of how the system is assembled. Introduce a `ClockInterface` and an
id generator interface early, since retrofitting them means touching every test that fails at midnight. More than
about five dependencies is a signal the class holds several responsibilities.

## Injection at the Sink

**ID: `php/injection-at-sinks`**

Every value is encoded for the specific interpreter it flows into. String concatenation into SQL, HTML, a shell, or a
path is the root cause of injection, and no amount of input filtering substitutes for correct encoding at the sink,
because the sink is where the value acquires meaning.

### How to validate

- [ ] Flag SQL built by concatenation or interpolation, including a `prepare(` whose string still interpolates a
  variable.
- [ ] Flag table names, column names, and sort directions taken from request data. Dynamic identifiers cannot be bound
  as parameters, so they need an allowlist.
- [ ] Look for unescaped output in templates, such as `echo $` or `<?= $` with no escaping helper, and raw output
  markers applied to data that is not known-safe HTML.
- [ ] Flag `exec(`, `shell_exec(`, `system(`, `passthru(`, `proc_open(`, and backticks carrying an interpolated value.
  Every such value needs `escapeshellarg`.
- [ ] Flag file operations on a user-supplied path, including `fopen(`, `include`, `require`, and `unlink(`, unless the
  path is resolved with `realpath` and checked against an allowlisted base directory.
- [ ] Check that `htmlspecialchars` is called with an explicit charset, and that the template layer autoescapes by
  default rather than relying on each call site.

### Mitigation

Use prepared statements with bound parameters everywhere, and keep a small allowlist for the identifiers that must be
dynamic, such as sortable columns. Use an autoescaping template engine so escaping is the default rather than a step
someone can forget. Pass process arguments as an array rather than building a command string. Resolve every
user-influenced path and reject anything that escapes the base directory.

## Dynamic Code and Deserialization

**ID: `php/dynamic-code`**

Constructs that turn data into code erase the boundary between input and program. `unserialize()` on untrusted input is
remote code execution, because the payload chooses which classes to instantiate and which magic methods run. Variable
functions, variable class names, and `extract()` hand control of the call graph to whoever supplied the string.

### How to validate

- [ ] Flag `eval(` anywhere.
- [ ] Flag `unserialize(` on anything reaching the process from a request, a cache, a cookie, or a queue.
- [ ] Flag `extract(`, which creates variables whose names come from the data.
- [ ] Flag variable functions and variable class names built from input, such as `$$var`, `$fn($x)` where `$fn` traces
  back to a request, or `new $class`.
- [ ] Look for callables assembled from request data and passed to `call_user_func`, `array_map`, or a router.
- [ ] Flag `create_function`, and dynamic `include`/`require` where the path is assembled rather than chosen from a
  fixed set.

### Mitigation

Replace `unserialize` with JSON decoding into a typed structure. Where a class must be chosen at runtime, map the input
through an explicit array of permitted values so an unknown key is a rejection rather than an instantiation. Replace
`extract()` with explicit assignment. Nothing that arrives from outside the process should ever name a function, a
class, or a file.

## Credential Handling

**ID: `php/credential-handling`**

Secrets are hashed with a purpose-built algorithm, compared in constant time, kept out of the repository, and kept out
of every output channel. A password compared with `===` leaks its length and prefix through timing, and a secret in a
log or an exception message outlives the request in a system with weaker access control than the one it protects.

### How to validate

- [ ] Flag `md5(`, `sha1(`, and general-purpose hashes applied to a password. Password storage uses `password_hash` and
  `password_verify` with the default algorithm.
- [ ] Flag secret comparison with `==` or `===`. Tokens, signatures, and MAC values need `hash_equals`.
- [ ] Look for high-entropy literals assigned to names such as key, secret, token, password, or dsn, and for
  credentials embedded in a connection string.
- [ ] Check that secrets never reach a log call, an exception message, or a `__toString`, including through whole
  request or response bodies passed as log context.
- [ ] Flag credentials read from anywhere but one configuration component, so rotation has a single place to change.
- [ ] Look for tokens and keys with no stated lifetime or rotation path where the value is long-lived by default.

### Mitigation

Store secrets in environment variables loaded once at bootstrap into a validated configuration object, and pass that
object rather than letting arbitrary code read the environment. Override `__toString` and any serialization hook on
types holding credentials. Log identifiers, never payloads. Where a value must be compared, compare it in constant
time.

## Persistence Is an Explicit, Transactional Boundary

**ID: `php/persistence-boundary`**

Database access lives behind repositories with intention-revealing methods, not query builder calls scattered through
controllers and templates. Writes that must succeed together run in one transaction, and transactions do not wrap
network calls or long computations. Queries inside loops are defects, since N+1 dominates real latency problems.

### How to validate

- [ ] Flag query construction in controllers, templates, and domain entities. Raw PDO, query builders, and ORM facades
  belong in the infrastructure layer.
- [ ] Look for a repository call, a `find(`, or a lazy relation access inside a `foreach`, `for`, or `while` body.
- [ ] Flag two or more write calls in one method with no surrounding transaction.
- [ ] Look for a transaction opened without a rollback on the failure path and a commit on the success path.
- [ ] Flag HTTP calls, queue dispatches, mail sends, and `sleep` inside a transaction block.
- [ ] Flag unbounded reads such as `findAll(` or a select with no limit against a table that grows with usage.
- [ ] Flag a migration file that is modified rather than added, since it has already run in shared environments, and
  flag schema changes performed from application code at runtime.
- [ ] Check that repository interfaces live in the domain layer while implementations live in infrastructure, so domain
  files do not import ORM classes.

### Mitigation

Give each aggregate one repository with a small named API such as `ordersAwaitingShipment()` rather than exposing a
query builder. Open the transaction at the application service level so domain code stays unaware of it, and dispatch
side effects after commit. Add a per-request query count assertion in tests so an N+1 regression fails before it ships.

## Cohesive, Final, Composed Units

**ID: `php/cohesion-and-composition`**

Classes are `final` by default and composed rather than extended, since inheritance exposes internals and freezes a
hierarchy that later requirements will not fit. Classes named `Manager`, `Helper`, or `Util` are a symptom of a missing
concept rather than a design. Public surface is kept small so refactoring stays local.

### How to validate

- [ ] Flag classes that are neither `final` nor `abstract` and have no documented extension contract.
- [ ] Flag `Helper`, `Util`, `Manager`, and `Handler` classes holding unrelated static methods, which is a namespace
  wearing a class as a costume.
- [ ] Flag `parent::` used to reuse an implementation rather than to extend a template method.
- [ ] Look for the same `instanceof` chain or status branch repeated across several files, which names a polymorphic
  method or enum behaviour that does not exist yet.
- [ ] Look for a method that calls several getters on one collaborator and touches no state of its own, since the
  logic belongs on the collaborator.
- [ ] Flag traits that declare properties and are used by unrelated classes, which is shared mutable state without the
  honesty of inheritance.
- [ ] Check that declarations used only inside their own namespace are not public.

### Mitigation

Extract a named class the moment a method needs a comment to explain its sections. Replace type switches with a method
on an enum or a small strategy interface resolved by a factory. When two classes need the same code, extract a
collaborator and inject it rather than pulling the code into a shared parent or a trait.

## Deterministic Tests

**ID: `php/test-determinism`**

A test that reaches for real time, the real network, or state left by another test fails for reasons unrelated to the
code it covers, and a suite that does this teaches the team to ignore red. A test that asserts on internals breaks on
every refactor, which makes the suite an obstacle to the change it was meant to protect.

### How to validate

- [ ] Flag `ReflectionProperty` and `setAccessible(true)` in test files, which assert on private state.
- [ ] Flag `sleep(` and `usleep(` in tests, and assertions comparing against the current time rather than an injected
  fixed clock.
- [ ] Flag real network access in tests, including `curl_`, `file_get_contents('http`, and an HTTP client built with no
  mock handler.
- [ ] Flag static properties on test classes and `@depends` chains, both of which make a test depend on execution
  order.
- [ ] Flag a test with no assertion, a test whose only assertion is `assertNotNull`, and `assertTrue` applied to a
  compound expression that hides which part failed.
- [ ] Look for tests where every collaborator is mocked and the assertions only verify that mocks were called, which
  tests the wiring rather than the behaviour.

### Mitigation

Inject a fixed clock and a seeded random source so time-dependent logic is reproducible. Use transactional fixtures
rolled back per test rather than shared seed data. Reserve mocks for outbound ports such as payment gateways and mail
transports, and let everything the subject owns be real. Name a test after the rule it protects, so a failure points at
a requirement rather than at a method.

## Runtime Resilience and Resource Lifecycle

**ID: `php/runtime-resilience`**

Code that talks to anything outside the process assumes it will be slow, fail, or return garbage. Every outbound call
sets an explicit timeout, because the default is often unbounded and one slow dependency then consumes every worker.
Long-running processes release what they acquire, since the request-scoped cleanup PHP relies on does not happen there.

### How to validate

- [ ] Flag HTTP client construction and requests with no connect timeout and no read timeout.
- [ ] Flag retry loops around non-idempotent operations such as payment capture or order creation, and retries with no
  backoff or no maximum attempt count.
- [ ] Flag `fopen(` with no matching close on every path, and loops that append to an array with no bound.
- [ ] Flag whole-file reads of data whose size is not controlled, where streaming or a line reader belongs.
- [ ] Look for a worker or command iterating an entire result set where chunking or a cursor is available.
- [ ] Flag `var_dump(`, `print_r(`, and `echo` used for diagnostics outside CLI code.
- [ ] Flag `ini_set('memory_limit'` and `set_time_limit(0)` used to work around unbounded processing rather than
  bounding it.
- [ ] Check that log severity means something: no error level for an expected validation failure, and no info level
  for a genuine fault.

### Mitigation

Wrap each third party integration in an adapter that owns its timeout, retry policy, and error translation, so the rest
of the codebase sees a domain-shaped interface. Add a circuit breaker or a cached fallback for dependencies whose
outage should not take the whole system down. Use a PSR-3 logger with structured context arrays rather than message
interpolation, and stream large payloads through generators so memory stays flat regardless of input size.
