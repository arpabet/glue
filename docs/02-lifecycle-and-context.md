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

## Reload

`Container.Reload(bean)` and `Container.ReloadWithContext(ctx, bean)` re-run static property resolution and lifecycle for ordinary managed beans.

Factory-produced objects are excluded from reload.
