# Lifecycle and Context

## Initialization

### `glue.InitializingBean`

Use `PostConstruct() error` when initialization does not need a context.

```go
type component struct {
    Dependency *anotherComponent `inject:""`
}

func (t *component) PostConstruct() error {
    return t.Dependency.DoSomething()
}
```

### `glue.ContextInitializingBean`

Use `PostConstruct(ctx context.Context) error` when initialization depends on context values, deadlines, or cancellation.

```go
type component struct {
    RequestID string
}

func (t *component) PostConstruct(ctx context.Context) error {
    if requestID, ok := ctx.Value("request_id").(string); ok {
        t.RequestID = requestID
    }
    return nil
}
```

Context source:
* `glue.NewWithContext(...)`
* `glue.NewWithOptions(... glue.WithContext(...))`
* `context.Background()` when no explicit context is provided

When both lifecycle styles exist, the context-aware variant takes precedence.

## Destruction

### `glue.DisposableBean`

Use `Destroy() error` for ordinary shutdown logic.

### `glue.ContextDisposableBean`

Use `Destroy(ctx context.Context) error` when shutdown depends on context.

```go
type component struct{}

func (t *component) Destroy(ctx context.Context) error {
    return ctx.Err()
}
```

Context source:
* `Container.CloseWithContext(ctx)`
* `context.Background()` when `Close()` is used

Child containers created via `glue.Child(...)` receive the same close context when the parent is closed with `CloseWithContext(ctx)`.

## Bean Post-Processors

### `glue.BeanPostProcessor`

A `BeanPostProcessor` is called for every non-processor bean after decoration but before `PostConstruct`. Use it for cross-cutting concerns that need to inspect all beans without wrapping them.

```go
type BeanPostProcessor interface {
    PostProcessBean(bean any, name string) error
}
```

Post-processors are collected during scanning and applied in `OrderedBean` order. Returning an error fails container creation. Post-processor beans are skipped when iterating targets (processors do not process other processors).

### Auto-register HTTP handlers

```go
type handlerRegistrar struct {
    Router *router `inject`
}

func (r *handlerRegistrar) PostProcessBean(bean any, name string) error {
    if h, ok := bean.(HTTPHandler); ok {
        r.Router.Handle(h.Pattern(), h)
    }
    return nil
}
```

### Validate configuration

```go
type configValidator struct{}

func (v *configValidator) PostProcessBean(bean any, name string) error {
    if c, ok := bean.(Configurable); ok {
        return c.Validate()
    }
    return nil
}
```

### Metrics registration

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

Post-processors can have `inject` fields — they are injected during the normal injection phase. Their own `PostConstruct` runs after all post-processing is done, in the same lifecycle as all other beans.

### Lifecycle Order

```
Scan → Inject → Decorate → PostProcess → PostConstruct → Ready
```

## Reload

`Container.Reload(bean)` and `Container.ReloadWithContext(ctx, bean)` re-run static property resolution and lifecycle for ordinary managed beans.

Factory-produced objects are excluded from reload.
