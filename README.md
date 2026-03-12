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

## Highlights

Core features:
* pointer and interface injection; functions are not registered as beans (scoped providers still use function-typed fields)
* qualifier, primary, optional, and lazy injection
* collection injection for slices and maps
* `PostConstruct` / `Destroy` lifecycle hooks with and without `context.Context`
* `FactoryBean` and `ContextFactoryBean` with factory-owned product naming and lifecycle
* `singleton`, `prototype`, and `request` scopes
* profiles and conditional bean registration
* parent-child containers and lazy child containers
* property sources, property resolvers, and resource sources

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

## Build

```bash
make build
make bench
```

See [Build and Benchmark](docs/07-build-and-benchmark.md) for targets, benchmark coverage, complexity notes, and optimization guidance.

## Contributions

If you find a bug or issue, please create a ticket.
For now no external contributions are allowed.
