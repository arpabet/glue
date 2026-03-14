# Glue Framework Evolution Plan (v2 — Revised)

## Current State Assessment

Glue is already the most feature-rich runtime DI framework for Go. The framework covers:

### Implemented Features

| Feature | Status |
|---------|--------|
| Pointer and interface injection with struct auto-wrap | Done |
| Qualifier, primary, optional, lazy injection | Done |
| Bare `inject` tag (no `:""`  needed) | Done |
| Collection injection (slice + map + ordered) | Done |
| PostConstruct / Destroy lifecycle with context variants | Done |
| FactoryBean / ContextFactoryBean with singleton/prototype | Done |
| Singleton, prototype, request scopes with ScopedBean | Done |
| Profiles with expression syntax (`!`, `\|`, `&`) and IfProfile | Done |
| Conditional beans (ConditionalBean.ShouldRegisterBean) | Done |
| Parent-child containers with search levels | Done |
| Property sources, resolvers, expressions (`${key:default}`) | Done |
| EnvPropertyResolver with prefix, key mapping, match gate | Done |
| Dynamic properties via `func() T`, `func() (T, error)`, `func(ctx) (T, error)` | Done |
| Dynamic property error handler (zero value + callback, no panic) | Done |
| Generic API: GetBean[T], GetBeans[T], GetProperty[T], factories | Done |
| Decorator with OrderedBean sorting and field update | Done |
| Gluegen (component scanning + decorator proxy generation) | Done |
| DOT-format dependency graph via Container.Graph() | Done |
| PrimaryBean for ambiguous resolution | Done |
| Bean reload for static property re-injection | Done |
| BeanDef caching (globalBeanDefCache via sync.Map) | Done |
| Interface candidate caching (interfaceCache per container) | Done |
| Per-container ContainerLogger with inheritance | Done |

### Architecture Decisions (Why Things Are the Way They Are)

**Interface caching (no separate index needed):** The `interfaceCache` already solves the O(n*m)
problem. Interface types are not known until injection fields are parsed, so we cannot pre-build
an index during scanning. The cache is populated on first lookup per interface type and reused
for all subsequent lookups. This is the right design.

**Property resolver chain (no caching needed):** The `Properties` in-memory store is already a
direct `map[string]string` lookup. `PropertyResolver` implementations (like cloud secret managers)
have their own caching and refresh logic — Glue should not second-guess their caching decisions.
The resolver chain is short (typically 2-3 resolvers), making the iteration cost negligible.

**Dynamic config via lazy properties (no push-based watcher needed):** Dynamic `func() T` fields
read the current property value on every call. The application controls when to request values.
The `PropertyResolver` decides whether to hit the secret manager or return a cached value.
This pull-based model is simpler, avoids data races, and avoids wasted refresh work for
properties that may never be read. It is better than push-based notification because:
- No wasted work refreshing properties nobody reads
- No data races from re-injecting static fields
- PropertyResolver owns its caching policy (TTL, circuit breaker, etc.)
- Application code is explicit about when it needs the value

---

## Remaining Improvements

After careful evaluation, the following items are genuinely valuable and not overhead:

### 1. Property Map Injection with Prefix

**Status:** Evaluate

**Problem:** Sometimes an application needs all properties under a prefix as a `map[string]string`
or `map[string]any`. This is common when passing a config section to a third-party library,
forwarding properties to a child container, or when the key set is not known at compile time.

**Current workaround:** Manually call `Properties.Keys()` + `Properties.Get()` in PostConstruct
to filter and collect. This works but is boilerplate.

**Proposed approach:** Support `map[string]string` field type with `value:"prefix=db"` tag:

```go
type myService struct {
    // Collects all properties starting with "db." into a map,
    // stripping the prefix from keys.
    DBProps map[string]string `value:"prefix=db"`
}
```

If properties are:
```
db.host = localhost
db.port = 5432
db.user = admin
app.name = myapp
```

Then `DBProps` would be `{"host": "localhost", "port": "5432", "user": "admin"}`.

**Evaluation criteria:**
- Is this a real use case in current applications? If yes, implement.
- Does it add complexity to the tag parser? Minimal — it's a new option alongside `default=`
  and `layout=`.
- Can it be done in PostConstruct instead? Yes, but it's boilerplate every time.

**Implementation (if approved):**
- In `bean.go`: When parsing `value` tag and field type is `map[string]string`, check for
  `prefix=` option. Create a new `propInjectionDef` variant with `isMapPrefix: true`.
- In `injection.go`: For map prefix injection, iterate `Properties.Keys()`, filter by prefix,
  strip prefix from keys, populate the map.
- For `map[string]any`, support type conversion per value (string, int, bool auto-detection).

**Files:** `bean.go`, `injection.go`
**Tests:** `prefix_map_test.go`

**Effort:** Small
**Risk:** Low — additive change, no impact on existing behavior

---

### 2. BeanPostProcessor

**Status:** Implement

**Problem:** Decorator wraps a specific interface type and replaces the bean with a proxy.
But some cross-cutting concerns need to inspect ALL beans without wrapping them:
- Register beans that match a pattern with an external system (metrics, event bus, HTTP router)
- Validate beans against application-specific rules
- Inject non-DI values (trace context, configuration) based on bean type
- Log or audit bean construction

Decorator cannot do this because it targets one interface at a time and replaces the bean.
A post-processor observes all beans without replacing them.

**Design:**

```go
// api.go

var BeanPostProcessorClass = reflect.TypeOf((*BeanPostProcessor)(nil)).Elem()

// BeanPostProcessor is called for every non-processor bean after injection
// is complete but before PostConstruct. It receives the fully injected bean
// and can inspect, validate, or register it with external systems.
//
// PostProcessors are collected during scanning and applied in OrderedBean order.
// A PostProcessor must NOT replace the bean — use Decorator for that.
// Returning an error fails container creation.
type BeanPostProcessor interface {
    PostProcessBean(bean any, name string) error
}
```

**Lifecycle position:**

```
Scan → Inject → Decorate → PostProcess → PostConstruct → Ready
```

PostProcessors run after decorators (so they see the final decorated value) but before
PostConstruct (so beans aren't yet active when post-processing happens).

**Example 1: Auto-register HTTP handlers**

```go
type handlerRegistrar struct {
    Router *router `inject`
    count  int
}

func (r *handlerRegistrar) PostProcessBean(bean any, name string) error {
    if h, ok := bean.(HTTPHandler); ok {
        r.Router.Handle(h.Pattern(), h)
        r.count++
    }
    return nil
}

func (r *handlerRegistrar) BeanOrder() int { return 100 } // run after other post-processors
```

**Example 2: Validate configuration**

```go
type configValidator struct{}

func (v *configValidator) PostProcessBean(bean any, name string) error {
    if c, ok := bean.(Configurable); ok {
        if err := c.Validate(); err != nil {
            return fmt.Errorf("bean '%s' has invalid configuration: %w", name, err)
        }
    }
    return nil
}
```

**Example 3: Metrics registration**

```go
type metricsRegistrar struct {
    Registry *prometheus.Registry `inject`
}

func (r *metricsRegistrar) PostProcessBean(bean any, name string) error {
    if c, ok := bean.(prometheus.Collector); ok {
        r.Registry.MustRegister(c)
    }
    return nil
}
```

**Implementation:**

In `container.go`, after `applyDecorators()` returns and before the `constructBean` loop
calls PostConstruct:

```go
func (t *container) applyPostProcessors() error {
    // collect all BeanPostProcessor beans
    var processors []BeanPostProcessor
    for _, beans := range t.core {
        for _, b := range beans {
            if p, ok := b.obj.(BeanPostProcessor); ok {
                processors = append(processors, p)
            }
        }
    }
    if len(processors) == 0 {
        return nil
    }

    // sort by OrderedBean
    sort.SliceStable(processors, func(i, j int) bool {
        oi, iOk := processors[i].(OrderedBean)
        oj, jOk := processors[j].(OrderedBean)
        if iOk && jOk {
            return oi.BeanOrder() < oj.BeanOrder()
        }
        return false
    })

    // apply to every non-processor bean
    for _, beans := range t.core {
        for _, b := range beans {
            if _, isProcessor := b.obj.(BeanPostProcessor); isProcessor {
                continue
            }
            for _, p := range processors {
                if err := p.PostProcessBean(b.obj, b.name); err != nil {
                    return errors.Errorf("post-processor %T failed for bean '%s': %v",
                        p, b.name, err)
                }
            }
        }
    }
    return nil
}
```

**Interaction with existing features:**
- **Decorator:** Runs before PostProcessor. Processor sees decorated beans.
- **PostConstruct:** Runs after PostProcessor. Beans are not yet initialized when processed.
- **FactoryBean products:** Post-processed like any other bean after factory creates them.
- **PostProcessor depends on other beans:** PostProcessors can have `inject` fields.
  They are injected during the normal injection phase. Their own PostConstruct runs
  in the normal lifecycle after all post-processing is done.

**Ordering:** PostProcessors implementing `OrderedBean` are sorted by `BeanOrder()`.
Unordered processors run after ordered ones.

**Files:** `api.go`, `container.go` (or new `postprocessor.go`)
**Tests:** `postprocessor_test.go`

**Effort:** Small-Medium
**Risk:** Low — additive, no impact on existing behavior

---

### 3. Example Application

**Status:** Implement (low priority, high documentation value)

Create a minimal but realistic example showing Glue's full feature set in one place.

```
examples/
  webapp/
    main.go           # HTTP server, container creation, graceful shutdown
    config.go         # PropertySource from YAML + EnvPropertyResolver
    scan.go           # GlueGen scanner (generated)
    handlers.go       # HTTP handler beans with inject tags
    services.go       # Business logic with decorator for logging
    repository.go     # Database repository
```

**Features demonstrated:**
- Profile-based bean selection (dev vs prod database)
- Dynamic `func() string` for secret that can change at runtime
- Request-scoped bean for per-request transaction
- Decorator for service-layer logging
- Property expressions with env var fallback
- Collection injection for HTTP handlers
- `Container.Graph()` for debugging

**Effort:** Medium
**Risk:** None — separate directory, no impact on framework

---

### 4. Integration Tests

**Status:** Implement (alongside example)

Add integration tests that exercise feature combinations:

```go
// integration_test.go

// Tests the full lifecycle: profiles + scopes + decorators + dynamic properties
func TestIntegration_FullLifecycle(t *testing.T) { ... }

// Tests reload with property changes + dynamic func() T fields
func TestIntegration_DynamicConfig(t *testing.T) { ... }

// Tests concurrent request-scoped beans under load
func TestIntegration_ConcurrentRequestScope(t *testing.T) { ... }

// Tests parent-child containers with profile inheritance
func TestIntegration_HierarchicalProfiles(t *testing.T) { ... }
```

**Effort:** Medium
**Risk:** None — test-only

---

## Non-Goals (Explicitly Rejected)

These were proposed but rejected after evaluation:

| Proposal | Reason for Rejection |
|----------|---------------------|
| Interface implementation index (1.1) | `interfaceCache` already solves this; interfaces aren't known until injection fields are parsed |
| Property resolution cache (1.2) | Properties store is already O(1) map lookup; PropertyResolvers own their caching |
| PropertyWatcher (push notification) | Pull-based via `func() T` is better — no wasted refresh, no data races, resolver owns cache policy |
| RefreshableBean | Same reasoning — `func() T` is the right pattern for dynamic config |
| Module system | Scanner + ProfileBean + PropertySource already compose; Module adds API surface without real benefit |
| Test utilities (MustNew, Override) | Tests should use real container creation — closer to production code |
| Dry-run Validate() | Creating the full context in tests is not expensive; validation without instantiation misses PostConstruct errors |
| Health checks | Application-level concern — apps define their own Component/Health interface and track stats at runtime |
| Shortened tag symbols (?, ~, @) | Decreases long-term code readability for minimal keystroke savings |
| Graceful shutdown helper | Container.CloseWithContext(ctx) already exists; signal handling is application-level |
| Constructor injection | Go has no constructors; struct-tag injection is more idiomatic |
| AOP / method interception | reflect.StructOf can't add methods; Decorator + gluegen proxies suffice |
| Cloud provider implementations | Separate modules (glue-aws, glue-gcp); core provides PropertyResolver interface |
| Custom scope registration | Singleton/prototype/request covers 99% of cases |

---

## Implementation Priority

| Priority | Item | Impact | Effort | Status |
|----------|------|--------|--------|--------|
| 1 | BeanPostProcessor | High (extensibility) | Small-Medium | Ready to implement |
| 2 | Property prefix map injection | Medium (DX) | Small | Evaluate if real use case exists |
| 3 | Example application | Medium (adoption) | Medium | Ready to implement |
| 4 | Integration tests | Medium (quality) | Medium | Ready to implement |

---

## Competitive Position

| Feature | Glue | Wire | Dig/Fx | samber/do |
|---------|------|------|--------|-----------|
| Runtime DI | Yes | No (codegen) | Yes | Yes |
| Struct tag injection (+ bare tag) | Yes | No | No | No |
| Generics API | Yes | N/A | No | Yes |
| Property injection (static + dynamic func) | Yes | No | No | No |
| Property expressions (${key:default}) | Yes | No | No | No |
| Env var resolver (built-in) | Yes | No | No | No |
| Dynamic config (lazy func properties) | Yes | No | No | No |
| Profiles with expressions | Yes | No | No | No |
| Conditions | Yes | No | No | No |
| Bean scopes (singleton/prototype/request) | Yes | No | Scopes | Transient/Lazy |
| Decorators with ordering | Yes | No | Yes (Decorate) | No |
| Bean post-processors | Planned | No | No | No |
| Factory beans (+ context-aware) | Yes | Providers | Constructors | Providers |
| Lifecycle hooks (+ context) | Yes | Cleanup | OnStart/OnStop | Shutdown |
| Container hierarchy with levels | Yes | No | Scopes | Scopes |
| Collection injection (slice + map + ordered) | Yes | No | Groups | No |
| Graph visualization (DOT) | Yes | DOT | DOT | Explain |
| Component scanning (gluegen) | Yes | Wire gen | No | No |
| Gluegen decorator proxy gen | Yes | No | No | No |
| Error handler callback | Yes | N/A | No | No |
| Bean reload | Yes | No | No | Hot swap |
| Lazy injection | Yes | N/A | No | Yes |
| Primary bean resolution | Yes | No | No | No |
| Struct auto-wrapping | Yes | N/A | No | No |

Glue is already the most complete runtime DI framework for Go. The remaining improvements
(BeanPostProcessor, prefix map injection) are refinements, not gaps. The framework's strength
is the combination of Spring-like familiarity with Go idioms — no other Go framework offers
this breadth of features in a single package.
