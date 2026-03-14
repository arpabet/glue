# Properties and Resources

## Property Injection

Glue supports `value:"..."` tags for property injection.

Example:

```go
type config struct {
    Host string        `value:"app.host"`
    Port int           `value:"app.port,default=8080"`
    TTL  time.Duration `value:"app.ttl,default=5s"`
}
```

Supported conversions include:
* `string`
* booleans
* signed and unsigned integers
* floats
* `time.Duration`
* `time.Time`
* `os.FileMode`
* slices of the supported types using `;` as separator

For `time.Time`, use `layout=...`:

```go
type config struct {
    Start time.Time `value:"app.start,layout=2006-01-02"`
}
```

## Property Sources

Glue can load properties from:
* `PropertySource`
* `FilePropertySource`
* `MapPropertySource`
* custom `PropertyResolver`

Supported file formats:
* `.properties`
* `.yaml` / `.yml`
* `.json`

For `.properties`, comment lines are accepted during parsing and ignored. Glue no longer stores or re-emits property comments.

Example:

```go
ctn, err := glue.New(
    glue.MapPropertySource{
        "app.host": "localhost",
        "app.port": 8080,
    },
    &config{},
)
```

## Property Resolvers

`PropertyResolver` allows custom dynamic lookup.

Typical use cases:
* environment variables
* secret stores
* external configuration systems

Resolvers are ordered by priority. Higher priority resolvers are checked first.
Glue sorts resolvers in descending priority order and returns the first resolver that matches a key.

Built-in priority baseline:
* `Properties` in-memory/file-backed store: `100`
* `EnvPropertyResolver`: `200`

That means environment variables override values loaded from property files or maps by default.

### Built-in: EnvPropertyResolver

`EnvPropertyResolver` resolves properties from OS environment variables. Register it as a bean and it automatically participates in property resolution.

Key mapping: property keys are uppercased with dots and dashes converted to underscores.

| Property key | Env variable |
|---|---|
| `app.db.host` | `APP_DB_HOST` |
| `app.read-timeout` | `APP_READ_TIMEOUT` |

Basic usage:

```go
// Properties are resolved from env vars automatically
ctn, err := glue.New(
    &glue.EnvPropertyResolver{},
    &config{},
)
```

With a prefix to namespace env vars:

```go
// "db.host" -> "MYAPP_DB_HOST"
ctn, err := glue.New(
    &glue.EnvPropertyResolver{Prefix: "MYAPP"},
    &config{},
)
```

With a custom key mapper for advanced mapping:

```go
ctn, err := glue.New(
    &glue.EnvPropertyResolver{
        KeyMapper: func(key string) string {
            return "CFG_" + strings.ToUpper(strings.ReplaceAll(key, ".", "__"))
        },
    },
    &config{},
)
```

Priority:
* higher number = higher precedence
* Glue sorts resolvers from highest priority to lowest priority
* lookup stops at the first resolver that returns a value
* `EnvPropertyResolver` defaults to `200`
* the built-in `Properties` store defaults to `100`

So this order:

```go
props := glue.NewProperties()
props.Set("app.port", "8080")

ctn, err := glue.NewWithOptions([]glue.ContainerOption{
    glue.WithProperties(props),
}, &glue.EnvPropertyResolver{}, &config{})
```

means:
* if `APP_PORT` exists, Glue returns that value first
* otherwise Glue falls back to the property store value `8080`

This follows common external-configuration practice where environment variables sit above file-based config. Spring Boot documents an ordered property-source chain where later sources override earlier ones, and SmallRye Config uses descending ordinals where higher ordinal sources win. The same rule applies in Glue: higher priority overrides lower priority.

Override `ResolverPriority` when you want different precedence:

```go
&glue.EnvPropertyResolver{ResolverPriority: 50} // lower than file properties
```

With `ResolverPriority: 50`, the property file or map value wins first, and the environment is only used as a fallback.

Full example:

```go
type appConfig struct {
    Host string `value:"app.host,default=localhost"`
    Port int    `value:"app.port,default=8080"`
}

// APP_PORT=9090 in the environment
cfg := &appConfig{}
ctn, err := glue.New(&glue.EnvPropertyResolver{}, cfg)
// cfg.Host = "localhost" (default, no env var set)
// cfg.Port = 9090        (from APP_PORT env var)
```

## Property Hierarchy

Child containers inherit parent property resolvers through `Properties.Extend(...)`.
This is especially useful for profile resolution and environment-backed configuration in parent/child container trees.

## Resources

Glue supports named resource sources using the pattern `name:path`.

Example:

```go
glue.ResourceSource{
    Name: "assets",
    AssetNames: []string{"a.txt", "b/c.txt"},
    AssetFiles: myFS,
}
```

Lookup example:

```go
res, ok := ctn.Resource("assets:a.txt")
```

Resources with the same source name are merged unless the same resource path appears twice, in which case container creation fails.
