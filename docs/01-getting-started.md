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

## Supported Bean Types

Glue supports:
* pointers
* interfaces
* functions

Struct values are not supported as registered beans. Register pointers instead.

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
