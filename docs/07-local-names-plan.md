# Plan: Local Name Index (`localNames`)

## Problem Statement

There is an inconsistency between what the container knows and what `Lookup(name)` can find.

Consider beans `a`, `b`, `c` where `c` depends on `a` and `b`, but nobody depends on `c`:

```go
ctn, _ := glue.New(&a{}, &b{}, &c{})

ctn.Lookup("pkg.a", 0)  // found — because c injects a, so a was registered in registry
ctn.Lookup("pkg.b", 0)  // found — same reason
ctn.Lookup("pkg.c", 0)  // NOT FOUND — nobody injects c, so it was never added to registry
```

All three beans are in `core` (the type catalog), they are all initialized (PostConstruct ran), they are all part of `disposables` (Destroy will be called). But `c` is invisible to name-based lookup because `registry.beansByName` is only populated as a side effect of injection resolution.

## Current Architecture

Three data structures hold bean references:

| Structure | Populated when | Used by |
|-----------|---------------|---------|
| `core map[reflect.Type][]*bean` | During scan (every bean) | `Bean(type)` fallback, injection resolution |
| `registry.beansByType` | When a type/interface is first resolved for injection or `Bean()` call | `Bean(type)` cached lookup |
| `registry.beansByName` | Side effect of `addBeanList` during injection resolution | `Lookup(name)` |

The problem: `registry.beansByName` is a derived cache, not an authoritative catalog. Beans that are never injected anywhere are never added to it.

## Root Cause

The `registry` was designed as a **runtime cache** for derived lookups (interface matching, cross-container search). It was never meant to be the authoritative source of all beans. But `Lookup(name)` treats it as if it were.

## Proposed Solution

### 1. Add `localNames map[string][]*bean` to container

```go
type container struct {
    parent     *container
    children   []ChildContainer
    core       map[reflect.Type][]*bean   // local type catalog (unchanged)
    localNames map[string][]*bean         // NEW: local name catalog
    disposables []*bean
    registry   registry                   // cache for derived lookups only
    properties Properties
    closeOnce  sync.Once
}
```

### 2. Populate `localNames` during scan

Every bean that enters `core` also enters `localNames` by its name. This happens in two places:

**a) Regular pointer beans** — in the `case reflect.Ptr` branch of `forEach` callback, after `investigate()`:

```go
registerBean(core, classPtr, objBean)
ctn.localNames[objBean.name] = append(ctn.localNames[objBean.name], objBean)
```

**b) Factory-produced placeholder beans** — the `elemBean` created for `FactoryBean`:

```go
registerBean(core, elemClassPtr, elemBean)
ctn.localNames[elemBean.name] = append(ctn.localNames[elemBean.name], elemBean)
```

**c) Function beans** — in the `case reflect.Func` branch:

```go
registerBean(core, classPtr, objBean)
ctn.localNames[objBean.name] = append(ctn.localNames[objBean.name], objBean)
```

**d) Container and Properties synthetic beans** — the `ctnBean` and `propertiesBean` created at the start of `createContainer` should also be registered if we want them discoverable by name.

### 3. Change `Lookup(name)` to consult `localNames` first

```go
func (t *container) searchByNameRecursive(name string) []beanlist {
    var candidates []beanlist
    level := 1
    for ctx := t; ctx != nil; ctx = ctx.parent {
        if list, ok := ctx.localNames[name]; ok && len(list) > 0 {
            candidates = append(candidates, beanlist{level: level, list: list})
        }
        level++
    }
    return candidates
}
```

The old `searchByNameInRepositoryRecursive` searched `registry.beansByName`. The new method searches `localNames` directly. Since `localNames` is populated at scan time and never modified at runtime, it is safe for concurrent reads without locking (same as `core`).

### 4. Keep `registry` as a cache for derived lookups only

`registry.beansByType` still caches interface-to-implementation resolution. `registry.beansByName` is no longer needed for `Lookup` — it can be removed or kept only for backward compatibility of the `addBean`/`addBeanList` path used during factory construction at runtime.

### 5. Factory non-singletons: cataloged as placeholders, not runtime instances

Non-singleton factory beans already create a placeholder `elemBean` in `core`. This placeholder should also go into `localNames`. When the factory produces additional runtime instances (non-singleton `ctor` calls), those runtime instances are added to `registry` for caching but **not** to `localNames`. This keeps `localNames` as a stable catalog and `registry` as the mutable runtime cache.

If a non-singleton factory produces a named bean (via `NamedBean`), the runtime instance gets its name registered in `registry.addBean`. This is fine — `Lookup` first checks `localNames` (which has the placeholder), then could optionally fall through to `registry` for runtime-produced beans. Whether we want this fallthrough is a design choice:

**Option A (simple):** `Lookup` only checks `localNames`. Non-singleton factory runtime instances are not discoverable by name via `Lookup`. Callers who need them should inject the factory or use scoped injection.

**Option B (complete):** `Lookup` checks `localNames` first, then falls through to `registry.beansByName`. This preserves current behavior for factory-produced beans that acquire names at runtime.

**Recommendation:** Option A. It is simpler and aligns with the principle that `localNames` is the authoritative catalog. Non-singleton beans are transient by nature and should be accessed through their provider mechanism, not through container lookup.

### 6. Interface resolution: unchanged

Interface resolution remains type-driven through `core` + `registry.beansByType`. No changes needed. `Bean(type)` continues to work as before.

## Summary of Changes

| File | Change |
|------|--------|
| `container.go` | Add `localNames map[string][]*bean` field to `container` struct |
| `container.go` | Initialize `localNames` in `createContainer` |
| `container.go` | Register every bean into `localNames` during scan (3 sites: pointer, factory placeholder, function) |
| `container.go` | New `searchByNameRecursive` method that reads `localNames` |
| `container.go` | `Lookup` calls `searchByNameRecursive` instead of `searchByNameInRepositoryRecursive` |
| `registry.go` | Optional: remove `beansByName` or mark as internal cache only |

## Impact

* `Lookup(name)` will find all beans, even those that nobody injects
* No performance regression — `localNames` is a simple map populated once at scan time
* `Bean(type)` is unaffected
* Scoped injection is unaffected
* Parent-child hierarchy works naturally — each container has its own `localNames`, searched level by level
* Thread safety: `localNames` is written during `createContainer` (single-threaded) and read-only after that

## Migration

This is a backward-compatible change. Beans that were previously findable via `Lookup` remain findable. Beans that were previously invisible become visible. No API changes.
