# Properties

This chapter is a focused overview. For the current reference, including expression support and `EnvPropertyResolver` behavior, see [Properties and Resources](06-properties-and-resources.md).

## Static Properties

Use the `value` tag to inject property values into bean fields.

```go
type config struct {
    Host string `value:"server.host"`
    Port int    `value:"server.port,default=8080"`
}
```

Supported types: `string`, `bool`, `int` variants, `uint` variants, `float32`, `float64`, `time.Duration`, `time.Time`, `os.FileMode`, and slices of these types (semicolon-separated).

### Default Values

```go
type config struct {
    Host    string        `value:"server.host,default=localhost"`
    Timeout time.Duration `value:"server.timeout,default=30s"`
}
```

If the property is not found and no default is provided, container creation fails with an error.

### Time Layout

```go
type config struct {
    StartDate time.Time `value:"app.start,layout=2006-01-02"`
}
```

Default time layout is `time.RFC3339`.

### Slices

```go
type config struct {
    Hosts []string `value:"server.hosts"`
}
```

Values are separated by semicolons: `server.hosts=host1;host2;host3`.

## Property Expressions

Glue supports `${...}` placeholders in property values.

```properties
app.name=myapp
app.log.dir=/var/log/${app.name}
app.port.override=9090
app.port=${app.port.override:8080}
```

Important behavior:
* `Properties.Get(key)` returns the raw value
* `Properties.Resolve(key)` returns the resolved value
* `value:"..."` injection uses resolved values
* dynamic property functions also resolve expressions on each call

To resolve env-style placeholders such as `${APP_PORT:8080}`, use `EnvPropertyResolver` with `MatchKey: glue.OnlyEnvStyle`.

## Dynamic Properties

Use function-typed fields to read properties lazily on each call.

```go
type config struct {
    GetHost func() string              `value:"server.host,default=localhost"`
    GetPort func() (int, error)        `value:"server.port"`
    GetTTL  func(context.Context) (time.Duration, error) `value:"cache.ttl"`
}
```

Supported signatures:
* `func() T` — requires `default` option (cannot report errors)
* `func() (T, error)` — resolved on each call
* `func(context.Context) (T, error)` — resolved on each call with context

Dynamic properties re-resolve from the `Properties` object on each invocation, so they reflect runtime changes from `PropertyResolver` implementations.

## Property Sources

Properties can be loaded from files or maps:

```go
ctn, err := glue.New(
    &glue.PropertySource{File: "file:config.properties"},
    &glue.PropertySource{Map: map[string]any{
        "server.host": "localhost",
        "server.port": 8080,
    }},
    &config{},
)
```

Shorthand forms:

```go
ctn, err := glue.New(
    glue.FilePropertySource("file:config.yaml"),
    glue.MapPropertySource{"server.host": "localhost"},
    &config{},
)
```

Supported file formats: `.properties`, `.yaml`, `.yml`, `.json`.

File paths use a `source:path` prefix. Use `file:path` for OS filesystem files, or a `ResourceSource` name prefix for embedded resources.

## Property Resolvers

Implement `PropertyResolver` to provide properties from external sources (environment, Vault, Consul, etc.).

```go
type envResolver struct{}

func (t *envResolver) Priority() int { return 200 }

func (t *envResolver) GetProperty(key string) (string, bool) {
    envKey := strings.ReplaceAll(strings.ToUpper(key), ".", "_")
    val, ok := os.LookupEnv(envKey)
    return val, ok
}
```

Resolvers are sorted by priority (higher is checked first). The default internal storage has priority 100.

## Options-based Properties

```go
props := glue.NewProperties()
props.Set("server.host", "0.0.0.0")

ctn, err := glue.NewWithOptions([]glue.ContainerOption{
    glue.WithProperties(props),
}, &config{})
```

## Reload

`Container.Reload(bean)` re-resolves static property values and re-runs `PostConstruct`. Dynamic function properties are not affected since they already read live values.
