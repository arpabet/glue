# Selection, Profiles, and Conditions

## Named Beans

Implement `glue.NamedBean` to override the default bean name and use that name for qualifier-based lookup and map injection.

```go
func (t *component) BeanName() string {
    return "new_component"
}
```

## Primary Beans

When multiple candidates implement the same interface, `glue.PrimaryBean` allows one implementation to be selected by default.

```go
func (t *primaryService) IsPrimaryBean() bool {
    return true
}
```

If there are multiple candidates and none is primary, injection fails.

## Profiles

Glue supports profile-based bean registration during scan.

Profile expression examples:
* `dev`
* `!prod`
* `dev|staging`
* `dev&local`

Activation:
* `glue.NewWithProfiles(...)`
* `glue.NewWithOptions(... glue.WithProfiles(...))`
* property lookup through `glue.profiles.active`

Bean-level example:

```go
type devStorage struct{}

func (t *devStorage) BeanProfile() string {
    return "dev"
}
```

Scanner-level example:

```go
ctn, err := glue.NewWithProfiles([]string{"prod"},
    glue.IfProfile("prod",
        &prodStorage{},
        &prodMetrics{},
    ),
)
```

Important behavior:
* profile filtering happens during scan
* if profiles come from properties, they must already be available through the `Properties` object or its resolvers
* if a scanner implements `ProfileBean`, the whole scanner is skipped
* beans returned by `ScannerBeans()` may also implement `ProfileBean`

## Conditional Beans

Implement `glue.ConditionalBean` when registration depends on runtime checks.

```go
func (t *redisCache) ShouldRegisterBean() bool {
    conn, err := net.DialTimeout("tcp", t.addr, time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}
```

Ordering:
* `ProfileBean` is checked first
* `ConditionalBean` is checked second

`ShouldRegisterBean()` runs before injection, so injected fields are still nil at that point.
