# Factories and Scopes

## Factory Beans

### `glue.FactoryBean`

Use `FactoryBean` when the container should create an object through explicit factory logic.

```go
type factory struct {
    Dependency *anotherComponent `inject:""`
}

func (t *factory) Object() (interface{}, error) {
    if err := t.Dependency.DoSomething(); err != nil {
        return nil, err
    }
    return &beanConstructed{}, nil
}
```

### `glue.ContextFactoryBean`

Use `ContextFactoryBean` when produced objects depend on construction context.

```go
type factory struct{}

func (t *factory) Object(ctx context.Context) (interface{}, error) {
    traceID, _ := ctx.Value("trace_id").(string)
    return &session{TraceID: traceID}, nil
}
```

Context source:
* `glue.NewWithContext(...)`
* `glue.NewWithOptions(... glue.WithContext(...))`
* `Container.ExtendWithContext(...)`
* `ChildContainer.ObjectWithContext(...)`
* `context.Background()` for runtime `Inject(...)`

Current behavior:
* `ContextFactoryBean` is preferred when both factory interfaces are implemented
* factory-produced objects are produced instances, not full managed beans
* they do not automatically receive property injection or lifecycle hooks

## Scopes

Glue supports three scopes:
* `singleton`
* `prototype`
* `request`

Constants:
* `glue.ScopeSingleton`
* `glue.ScopePrototype`
* `glue.ScopeRequest`

### Singleton

This is the default scope.

```go
type consumer struct {
    Storage *storageImpl `inject:""`
}
```

### Prototype

Each provider call creates a new instance.

```go
type consumer struct {
    NewWorker func() (*worker, error) `inject:"scope=prototype"`
}
```

Supported provider signatures:
* `func() (T, error)`
* `func(context.Context) (T, error)`

Prototype works with:
* `FactoryBean`
* `ContextFactoryBean`
* classical beans

For classical beans, Glue allocates a fresh instance, injects fields and properties, and runs `PostConstruct`.

### Request

Request scope caches one instance per `RequestScope` attached to a context.

```go
type consumer struct {
    GetSession func(context.Context) (*session, error) `inject:"scope=request"`
}

scope := glue.NewRequestScope()
reqCtx := glue.WithRequestScope(context.Background(), scope)

session, err := consumer.GetSession(reqCtx)
```

Rules:
* request scope requires `func(context.Context) (T, error)`
* if there is no `RequestScope` in the context, the provider returns an error
* same request scope returns the same instance
* different request scopes get different instances

### `glue.ScopedBean`

Classical beans can declare their own scope by implementing `glue.ScopedBean`.

```go
func (t *requestLogger) BeanScope() glue.BeanScope {
    return glue.ScopeRequest
}
```

This is useful when the bean itself owns the scope contract rather than the consumer field.
