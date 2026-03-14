# glue

![build workflow](https://go.arpabet.com/glue/actions/workflows/build.yaml/badge.svg)

Glue is a runtime dependency injection container for Go inspired by Spring Framework.

It is designed for applications that need:
* runtime wiring of complex dependency graphs
* lifecycle hooks for startup and shutdown
* hierarchical containers
* property and resource loading
* profile- and condition-based bean registration
* scoped providers such as `prototype` and `request`

Glue uses reflection at container startup. Direct pointer wiring is relatively cheap, while interface-heavy graphs cost more because the container must match candidates dynamically. At a high level, startup behaves roughly like `O(n*m)` where `n` is the number of interface requirements and `m` is the number of candidate services.

## Competitive Position

| Feature | Glue | Wire | Dig/Fx | samber/do |
|---------|------|------|--------|-----------|
| Runtime DI | Yes | No (codegen) | Yes | Yes |
| Struct tag injection (+ bare tag) | Yes | No | No | No |
| Generics API | Yes | N/A | No | Yes |
| Property injection (static + dynamic func) | Yes | No | No | No |
| Property prefix map (`value:"prefix=X"`) | Yes | No | No | No |
| Property expressions (`${key:default}`) | Yes | No | No | No |
| Env var resolver (built-in + enumerable) | Yes | No | No | No |
| Dynamic config via lazy properties | Yes | No | No | No |
| Profiles with expressions | Yes | No | No | No |
| Conditions | Yes | No | No | No |
| Bean scopes (singleton/prototype/request) | Yes | No | Scopes | Transient/Lazy |
| Decorators with ordering | Yes | No | Yes (Decorate) | No |
| Bean post-processors | Yes | No | No | No |
| Factory beans (+ context-aware) | Yes | Providers | Constructors | Providers |
| Lifecycle hooks (+ context) | Yes | Cleanup | OnStart/OnStop | Shutdown |
| Context hierarchy with levels | Yes | No | Scopes | Scopes |
| Collection injection (slice + map + ordered) | Yes | No | Groups | No |
| Graph visualization (DOT format) | Yes | Yes | Yes | Explain |
| Component scanning | gluegen | Wire gen | No | No |
| Compile-time validation | Via gluegen | Native | ValidateApp | No |
| Gluegen decorator proxy gen | Yes | No | No | No |
| Error handler callback | Yes | N/A | No | No |
| Bean reload | Yes | No | No | Hot swap |
| Lazy injection | Yes | N/A | No | Yes |
| Primary bean resolution | Yes | No | No | No |
| Struct auto-wrapping | Yes | N/A | No | No |

**Result:** Glue is the only Go DI framework that combines runtime reflection DI,
type-safe generics API, built-in property management with dynamic refresh, profiles,
multiple scopes, and Spring-like developer experience — while also offering a code
generation tool for compile-time validation.

## Quick Start

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
* `glue.NewWithProfiles([]string{"dev"}, ...)`
* `glue.NewWithProperties(ctx, props, ...)`
* `glue.NewWithOptions([]glue.ContainerOption{...}, ...)`

Common options:
* `glue.WithContext(ctx)`
* `glue.WithProperties(props)`
* `glue.WithProfiles("dev", "local")`
* `glue.WithLogger(logger)` — per-container diagnostic logging via `ContainerLogger` interface

## Documentation

Detailed documentation is organized under `docs/`:
* [Documentation Index](docs/README.md)
* [Getting Started](docs/01-getting-started.md)
* [Lifecycle and Context](docs/02-lifecycle-and-context.md)
* [Selection, Profiles, and Conditions](docs/03-selection-profiles-conditions.md)
* [Factories and Scopes](docs/04-factories-and-scopes.md)
* [Hierarchy and Search](docs/05-hierarchy-and-search.md)
* [Properties and Resources](docs/06-properties-and-resources.md)
* [Build and Benchmark](docs/07-build-and-benchmark.md)
* [Gluegen](docs/08-gluegen.md)
* [Dependency Graph](docs/09-dependency-graph.md)
* [Decorators](docs/10-decorators.md)
* [Dynamic Properties](docs/11-dynamic-properties.md)

## Highlights

Core features:
* pointer and interface injection with automatic struct-to-pointer wrapping; functions are not registered as beans (scoped providers still use function-typed fields)
* qualifier, primary, optional, and lazy injection
* collection injection for slices and maps
* `PostConstruct` / `Destroy` lifecycle hooks with and without `context.Context`
* `FactoryBean` and `ContextFactoryBean` with factory-owned product naming and lifecycle
* `singleton`, `prototype`, and `request` scopes
* profiles and conditional bean registration
* parent-child containers and lazy child containers
* property sources, property resolvers, `EnumerablePropertyResolver` for key-discoverable resolvers, and resource sources
* prefix map injection (`value:"prefix=X"`) for grouped configuration into `map[string]string`
* `${key}` / `${key:default}` property expressions with raw and resolved access paths
* built-in `EnvPropertyResolver` for environment variable config (twelve-factor app)
* per-container `ContainerLogger` with `WithLogger` option and parent inheritance
* decorator support with ordered application and automatic field updates
* bean post-processors for cross-cutting concerns (handler registration, validation, metrics)
* dynamic properties via `func() T`, `func() (T, error)`, and `func(context.Context) (T, error)` for live config
* DOT-format dependency graph export via `Container.Graph()`

Search constants:

```go
const (
    DefaultSearchLevel         = SearchFallback
    SearchFallback             = 0
    SearchCurrent              = 1
    SearchCurrentAndParent     = 2
    SearchCurrentAndTwoParents = 3
    SearchCurrentAndAllParents = -1
)
```

## Examples

The `examples/` directory contains runnable sample applications:

* **[webapp](examples/webapp/)** — HTTP server demonstrating the full feature set: profiles, request scope, decorators, prefix maps, dynamic properties, BeanPostProcessor for handler auto-registration, `Container.Graph()`, and graceful shutdown.
* **[secrets](examples/secrets/)** — Dynamic secret-driven DB client with a custom `EnumerablePropertyResolver` backed by a mock secret store, live secret rotation via `func() (T, error)`, and `Container.Reload` for prefix map refresh.
* **[profiles](examples/profiles/)** — Multi-profile bootstrap with `IfProfile`, profile expressions (`dev|staging`, `!prod`), `ConditionalBean`, parent-child container hierarchy, and collection injection.

## Build

```bash
make build
make bench
```

See [Build and Benchmark](docs/07-build-and-benchmark.md) for targets, benchmark coverage, complexity notes, and optimization guidance.

## Contributions

If you find a bug or issue, please create a ticket.
For now no external contributions are allowed.
