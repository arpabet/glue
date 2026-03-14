# Dynamic Properties

Static `value:"..."` fields are resolved once at container startup. Dynamic properties use function-typed fields to read the current property value on every call, supporting live configuration updates without container restart.

## Supported Signatures

Glue supports three function signatures for dynamic property fields:

| Signature | Requires Default | Error Channel |
|---|---|---|
| `func() T` | Yes | Error handler callback |
| `func() (T, error)` | No | Returned error |
| `func(context.Context) (T, error)` | No | Returned error |

All three use the same `value:"..."` tag syntax as static properties:

```go
type config struct {
    // Static — resolved once at startup
    Port int `value:"app.port,default=8080"`

    // Dynamic — resolved on every call
    GetHost   func() string                         `value:"app.host,default=localhost"`
    GetSecret func() (string, error)                `value:"db.password"`
    GetConn   func(context.Context) (string, error)  `value:"db.conn,default=mem://"`
}
```

## func() T

The simplest dynamic signature. Returns the current property value directly, with no error return.

```go
type config struct {
    GetHost func() string `value:"app.host,default=localhost"`
}
```

This signature **requires a default value** in the tag because it has no way to report a missing property. If the property key is absent and no default is provided, container creation fails.

When the property value cannot be converted to `T` (e.g. a non-numeric string for `func() int`), the function returns the zero value for `T` and reports the error via the properties error handler:

```go
ctn.Properties().SetErrorHandler(func(key string, err error) {
    log.Printf("property %s: %v", key, err)
})
```

This is consistent with how `GetBool`, `GetInt`, and other typed property getters handle conversion errors.

## func() (T, error)

Returns an error when the property is missing or cannot be converted:

```go
type config struct {
    GetSecret func() (string, error) `value:"db.password"`
}
```

No default is required — callers handle the error explicitly:

```go
secret, err := cfg.GetSecret()
if err != nil {
    return fmt.Errorf("secret unavailable: %w", err)
}
```

## func(context.Context) (T, error)

Container-aware variant. The context parameter is accepted but reserved for future use (e.g. scoped property resolution):

```go
type config struct {
    GetConn func(context.Context) (string, error) `value:"db.conn,default=mem://"`
}
```

The first parameter must implement `context.Context`. Any other parameter type is rejected at container creation.

## Live Updates

Dynamic properties read from the property store on every call. When a property is updated via `Properties.Set`, the next call to the function returns the new value:

```go
ctn, _ := glue.New(&config{})
cfg := /* injected config */

// initial value from default
fmt.Println(cfg.GetHost()) // "localhost"

// update at runtime
ctn.Properties().Set("app.host", "live.example.com")

// next call returns the updated value
fmt.Println(cfg.GetHost()) // "live.example.com"
```

This works because the generated closure calls `Properties.Resolve` on each invocation rather than capturing a snapshot.

## Supported Types

Dynamic properties support the same type conversions as static properties:
* `string`
* `bool`
* signed and unsigned integers
* `float32`, `float64`
* `time.Duration`
* `time.Time` (with `layout=...`)
* `os.FileMode`
* slices of the above using `;` as separator

## Error Handler

The `func() T` signature uses the properties error handler for conversion errors:

```go
ctn.Properties().SetErrorHandler(func(key string, err error) {
    log.Printf("property error: key=%s err=%v", key, err)
})
```

When no error handler is set, conversion errors are silently swallowed and the zero value of `T` is returned. The error handler is shared with `GetBool`, `GetInt`, `GetString`, and other typed property getters.

## Validation Rules

Invalid function signatures are rejected at container creation:

| Signature | Error |
|---|---|
| `func(string) string` | parameter must be `context.Context` |
| `func() (string, int)` | second return must be `error` |
| `func()` | must return 1 or 2 values |
| `func() string` without default | `func() T` requires a default value |

## Reload Behavior

`Container.Reload(bean)` re-resolves static `value:"..."` fields but does not affect dynamic function fields. Dynamic properties already read live values on each call, so they are naturally up to date.

## When to Use Dynamic vs Static

| Use Case | Recommendation |
|---|---|
| Config read once at startup | Static `value:"..."` |
| Feature flags, rate limits | Dynamic `func() T` |
| Secrets that may rotate | Dynamic `func() (T, error)` |
| Values that must never fail | Static with `default=...` |
