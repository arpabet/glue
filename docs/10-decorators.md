# Decorators

Decorators wrap existing beans transparently. They are applied after all beans are created and injected but before `PostConstruct`, so consumers see the decorated version from the start.

## Decorator Interface

```go
type Decorator interface {
    DecorateType() reflect.Type
    Decorate(original any) (any, error)
}
```

`DecorateType` returns the interface type this decorator targets. Every bean implementing that interface is passed to `Decorate` one by one. The returned value must implement the same interface.

## Basic Example

Define a service interface and implementation:

```go
type UserService interface {
    GetUser(id string) string
}

type userServiceImpl struct{}

func (s *userServiceImpl) GetUser(id string) string {
    return "user:" + id
}
```

Create a logging decorator:

```go
type loggingWrapper struct {
    delegate UserService
    log      *slog.Logger
}

func (w *loggingWrapper) GetUser(id string) string {
    w.log.Info("GetUser called", "id", id)
    return w.delegate.GetUser(id)
}

type loggingDecorator struct {
    Log *slog.Logger `inject:""`
}

func (d *loggingDecorator) DecorateType() reflect.Type {
    return reflect.TypeOf((*UserService)(nil)).Elem()
}

func (d *loggingDecorator) Decorate(original any) (any, error) {
    return &loggingWrapper{
        delegate: original.(UserService),
        log:      d.Log,
    }, nil
}
```

Register everything:

```go
ctn, err := glue.New(
    &userServiceImpl{},
    &loggingDecorator{},
    &consumer{},
)
```

The consumer receives the logging wrapper. Calls to `GetUser` are logged and then forwarded to the real implementation.

## Ordered Decorators

When multiple decorators target the same interface, implement `OrderedBean` to control application order:

```go
type authDecorator struct{}

func (d *authDecorator) BeanOrder() int             { return 1 }
func (d *authDecorator) DecorateType() reflect.Type { return userServiceClass }

func (d *authDecorator) Decorate(original any) (any, error) {
    return &authWrapper{delegate: original.(UserService)}, nil
}

type cacheDecorator struct{}

func (d *cacheDecorator) BeanOrder() int             { return 2 }
func (d *cacheDecorator) DecorateType() reflect.Type { return userServiceClass }

func (d *cacheDecorator) Decorate(original any) (any, error) {
    return &cacheWrapper{delegate: original.(UserService)}, nil
}
```

Lower `BeanOrder` is applied first. With the setup above the call chain is:

```
caller -> cacheWrapper -> authWrapper -> userServiceImpl
```

Decorators without `OrderedBean` are applied after all ordered ones, in registration order.

## Injected Field Updates

When a decorator replaces a bean, the container walks all already-injected interface fields and updates any that still point to the original value. Consumers do not need to re-resolve their dependencies.

## Error Handling

If `Decorate` returns an error, container creation fails immediately. If it returns `nil`, the container also fails with a descriptive error.

## Decorator Lifecycle

| Phase | What Happens |
|---|---|
| Scan | Decorator beans are registered like any other bean |
| Injection | Decorator fields are injected (decorators can depend on other beans) |
| Decoration | `applyDecorators()` runs — all decorator beans are collected, sorted by `BeanOrder`, and applied |
| PostConstruct | Lifecycle hooks run on the final (decorated) bean graph |
| Destroy | Normal destroy order; decorator wrappers are not managed as separate beans |

## Gluegen Proxy Generation

For interfaces marked with `//glue:decorator`, `gluegen` generates proxy structs with function-typed fields that can be intercepted at runtime via `reflect.MakeFunc`:

```go
//glue:decorator
type UserService interface {
    GetUser(id string) string
}
```

Running `gluegen ./...` generates:
* `UserServiceProxy` struct with `DoGetUser func(string) string` field
* Delegating methods that satisfy the `UserService` interface
* `NewUserServiceProxy(target UserService)` factory
* `WrapUserServiceProxy(proxy *UserServiceProxy, interceptor func(method string, args []any) []any)` for runtime interception

This separates the proxy boilerplate from the decorator logic and allows runtime method interception without manual wrapper structs.
