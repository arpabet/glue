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
