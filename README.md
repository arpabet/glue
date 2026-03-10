# glue

![build workflow](https://go.arpabet.com/glue/actions/workflows/build.yaml/badge.svg)

Dependency Injection Runtime Framework for Golang inspired by Spring Framework in Java.

All injections happen on runtime and took O(n*m) complexity, where n - number of interfaces, m - number of services.
In golang we have to check each interface with each instance to know if they are compatible. 
All injectable fields must have tag `inject` and be public.

### Usage

Dependency Injection framework for complex applications written in Golang.
There is no capability to scan components in packages provided by Golang language itself, therefore the container creation needs to see all beans as memory allocated instances by pointers.
The best practices are to inject beans by interfaces between each other, but create container of their implementations.

Example:
```
var ctx, err = glue.New(
    logger,
    &storageImpl{},
    &configServiceImpl{},
    &userServiceImpl{},
    &struct {
        UserService UserService `inject:""`  // injection based by interface or pointer 
    }{}, 
)
require.Nil(t, err)
defer ctx.Close()
```

Glue Framework does not support anonymous injection fields.

Wrong:
```
type wrong struct {
    UserService `inject:""`  // since the *wrong structure also implements UserService interface it can lead to cycle and wrong injections in container
}
```

Right:
```
type right struct {
    UserService UserService `inject:""`  // guarantees less conflicts with method names and dependencies
}
```

### Types

Glue Framework supports following types for beans:
* Pointer
* Interface
* Function

Glue Framework does not support Struct type as the bean instance type, in order to inject the object please use pointer on it. 

### Function

Function in golang is the first type citizen, therefore Bean Framework supports injection of functions by default. But you can have only unique args list of them.
This funtionality is perfect to inject Lazy implementations.

Example:
```
type holder struct {
	StringArray   func() []string `inject:""`
}

var ctx, err = glue.New (
    &holder{},
    func() []string { return []string {"a", "b"} },
)
require.Nil(t, err)
defer ctx.Close()
``` 
 
### Collections 
 
Glue Framework supports injection of bean collections including Slice and Map.
All collection injections require being a collection of beans. 
If you need to inject collection of primitive types, please use function injection.

Example:
```
type holderImpl struct {
	Array   []Element          `inject:""`
	Map     map[string]Element `inject:""`
}

var ElementClass = reflect.TypeOf((*Element)(nil)).Elem()
type Element interface {
    glue.NamedBean
    glue.OrderedBean
}
```  
 
Element should implement glue.NamedBean interface in order to be injected to map. Bean name would be used as a key of the map. Dublicates are not allowed.

Element also can implement glue.OrderedBean to assign the order for the bean in collection. Sorted collection would be injected. It is allowed to have sorted and unsorted beans in collection, sorted goes first.
 
### glue.InitializingBean

For each bean that implements InitializingBean interface, Glue Framework invokes PostConstruct() method each the time of bean initialization.
Glue framework guarantees that at the time of calling this function all injected fields are not nil and all injected beans are initialized.
This functionality could be used for safe bean initialization logic.

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject:""`
}

func (t *component) PostConstruct() error {
    if t.Dependency == nil {
        // for normal required dependency can not be happened, unless `optional` field declared
        return errors.New("empty dependency")
    }
    if !t.Dependency.Initialized() {
        // for normal required dependency can not be happened, unless `lazy` field declared
        return errors.New("not initialized dependency")
    }
    // for normal required dependency Glue guarantee all fields are not nil and initialized
    return t.Dependency.DoSomething()
}
``` 

### glue.ContextInitializingBean

For each bean that implements ContextInitializingBean interface, Glue Framework invokes `PostConstruct(context.Context)` during initialization.
The context comes from `glue.NewWithContext(...)`, `glue.NewWithOptions(...)` with `glue.WithContext(...)`, or `context.Background()` when no explicit context is provided.

Example:
```
type component struct {
    RequestID string
}

func (t *component) PostConstruct(ctx context.Context) error {
    if requestID, ok := ctx.Value("request_id").(string); ok {
        t.RequestID = requestID
    }
    return nil
}

ctx := context.WithValue(context.Background(), "request_id", "abc-123")
ctn, err := glue.NewWithContext(ctx, &component{})
require.Nil(t, err)
defer ctn.Close()
```

Corner cases:
* If a bean implements `ContextInitializingBean`, the context-aware method is used for initialization.
* `glue.New(...)` still provides `context.Background()`, so `PostConstruct(ctx)` is always called with a non-nil context.
* If `PostConstruct(ctx)` returns an error, container creation fails.

### glue.DisposableBean

For each bean that implements DisposableBean interface, Glue Framework invokes Destroy() method at the time of closing container in reverse order of how beans were initialized.

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject:""`
}

func (t *component) Destroy() error {
    // guarantees that dependency still not destroyed by calling it in backwards initialization order
    return t.Dependency.DoSomething()
}
```

### glue.ContextDisposableBean

For each bean that implements ContextDisposableBean interface, Glue Framework invokes `Destroy(context.Context)` when the container is closed.
The context comes from `CloseWithContext(ctx)`, or `context.Background()` when `Close()` is used.

Example:
```
type component struct {
    AuditID string
}

func (t *component) Destroy(ctx context.Context) error {
    if auditID, ok := ctx.Value("audit_id").(string); ok {
        t.AuditID = auditID
    }
    return nil
}

ctn, err := glue.New(&component{})
require.Nil(t, err)

destroyCtx := context.WithValue(context.Background(), "audit_id", "shutdown-42")
require.Nil(t, ctn.CloseWithContext(destroyCtx))
```

Corner cases:
* `Close()` still destroys beans that implement `ContextDisposableBean`, using `context.Background()`.
* If `Destroy(ctx)` returns an error, container close returns that error.
* Child containers created through `glue.Child(...)` also receive the same close context when parent `CloseWithContext(ctx)` is used.

### glue.NamedBean

For each bean that implements NamedBean interface, Glue Framework will use a returned bean name by calling function BeanName() instead of class name of the bean.
Together with qualifier this gives ability to select that bean particular to inject to the application container. 

Example:
```
type component struct {
}

func (t *component) BeanName() string {
    // overrides default bean name: package_name.component
    return "new_component"
}
```

Having this qualifier would inject correct bean
```
type component struct {
Dependency  *anotherComponent  `inject:"qualifier=new_component"`
}
```

Is similar to legacy `bean`
```
type component struct {
Dependency  *anotherComponent  `inject:"bean=new_component"`
}
```

And shortcut version
```
type component struct {
Dependency  *anotherComponent  `inject:"new_component"`
}
```

### glue.OrderedBean

For each bean that implements OrderedBean interface, Glue Framework invokes method BeanOrder() to determining position of the bean inside collection at the time of injection to another bean or in case of runtime lookup request. 

Example:
```
type component struct {
}

func (t *component) BeanOrder() int {
    // created ordered bean with order 100, would be injected in Slice(Array) in this order. 
    // first comes ordered beans, rest unordered with preserved order of initialization sequence.
    return 100
}
```

### glue.FactoryBean

FactoryBean interface is using to create beans by application with specific dependencies and complex logic.
FactoryBean can produce singleton and non-singleton glue.

Example:
```
var beanConstructedClass = reflect.TypeOf((*beanConstructed)(nil))
type beanConstructed struct {
}

type factory struct {
    Dependency  *anotherComponent  `inject:""`
}

func (t *factory) Object() (interface{}, error) {
    if err := t.Dependency.DoSomething(); err != nil {
        return nil, err
    }
	return &beanConstructed{}, nil
}

func (t *factory) ObjectType() reflect.Type {
	return beanConstructedClass
}

func (t *factory) ObjectName() string {
	return "qualifierBeanName" // could be an empty string, used as a bean name for produced bean, usially singleton
}

func (t *factory) Singleton() bool {
	return true
}
```

### glue.ContextFactoryBean

ContextFactoryBean is similar to `glue.FactoryBean`, but receives the current container construction context in `Object(ctx context.Context)`.
This is useful when produced objects depend on deadlines, cancellation, request-scoped values, trace data, or context-aware secret/config access.

The context comes from:
* `glue.NewWithContext(...)`
* `glue.NewWithOptions(...)` with `glue.WithContext(...)`
* `Container.ExtendWithContext(...)`
* `ChildContainer.ObjectWithContext(...)`
* `context.Background()` when no explicit context is provided

Example:
```
type factory struct {
    Dependency *anotherComponent `inject:""`
}

func (t *factory) Object(ctx context.Context) (interface{}, error) {
    requestID, _ := ctx.Value("request_id").(string)
    if err := t.Dependency.DoSomething(); err != nil {
        return nil, err
    }
    return &beanConstructed{
        RequestID: requestID,
    }, nil
}

func (t *factory) ObjectType() reflect.Type {
    return beanConstructedClass
}

func (t *factory) ObjectName() string {
    return ""
}

func (t *factory) Singleton() bool {
    return true
}
```

Corner cases:
* `ContextFactoryBean` is preferred over legacy `FactoryBean` when a factory implements the context-aware interface.
* Runtime `container.Inject(...)` can still materialize factory products; in that path Glue uses `context.Background()`.
* Factory-produced objects are still treated as produced instances, not full managed beans. They do not automatically receive `value` property injection, `PostConstruct(ctx)`, or `Destroy(ctx)`.

### Lazy fields

Added support for lazy fields, that defined like this: `inject:"lazy"`.

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject:"lazy"`
    Initialized bool
}

type anotherComponent struct {
    Dependency  *component  `inject:""`
    Initialized bool
}

func (t *component) PostConstruct() error {
    // all injected required fields can not be nil
    // but for lazy fields, to avoid cycle dependencies, the dependency field would be not initialized
    println(t.Dependency.Initialized) // output is false
    t.Initialized = true
}

func (t *anotherComponent) PostConstruct() error {
    // all injected required fields can not be nil and non-lazy dependency fields would be initialized
    println(t.Dependency.Initialized) // output is true
    t.Initialized = true
}
```

### Optional fields

Added support for optional fields, that defined like this: `inject:"optional"`.

Example:

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject:"optional"`
}
```

Suppose we do not have anotherComponent in container, but would like our container to be created anyway, that is good for libraries.
In this case there is a high risk of having null-pointer panic during runtime, therefore for optional dependency
fields you need always check if it is not nil before use.

```
if t.Dependency != nil {
    t.Dependency.DoSomething()
}
```

### Profiles

Glue supports profile-based bean registration during container scan.
This is useful when applications have multiple environments or hierarchical containers where each level may enable a different set of beans.

Active profiles can be provided in two ways:
* Explicitly in code by using `glue.NewWithProfiles(...)` or `glue.NewWithOptions(...)`
* By property lookup through `glue.profiles.active`

Profile expressions:
* `"dev"`: active when profile `dev` is active
* `"!prod"`: active when profile `prod` is not active
* `"dev|staging"`: active when either `dev` or `staging` is active
* `"dev&local"`: active when both `dev` and `local` are active

Bean-level example:
```
type devStorage struct {
}

func (t *devStorage) BeanProfile() string {
    return "dev"
}

ctn, err := glue.NewWithProfiles([]string{"dev"},
    &devStorage{},
)
require.Nil(t, err)
defer ctn.Close()
```

Scanner-level example with shortcut wrapper:
```
ctn, err := glue.NewWithProfiles([]string{"prod"},
    glue.IfProfile("prod",
        &prodStorage{},
        &prodMetrics{},
    ),
    glue.IfProfile("dev|local",
        &debugHandler{},
    ),
)
require.Nil(t, err)
defer ctn.Close()
```

Options-based example:
```
ctn, err := glue.NewWithOptions([]glue.ContainerOption{
    glue.WithProfiles("dev", "local"),
}, 
    &devStorage{},
    &debugHandler{},
)
require.Nil(t, err)
defer ctn.Close()
```

Property-based example:
```
props := glue.NewProperties()
props.Set("glue.profiles.active", "dev,local")

ctn, err := glue.NewWithOptions([]glue.ContainerOption{
    glue.WithProperties(props),
},
    &devStorage{},
    &debugHandler{},
)
require.Nil(t, err)
defer ctn.Close()
```

Corner cases:
* Profile filtering happens during scan. If active profiles are provided explicitly with `WithProfiles(...)` or `NewWithProfiles(...)`, they are used immediately.
* If active profiles come from properties, they must already be available through the `Properties` object or through its registered resolvers before the container starts scanning beans.
* `PropertyResolver` based activation works well for dynamic sources such as environment-backed profile lookup.
* Scanned `PropertySource` files are loaded after scan-time profile filtering. Because of that, `glue.profiles.active` from a `PropertySource` does not affect bean inclusion in that same container creation pass.
* In a parent-child hierarchy this is often acceptable: the parent can load property files first, and child containers created later via `Extend(...)` can resolve `glue.profiles.active` from inherited properties or property resolvers.
* If a `Scanner` implements `ProfileBean`, the whole scanner is skipped when the profile does not match.
* Beans returned by `ScannerBeans()` may also implement `ProfileBean`, which allows fine-grained filtering inside a scanner.

### Conditional Beans

Glue supports conditional bean registration for advanced cases where inclusion depends on arbitrary runtime conditions (e.g., "only if Redis is available", "only if a config flag is set").

A bean that implements `ConditionalBean` is only registered when `ShouldRegisterBean()` returns `true`. The method is called during scanning, before injection and `PostConstruct`.

```
type ConditionalBean interface {
    ShouldRegisterBean() bool
}
```

Ordering with `ProfileBean`:
* `ProfileBean` is checked first (cheap string match).
* `ConditionalBean` is checked second (may do I/O or complex logic).
* Both interfaces can be implemented on the same bean. If the profile does not match, `ShouldRegisterBean()` is never called.

Example:
```
type redisCache struct {
    addr string
}

func (t *redisCache) ShouldRegisterBean() bool {
    conn, err := net.DialTimeout("tcp", t.addr, time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}

ctn, err := glue.New(
    &redisCache{addr: "localhost:6379"},
)
require.Nil(t, err)
defer ctn.Close()
```

Combined with profiles:
```
type devRedisCache struct {
    addr string
}

func (t *devRedisCache) BeanProfile() string {
    return "dev"
}

func (t *devRedisCache) ShouldRegisterBean() bool {
    conn, err := net.DialTimeout("tcp", t.addr, time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}

ctn, err := glue.NewWithProfiles([]string{"dev"},
    &devRedisCache{addr: "localhost:6379"},
)
require.Nil(t, err)
defer ctn.Close()
```

Corner cases:
* `ShouldRegisterBean()` is called on the raw object before any field injection. Injected fields are nil at this point.
* If a `Scanner` implements `ConditionalBean`, the whole scanner is skipped when the condition is false.
* Beans returned by `ScannerBeans()` may also implement `ConditionalBean` for fine-grained filtering.

### Extend

Glue Framework has method Extend to create inherited container whereas parent sees only own beans, extended container sees parent and own glue.
The level of lookup determines the logic how deep we search beans in parent hierarchy. 

Example:
```
struct a {
}

parent, err := glue.New(new(a))

struct b {
}

child, err := parent.Extend(new(b))

len(parent.Lookup("package_name.a", 0)) == 1
len(parent.Lookup("package_name.b", 0)) == 0

len(child.Lookup("package_name.a", 0)) == 1
len(child.Lookup("package_name.b", 0)) == 1
```

If we destroy child container, parent container still be alive.

Example:
```
child.Close()
// Extend method does not transfer ownership of beans from parent to child container, you would need to close parent container separatelly, after child
parent.Close()
```

### Level

After extending container, we can end up with hierarchy of containers, therefore we need levels in API to understand how deep we need to retrieve beans from parent containers.

Lookup level defines how deep we will go in to beans:
* level 0: look in the current container, if not found then look in the parent container and so on (default)
* level 1: look only in the current container
* level 2: look in the current container in union with the parent container
* level 3: look in union of current, parent, parent of parent containers
* and so on.
* level -1: look in union of all containers.

### Contributions

If you find a bug or issue, please create a ticket.
For now no external contributions are allowed.
