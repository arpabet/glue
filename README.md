# glue

Dependency Injection Runtime Framework for Golang inspired by Spring Framework in Java.

All injections happen on runtime and took O(n*m) complexity, where n - number of interfaces, m - number of services.
In golang we have to check each interface with each instance to know if they are compatible. 
All injectable fields must have tag `inject` and be public.

### Usage

Dependency Injection framework for complex applications written in Golang.
There is no capability to scan components in packages provided by Golang language itself, therefore the context creation needs to see all beans as memory allocated instances by pointers.
The best practices are to inject beans by interfaces between each other, but create context of their implementations.

Example:
```
var ctx, err = glue.New(
    logger,
    &storageImpl{},
    &configServiceImpl{},
    &userServiceImpl{},
    &struct {
        UserService UserService `inject`  // injection based by interface or pointer 
    }{}, 
)
require.Nil(t, err)
defer ctx.Close()
```

Glue Framework does not support anonymous injection fields.

Wrong:
```
type wrong struct {
    UserService `inject`  // since the *wrong structure also implements UserService interface it can lead to cycle and wrong injections in context
}
```

Right:
```
type right struct {
    UserService UserService `inject`  // guarantees less conflicts with method names and dependencies
}
```

### Types

Glue Framework supports following types for beans:
* Pointer
* Interface
* Function

Glue Framework does not support Struct type as the bean instance type. 

### Function

Function in golang is the first type citizen, therefore Bean Framework supports injection of functions by default.

Example:
```
type holder struct {
	StringArray   func() []string `inject`
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
All collection injections would be treated as collection of glue. 
If you need to inject collection of primitive types, please use function injection.

Example:
```
type holderImpl struct {
	Array   []Element          `inject`
	Map     map[string]Element `inject`
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

For each bean that implements InitializingBean interface, Glue Framework invokes PostConstruct() method at the time of Construction of the bean.
This functionality could be used for safe initialization.

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject`
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
    return t.Dependency.DoSomething()
}
``` 

### glue.DisposableBean

For each bean that implements DisposableBean interface, Glue Framework invokes Destroy() method at the time of closing context, in reverse order how beans were initialized.

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject`
}

func (t *component) Destroy() error {
    // guarantees that dependency still not destroyed by calling in backwards initialization order
    return t.Dependency.DoSomething()
}
```

### glue.NamedBean

For each bean that implements NamedBean interface, Glue Framework will use returned bean name of calling function BeanName() instead of class name of the bean.
Together with qualifier gives ability to select that bean to inject in application context. 

Example:
```
type component struct {
}

func (t *component) BeanName() string {
    // overrides default bean name: package_name.component
    return "new_component"
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
    Dependency  *anotherComponent  `inject`
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

### Lazy fields

Added support for lazy fields, that defined like this: `inject:"lazy"`.

Example:
```
type component struct {
    Dependency  *anotherComponent  `inject:"lazy"`
    Initialized bool
}

type anotherComponent struct {
    Dependency  *component  `inject`
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

Suppose we do not have anotherComponent in context, but would like our context to be created anyway, that is good for libraries.
In this case there is a high risk of having null-pointer panic during runtime, therefore for optional dependency
fields you need always check if it is not nil before use.

```
if t.Dependency != nil {
    t.Dependency.DoSomething()
}
```

### Extend

Glue Framework has method Extend to create inherited contexts whereas parent sees only own beans, extended context sees parent and own glue.
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

If we destroy child context, parent context still be alive.

Example:
```
child.Close()
// Extend method does not transfer ownership of beans from parent to child context, you would need to close parent context separatelly, after child
parent.Close()
```

### Level

After extending context, we can end up with hierarchy of contexts, therefore we need levels in API to understand how deep we need to retrieve beans from parent contexts.

Lookup level defines how deep we will go in to beans:
* level 0: look in the current context, if not found then look in the parent context and so on (default)
* level 1: look only in the current context
* level 2: look in the current context in union with the parent context
* level 3: look in union of current, parent, parent of parent contexts
* and so on.
* level -1: look in union of all contexts.

### Contributions

If you find a bug or issue, please create a ticket.
For now no external contributions are permitted.



