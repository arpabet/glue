# Glue Framework Enhancement Plan

## Context
The Glue project provides a runtime dependency‑injection container for Go. The user wants to extend it with dynamic configuration, better performance, stronger type safety, profile support, bean scopes, simplified syntax, component scanning, and richer tests/sample apps.

## Goals
1. **Dynamic property injection** – obtain secrets/config at runtime via a generic `PropertyProvider`.
2. **Caching of injection resolution** – eliminate the O(n*m) lookup cost while keeping dynamic updates possible.
3. **Type‑safe API** – expose generic `Get[T]` / `Inject[T]` helpers, deprecate the raw `Inject(interface{})`.
4. **Profiles** – activate/deactivate beans based on environment‑specific profiles (like Spring).
5. **Bean scopes** – support singleton, prototype, and request‑scoped beans.
6. **Simplified injection syntax** – clearer tags, explicit qualifier support.
7. **Component scanning** – auto‑register beans by package.
8. **Improved property handling** – templating, caching with invalidation on change.
9. **Testing & examples** – integration tests and a small example application.

## High‑Level Implementation Steps
| Phase | Description | Key Files Modified | New Files |
|------|-------------|-------------------|-----------|
| 1 | Define `PropertyProvider` interface and built‑in providers (env, file). Update `propInjectionDef` to resolve via provider, add caching/invalidation. | `properties.go`, `injection.go` | `aws_provider.go`, `gcp_provider.go` |
| 2 | Add per‑context injection cache (`injectionCache`) and reuse it in `searchAndCacheObjectRecursive`. | `context.go`, `registry.go` | – |
| 3 | Introduce generic API: `func (c *context) Get[T any](qualifier string) (T, error)` and `Inject[T any]`. Keep old `Inject` for compatibility (deprecated). | `context.go` | – |
| 4 | Implement profile support: `type Profile string`, `activeProfiles` set, `profile` tag parsing, conditional bean registration. | `context.go`, `bean.go` | – |
| 5 | Add bean scope enum (`Singleton`, `Prototype`, `Request`) and `scope` tag parsing. Implement prototype factories and request‑context helper (`NewRequestContext`). | `bean.go`, `injection.go`, `context.go` | – |
| 6 | Simplify tag syntax: introduce separate `qualifier` tag, update `investigate` to read it, keep backward‑compatible parsing of old `inject` tag. | `injection.go` | – |
| 7 | Component scanning: create `PackageScanner` implementing `Scanner` that walks Go AST to discover structs marked `// @injectable` or implementing a marker interface. | `scanner.go` (new) | – |
| 8 | Extend property templating: parse `${key}` placeholders, resolve via provider chain, cache result, register watch callbacks to invalidate on change. | `properties.go`, `propInjectionDef` in `injection.go` | – |
| 9 | Add unit & integration tests for each new feature; add `example/` module demonstrating profiles, prototype beans, and secret injection. | `*_test.go` (new), `example/main.go` (new) | – |

## Detailed Changes per File
- **context.go**: add `activeProfiles map[Profile]struct{}`, `BeanScope` type, generic helper methods, request‑context constructor, profile activation via `glue.profiles` property, modify `createContext` to filter beans by profile and scope.
- **bean.go**: add fields `profiles []Profile`, `scope BeanScope`; extend `investigate` to parse `profile` and `scope` tags; adjust `registerBean` to respect profiles.
- **injection.go**: modify `propInjectionDef` to hold a `PropertyProvider`; add caching map `propCache sync.Map`; implement placeholder parsing and watch registration; add new tag parsing for `qualifier` and `scope`.
- **properties.go**: add watch registration (`Watch(key string, fn func(string))`), template resolution helper, and cache eviction logic.
- **registry.go**: extend registration methods to store beans per profile and scope; provide lookup helpers that respect scope.
- **scanner.go** (new): implement `PackageScanner` using `go/parser` to collect injectable structs.
- **aws_provider.go**, **gcp_provider.go** (new): simple implementations of `PropertyProvider` that fetch secrets via AWS SDK / GCP Secret Manager (stubbed for now, can be swapped with real SDKs).
- **tests**: new test files covering dynamic property changes, prototype bean recreation, profile activation, and component scanning.
- **example/**: minimal program showing how to enable a profile, request a prototype bean, and retrieve a secret via a custom provider.

## Risks & Mitigations
- **Breaking API** – keep old `Inject` as deprecated; provide migration guide.
- **Performance of scanning** – only run once at context creation; allow disabling via a flag.
- **Secret provider latency** – cache values and expose timeout/fallback; watch‑based invalidation keeps cache fresh.
- **Prototype memory leaks** – request‑context will clean up after each request.

## Verification
1. Run `go test ./...` – all existing tests must pass; new tests should cover each added feature.
2. Build the example (`go run ./example`) and verify:
   - Profile‑specific beans are present/absent.
   - Changing an environment variable updates the injected value without restarting.
   - Prototype beans return a new instance on each injection.
3. Run a benchmark comparing bean lookup before and after caching to confirm O(1) lookup after warm‑up.

---
*Prepared for user review. Once approved, implementation can proceed.*
