# Getting Started

## What Glue Is

Glue is a runtime dependency injection container for Go inspired by Spring.
Beans are registered as in-memory instances, then wired by reflection using exported fields with `inject` and `value` tags.

## Container Creation

Typical startup:

```go
ctn, err := glue.New(
    logger,
    &storageImpl{},
    &configServiceImpl{},
    &userServiceImpl{},
    &struct {
        UserService UserService `inject:""`
    }{},
)
require.Nil(t, err)
defer ctn.Close()
```

Constructor variants:
* `glue.New(...)`
* `glue.NewWithContext(ctx, ...)`
* `glue.NewWithProperties(ctx, props, ...)`
* `glue.NewWithProfiles([]string{"dev"}, ...)`
* `glue.NewWithOptions([]glue.ContainerOption{...}, ...)`

Common options:
* `glue.WithContext(ctx)`
* `glue.WithProperties(props)`
* `glue.WithProfiles("dev", "local")`
* `glue.WithLogger(logger)`

## Logging

Glue uses the `ContainerLogger` interface for diagnostic logging during container creation, bean construction, property injection, and shutdown.

```go
type ContainerLogger interface {
    Enabled() bool
    Printf(format string, v ...any)
    Println(v ...any)
}
```

The `Enabled()` method allows guarding expensive log argument evaluation. When `Enabled()` returns false, the container skips log calls that involve allocations (such as building indentation strings or formatting bean descriptions).

### Using WithLogger

Pass a logger via the `WithLogger` container option:

```go
ctn, err := glue.NewWithOptions(
    []glue.ContainerOption{glue.WithLogger(myLogger)},
    &myBean{},
)
```

### Using the global Verbose (backward compatibility)

The legacy `glue.Verbose()` function sets a global `*log.Logger` that is used as a fallback when no `WithLogger` option is provided:

```go
glue.Verbose(log.Default()) // enable verbose logging globally
```

`*log.Logger` satisfies `ContainerLogger`, so it works directly. To disable logging, pass `nil`:

```go
glue.Verbose(nil) // disable
```

### Logger inheritance

Child containers created via `Extend` inherit the parent's logger unless overridden with `WithLogger`:

```go
parent, _ := glue.NewWithOptions(
    []glue.ContainerOption{glue.WithLogger(myLogger)},
    &parentBean{},
)
// child inherits myLogger
child, _ := parent.Extend(&childBean{})
```

### Custom logger example

```go
type slogAdapter struct {
    logger *slog.Logger
}

func (a *slogAdapter) Enabled() bool { return true }

func (a *slogAdapter) Printf(format string, v ...any) {
    a.logger.Info(fmt.Sprintf(format, v...))
}

func (a *slogAdapter) Println(v ...any) {
    a.logger.Info(fmt.Sprint(v...))
}

ctn, err := glue.NewWithOptions(
    []glue.ContainerOption{
        glue.WithLogger(&slogAdapter{logger: slog.Default()}),
    },
    &myBean{},
)
```

When no logger is configured (no `WithLogger`, no `Verbose`, no parent logger), a built-in `nullLogger` is used that discards all output with zero overhead.

## Supported Bean Types

Glue supports:
* pointers
* struct values (auto-wrapped to pointers)
* interfaces

Struct values passed to `glue.New()` are automatically wrapped to pointers by the container. This is useful for registering pre-built values such as external library objects or configuration structs without explicitly taking their address:

```go
type AppConfig struct {
    Host string
    Port int
}

cfg := AppConfig{Host: "localhost", Port: 8080}
ctn, err := glue.New(cfg, &myService{})
```

The container allocates a pointer and copies the value, so the result is equivalent to passing `&cfg`. Since Go already copies the struct to the heap when boxing it as `any`, there is no extra overhead.

Functions are not registered as beans; use factory beans or struct providers. Scoped providers still use function-typed fields (for `scope=prototype` / `scope=request`).

## Injection Basics

Field injection example:

```go
type app struct {
    Storage storage.Service `inject:""`
}
```

Qualifier example:

```go
type app struct {
    Storage storage.Service `inject:"qualifier=fastStorage"`
}
```

Legacy qualifier syntax is also supported:

```go
type app struct {
    Storage storage.Service `inject:"bean=fastStorage"`
}
```

Bare-name shorthand is supported too:

```go
type app struct {
    Storage storage.Service `inject:"fastStorage"`
}
```

## Collections

Slices and maps of beans are supported:

```go
type holder struct {
    Handlers []Handler          `inject:""`
    Named    map[string]Handler `inject:""`
}
```

For map injection, beans must implement `glue.NamedBean`.
For ordering in slices, beans may implement `glue.OrderedBean`.

## Lazy and Optional Injection

Lazy injection:

```go
type component struct {
    Dependency *anotherComponent `inject:"lazy"`
}
```

Optional injection:

```go
type component struct {
    Dependency *anotherComponent `inject:"optional"`
}
```

Use `lazy` to break cycles or defer initialization assumptions.
Use `optional` only when nil is a legitimate runtime state and your code checks for it explicitly.
