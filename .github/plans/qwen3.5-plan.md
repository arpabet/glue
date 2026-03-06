# Glue Framework - Improvement Plan

## Context

The Glue framework is a Spring-inspired runtime dependency injection framework for Go. While feature-rich, it has several areas that need improvement to be competitive with modern DI frameworks like Google Wire, Guice, and Spring Framework.

**Key Challenges**:
1. Performance: O(n*m) complexity for interface lookups with no caching
2. Type Safety: Heavy use of `interface{}` leads to runtime errors
3. API Ergonomics: Confusing naming and verbose injection tags
4. Missing Features: No constructor injection, profiles, or bean scopes

---

## Implementation Plan

### Phase 1: API Cleanup (Non-Breaking Renames)

#### 1.1 Internal Field Renames (No API impact)
**Files**: `bean.go`, `injection.go`

| Current | New | Scope |
|---------|-----|-------|
| `beanDef.classPtr` | `beanDef.type` | Internal |
| `bean.beenFactory` | `bean.factory` | Internal |
| `beanDef.anonymousFields` | `beanDef.embeddedTypes` | Internal |
| `injectionDef.table` | `injectionDef.isMap` | Internal |
| `injectionDef.slice` | `injectionDef.isSlice` | Internal |
| `propInjectionDef.layout` | `propInjectionDef.timeFormat` | Internal |

**Approach**: Direct rename of internal fields. No user-facing changes.

#### 1.2 Method Additions (Deprecation path)
**File**: `api.go`

Add new type-safe methods alongside existing ones:
```go
// New type-safe API
Context beans by type
class BeansByType[T any](level int) []T

// Keep old method with deprecation comment
// Deprecated: Use BeansByType[T] instead
Lookup(name string, level int) []Bean
```

---

### Phase 2: Performance Optimizations

#### 2.1 Add Type Caching
**File**: `context.go`

Add caching to context struct:
```go
type context struct {
    parent          *context
    core            map[reflect.Type][]*bean
    registry        registry
    properties      Properties

    // NEW: Type caching for O(1) lookups
    typeCache       sync.Map // map[reflect.Type][]*bean
    interfaceCache  sync.Map // map[reflect.Type][]*bean
    runtimeCache    sync.Map // existing cache for Inject()
}
```

**Implementation**:
- Modify `searchInterfaceCandidatesRecursive` to cache results
- Modify `searchAndCacheObjectRecursive` to use cache first
- Clear cache on `Extend()` to create child-specific caches

**Files to modify**: `context.go` (lines 606-710)

#### 2.2 Optimize Property Resolution
**File**: `properties.go`

Add LRU cache for property resolution:
```go
type properties struct {
    sync.RWMutex
    priority int
    store    map[string]string
    comments map[string][]string
    resolvers []PropertyResolver

    // NEW: Cache for resolved property values
    cache sync.Map // map[string]string
}
```

---

### Phase 3: New Features

#### 3.1 Constructor Injection
**File**: `api.go`, `context.go`

Add a new interface for constructor injection:
```go
type Constructable interface {
    // Mark struct as using constructor injection
}

// Or use a specific injection tag on constructor
func NewService(dep Dependency) *Service `inject:"constructor"`
```

**Alternative approach (simpler)**: Use primary constructor pattern
```go
type Service struct {
    ID   int
    name string
}

// Constructor function that returns the bean
func NewService(dep Dependency, id int) (*Service, error) {
    return &Service{ID: id, name: dep.Name()}, nil
}
```

**File**: `api.go` - Add `ConstructorBean` interface

#### 3.2 Profile/Conditional Beans
**File**: `api.go`, `context.go`

Add profile support similar to Spring:
```go
// Add to context creation
NewWithContext(options ContextOptions, beans ...interface{}) (Context, error)

// ContextOptions includes:
type ContextOptions struct {
    ActiveProfiles []string
    PropertySources []PropertySource

    // Conditional options
    ConditionResolver func(condition string) bool
}

// Bean-level profile control
type ProfiledBean interface {
    BeanName() string
    Profiles() []string  // "dev", "test", "prod"
}

// Or use tags
// inject:"profile=dev"
```

#### 3.3 Bean Scopes
**File**: `api.go`, `context.go`

Support different bean lifecycles:
```go
// Scope enum
type BeanScope int

const (
    ScopeSingleton BeanScope = iota  // default
    ScopePrototype                    // new instance each injection
    ScopeRequest                      // one per request (runtime inject)
    ScopeSession                      // one per session
)

// Bean-level scope
type ScopedBean interface {
    BeanName() string
    BeanScope() BeanScope
}

// Or via tag
// inject:"scope=prototype"
```

#### 3.4 Primary Bean Selection
**File**: `context.go`

Add `@Primary` equivalent to disambiguate multiple implementations:
```go
type PrimaryBean interface {
    IsPrimary() bool
}

// In injection, prefer primary bean when multiple candidates exist
func selectPrimary(candidates []*bean) *bean {
    // Find primary, fall back to first ordered
}
```

---

### Phase 4: API Improvements

#### 4.1 Simplify Injection Tags
**File**: `context.go`, `injection.go`

Current verbose tags:
```go
// Current
type MyService struct {
    UserService UserService `inject:"bean=com.example.UserServiceImpl"`
    Config      Config      `inject:"optional,level=2"`
}

// Improved: Use bean name directly
type MyService struct {
    UserService UserService `inject:"userServiceImpl"`
    Config      Config      `inject:"optional"`
}
```

**Implementation**:
- Make qualifier be just the bean name (not package.class)
- Remove level from tag (use context-wide defaults)
- Keep "optional", "lazy" as flag-only tags

#### 4.2 Builder Pattern for Context Creation
**File**: `api.go`, `context.go`

Add fluent builder:
```go
ctx, err := glue.NewBuilder().
    ActiveProfiles("dev").
    PropertySource(file: "config.properties").
    Bean(logger).
    Bean(storageImpl{}).
    Child("web", func(c glue.ChildBuilder) {
        c.Bean(webHandlers...)
    }).
    Build()
```

**Implementation**:
```go
type ContextBuilder interface {
    ActiveProfiles(profiles ...string) ContextBuilder
    PropertySource(src PropertySource) ContextBuilder
    PropertyResolver(r PropertyResolver) ContextBuilder
    Bean(beans ...interface{}) ContextBuilder
    Child(name string, fn func(ChildBuilder)) ContextBuilder
    Build() (Context, error)
}
```

#### 4.3 Type-Safe FactoryBean
**File**: `api.go`

Add generics-based FactoryBean:
```go
type FactoryBean[T any] interface {
    Produce() (T, error)
    ObjectType() reflect.Type  // For runtime reflection
    ObjectName() string
    Singleton() bool
}

// Usage:
type MyFactory struct {
    Dep Dependency `inject:""`
}

func (f *MyFactory) Produce() (*MyBean, error) {
    return &MyBean{dep: f.Dep}, nil
}
```

Keep old `FactoryBean` interface for backward compatibility with deprecation.

---

### Phase 5: Component Scanning

#### 5.1 Package Scanning
**File**: New package `scan/`

Add component scanning to replace manual bean listing:
```go
// Scan packages for beans
count, err := glue.Scan(ctx,
    glue.Package("myapp.services"),
    glue.WithAnnotations(glue.Injectable{}),
)

// Or scan into context directly
ctx, err := glue.NewFromPackages(
    "myapp.services",
    "myapp.handlers",
)
```

**Implementation**:
- Use `go/packages` or `golang.org/x/tools/go/packages`
- Look for types with `//go:generate glue-bean` comments
- Or types implementing marker interfaces

---

### Phase 6: Testing & Documentation

#### 6.1 Integration Tests
Add comprehensive integration tests covering:
- Performance benchmarks for type lookups
- Memory usage with large bean graphs
- Concurrent context creation
- Profile-specific bean loading

**File**: `bench_test.go` (new)

#### 6.2 Example Applications
Create examples directory:
- `examples/basic/` - Simple DI usage
- `examples/web/` - HTTP server with DI
- `examples/profiles/` - Multi-environment config
- `examples/factory/` - FactoryBean patterns

---

## Priority Order

### High Priority (Core Improvements)
1. **Performance: Add type caching** - Reduces O(n*m) to O(1) for lookups
2. **Type-safe FactoryBean** - Eliminates `interface{}` return type
3. **Simplified injection tags** - Better API ergonomics
4. **Primary bean selection** - Disambiguate multiple implementations

### Medium Priority (Quality of Life)
5. **Profile support** - Environment-specific configurations
6. **Builder pattern** - More intuitive context creation
7. **Constructor injection** - Immutable dependencies

### Low Priority (Advanced Features)
8. **Component scanning** - Reduce manual bean registration
9. **Bean scopes** - Prototype, request, session
10. **AOP/interceptors** - Cross-cutting concerns

---

## Files to Modify

| File | Changes | Priority |
|------|---------|----------|
| `api.go` | Add new interfaces, deprecate old ones | High |
| `context.go` | Caching, new features, builder | High |
| `bean.go` | Internal field renames | High |
| `injection.go` | Internal field renames, tag parsing | High |
| `properties.go` | Property cache | Medium |
| `registry.go` | Secondary indexes | Medium |
| `stub.go` | Update stub implementations | Medium |

## Files to Create

| File | Purpose |
|------|----------|
| `builder.go` | Context builder implementation |
| `scan/` | Component scanning package |
| `bench_test.go` | Performance benchmarks |
| `examples/` | Example applications |

---

## Backward Compatibility Strategy

1. **Deprecation phase** (v1.x):
   - Add new APIs with deprecation comments
   - Old APIs continue to work
   - Document migration path

2. **Transition phase** (v2.0):
   - New default behavior (simplified tags)
   - Old tags still supported with warning
   - Type-safe generics APIs preferred

3. **Breaking changes** (v3.0):
   - Remove deprecated APIs
   - Internal field renames go public if needed

---

## Success Metrics

1. **Performance**: 10x faster interface lookup
2. **Type safety**: Zero `interface{}` in new APIs
3. **Ergonomics**: 50% less boilerplate code
4. **Features**: Parity with Spring's core DI features
