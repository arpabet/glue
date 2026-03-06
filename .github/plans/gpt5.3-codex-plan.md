# Glue Enhancement Plan

## Vision
Make Glue the most practical Spring-style DI framework for Go by improving:
- Dynamic config and secret rotation.
- Runtime performance and startup scalability.
- Type safety and developer ergonomics.
- Composition features (profiles, scopes, scanning).
- Production readiness (integration tests, benchmarks, examples).

## Current Gaps
- `value:"..."` supports static typed injection only; function value injection for lazy/refreshable config is not implemented.
- Property lookup walks resolvers repeatedly without formal versioning/invalidation for dynamic providers.
- Context creation still performs broad reflective matching and interface candidate scanning, which grows poorly with bean count.
- Public API relies heavily on `interface{}` entry points (`New`, `Inject`, `FactoryBean.Object`).
- No first-class profile support.
- No first-class bean scopes beyond singleton and factory-specific behavior.
- No package scanning automation; registration is manual or via `Scanner`.
- Limited integration/e2e samples for real dynamic config use cases.

## Strategic Goals
1. Introduce first-class dynamic config with lazy and refreshable property access.
2. Reduce context build and lookup overhead with compiled injection plans and indexes.
3. Add typed API surface without breaking existing users.
4. Add profile and scope primitives for modern multi-environment applications.
5. Improve adoption through docs, examples, and measurable performance improvements.

## Phase 1: Dynamic Config Foundation (Highest Priority)
### 1.1 Add `PropertyProvider` Superset API
Keep `PropertyResolver` for compatibility and introduce a richer provider:
- `Get(key string) (value string, ok bool, err error)`
- `Version() uint64` (or per-key ETag) for cache invalidation
- Optional: `Watch(keys ...string) (<-chan PropertyEvent, error)`

Add adapter so existing `PropertyResolver` works unchanged.

### 1.2 Add Function Injection for `value` Fields
Support function field injection from `value` tags:
- `func() T`
- `func() (T, error)`
- Optional advanced form: `func(context.Context) (T, error)`

Tag mode options:
- `value:"secret.db.password,mode=lazy"`
- `value:"secret.db.password,mode=refreshable,ttl=30s"`
- optional future mode: `mode=watch`

### 1.3 Property Cache with Dynamic Invalidation
Introduce two-layer cache:
- Raw cache: key -> string value (+ provider version metadata)
- Typed cache: `(key, type, layout, default, options)` -> converted value

Invalidation triggers:
- Provider version changes
- Watch events
- Explicit invalidate calls

### 1.4 Public API Additions (Non-Breaking)
- `Properties.Refresh(ctx context.Context) error`
- `Properties.Invalidate(keys ...string)`
- `Properties.Version() uint64`

## Phase 2: Injection Performance and Startup Scaling
### 2.1 Compile-Time-Like Runtime Plan
During context creation:
- Build `TypeIndex`: direct type, interface implementations, qualifiers, ordered collections.
- Build `InjectionPlan` per bean once.
- Apply plan during wiring instead of repeated global reflective searches.

### 2.2 Optimize Parent Lookup
- Cache search results with level-sensitive keys.
- Distinguish positive and negative cache entries.
- Invalidate only affected keys on context extension or factory-produced beans.

### 2.3 Benchmarks and Targets
Add benchmarks for:
- Context startup with 100/1000/5000 beans.
- Interface resolution latency.
- Property conversion with and without cache.

Target: significant startup and lookup reduction under high bean counts.

## Phase 3: Type Safety and API Evolution
### 3.1 Generic Helper APIs
Add typed helpers while preserving existing API:
- `Get[T any](ctx Context) (T, error)`
- `MustGet[T any](ctx Context) T`
- `Lookup[T any](ctx Context, level int) ([]T, error)`

### 3.2 Typed Factory Support
Add optional typed factory abstraction:
- `FactoryBeanOf[T any]`

Retain `FactoryBean` for backward compatibility.

### 3.3 Reduce `interface{}` in Internal Paths
Incrementally tighten internals where possible:
- typed wrappers for common pathways
- improved compile-time constraints in helper APIs

## Phase 4: Profiles and Conditional Composition
### 4.1 Profile Activation
Add context options:
- `WithProfiles("dev", "prod", "aws")`

Conditional registration wrappers:
- `glue.IfProfile("prod", beanA, beanB...)`

### 4.2 Profile-Aware Properties
Support layered property sources:
- `application.properties`
- `application-{profile}.properties`

Merge order:
- base < profile-specific < runtime providers.

## Phase 5: Bean Scopes
### 5.1 Built-In Scopes
Introduce:
- `singleton` (default)
- `prototype` (new instance per retrieval/injection)
- request/custom scope via scope context

### 5.2 Scope APIs
- `InjectWithScope(obj any, scope ScopeContext) error`
- scope-aware factory contracts for runtime-created beans

## Phase 6: Syntax and Naming Improvements
### 6.1 Tag Alias Enhancements
Maintain compatibility, add readable aliases:
- `qualifier=` alias for `bean=`
- named search constants to reduce numeric `level` confusion

### 6.2 Documentation Cleanup
- Standardize wording around qualifiers, levels, lazy/optional semantics.
- Add migration docs for new tags/options.

## Phase 7: Component Scanning by Package
Go-friendly implementation via generation:
- CLI command to scan packages and generate bean registry file (`zz_glue_scan_gen.go`).
- Keep current `Scanner` interface as manual/advanced fallback.

This avoids brittle Java-style runtime scanning and keeps startup deterministic.

## Phase 8: Templates and Property Expressions
Add property expression support:
- `${key}`
- `${key:default}`

Optional template functions:
- `env`
- provider-backed secret/template helpers

Cache compiled templates and invalidate by dependent keys.

## Phase 9: Testing, Samples, and Quality Bar
### 9.1 Integration Test Matrix
- Dynamic config refresh (provider update reflected in function-injected values).
- Secret rotation scenario (AWS/GCP-like provider mock).
- Profile activation and property overlay.
- Scoped beans lifecycle.
- Generated scan registry startup.

### 9.2 Sample Applications
- Web API with request scope.
- Dynamic secret-driven DB client.
- Multi-profile app bootstrap.

### 9.3 CI Quality Gates
- Benchmark regression checks.
- Race detector on dynamic refresh paths.
- Compatibility tests for legacy tags and APIs.

## Backward Compatibility Rules
- Existing `inject` and `value` tags must continue to work as-is.
- Existing `PropertyResolver` and `FactoryBean` remain supported.
- New features are opt-in via options/tags.
- Any behavioral changes require feature flags for one major cycle.

## Suggested Rollout
1. Deliver Phase 1 first (dynamic config + function value injection + cache invalidation).
2. Deliver Phase 2 next (injection plan and index performance work).
3. Parallelize small typed API additions from Phase 3 once core is stable.
4. Add profiles/scopes/scanning in separate incremental releases.
5. Publish migration guide and sample apps before marking features stable.

## Success Criteria
- Dynamic secret/config updates reflected without full context rebuild.
- Startup time and injection latency improved measurably on large graphs.
- New typed APIs reduce unsafe casts in user code.
- Better parity with Spring-like ergonomics while preserving Go pragmatism.
