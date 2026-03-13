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

Priority: `EnvPropertyResolver` defaults to priority 200, which is higher than file-based properties (100). This means environment variables override file-based properties, following the twelve-factor app convention. Override with `ResolverPriority`:

```go
&glue.EnvPropertyResolver{ResolverPriority: 50} // lower than file properties
```

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
