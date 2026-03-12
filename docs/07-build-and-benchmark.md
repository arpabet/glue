# Build and Benchmark

## Build

Glue ships with a simple `Makefile`.

Targets:
* `make build`: runs tests with coverage, then builds the module
* `make bench`: runs the benchmark suite
* `make version`: prints the git-derived version string
* `make update`: updates module dependencies

Equivalent commands:

```bash
go test -cover ./...
go build -v
go test -bench=Benchmark -benchmem -count=1 -run=^$
```

## Complexity

Glue is a runtime DI container, so startup cost is dominated by reflection and matching.

Important characteristics:
* direct pointer lookups are relatively cheap
* interface-based resolution is more expensive because candidate implementations must be matched dynamically
* startup complexity grows with bean count and especially with interface-heavy graphs
* README-level rule of thumb: startup behaves roughly like `O(n*m)` where `n` is the number of interface requirements and `m` is the number of candidate services

This cost is paid mostly during container construction, not on every use.

## Existing Optimizations

The current implementation already includes several important optimizations:
* lookup registry caches results by type and name
* negative lookup results are cached too
* runtime injection metadata is cached in `runtimeCache`
* request and prototype scoped providers generate functions once, then reuse them
* name lookup is much cheaper than repeated interface scans after registration

## Benchmark Coverage

The benchmark suite in [benchmark_test.go](../benchmark_test.go) measures:
* startup with pointer-heavy graphs
* startup with interface-heavy graphs
* lookup by concrete type
* lookup by interface
* lookup by bean name

Run:

```bash
make bench
```

## How To Read the Results

Expect these patterns:
* pointer startup is the cheapest startup path
* interface startup is slower because of compatibility matching
* name lookup is typically the cheapest steady-state lookup path
* type and interface lookup cost grows with registered bean count

## Practical Optimization Advice

For large applications:
* prefer interface injection between major components, but keep the number of implementations per interface under control
* use qualifiers or primary beans when an interface has multiple candidates
* prefer name-based lookup only where dynamic lookup is really needed
* keep startup scans explicit and deterministic
* move expensive conditional registration logic behind profiles or resolvers when possible
* use request/prototype scope only where needed, because scoped providers deliberately create work on demand

## Future Optimization Areas

Likely future improvements include:
* precompiled injection plans
* more aggressive interface candidate indexing
* property conversion caching
* dynamic-config-aware property caches with invalidation
