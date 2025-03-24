/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"io"
	"net/http"
	"os"
	"reflect"
	"time"
)

type BeanLifecycle int32

const (
	BeanAllocated BeanLifecycle = iota
	BeanCreated
	BeanConstructing
	BeanInitialized
	BeanDestroying
	BeanDestroyed
)

func (t BeanLifecycle) String() string {
	switch t {
	case BeanAllocated:
		return "BeanAllocated"
	case BeanCreated:
		return "BeanCreated"
	case BeanConstructing:
		return "BeanConstructing"
	case BeanInitialized:
		return "BeanInitialized"
	case BeanDestroying:
		return "BeanDestroying"
	case BeanDestroyed:
		return "BeanDestroyed"
	default:
		return "BeanUnknown"
	}
}

var BeanClass = reflect.TypeOf((*Bean)(nil)).Elem()

type Bean interface {

	/*
		Returns name of the bean, that could be instance name with package or if instance implements NamedBean interface it would be result of BeanName() call.
	*/
	Name() string

	/*
		Returns real type of the bean
	*/
	Class() reflect.Type

	/*
		Returns true if bean implements interface
	*/
	Implements(ifaceType reflect.Type) bool

	/*
		Returns initialized object of the bean
	*/
	Object() interface{}

	/*
		Returns factory bean of exist only beans created by FactoryBean interface
	*/
	FactoryBean() (Bean, bool)

	/*
		Re-initialize bean by calling Destroy method if bean implements DisposableBean interface
		and then calls PostConstruct method if bean implements InitializingBean interface

		Reload can not be used for beans created by FactoryBean, since the instances are already injected
	*/
	Reload() error

	/*
		Returns current bean lifecycle
	*/
	Lifecycle() BeanLifecycle

	/*
		Returns information about the bean
	*/
	String() string
}

var ContextClass = reflect.TypeOf((*Context)(nil)).Elem()

type Context interface {
	/*
		Gets parent context if exist
	*/
	Parent() (Context, bool)

	/*
		New new context with additional beans based on current one
	*/
	Extend(scan ...interface{}) (Context, error)

	/*
		Returns list of ctx context inside the current context only
	*/
	Children() []ChildContext

	/*
		Destroy all beans that implement interface DisposableBean.
	*/
	Close() error

	/*
		Get list of all registered instances on creation of context with scope 'core'
	*/
	Core() []reflect.Type

	/*
		Gets obj by type, that is a pointer to the structure or interface.

		Example:
			package app
			type UserService interface {
			}

			list := ctx.Bean(reflect.TypeOf((*app.UserService)(nil)).Elem(), 0)

		Lookup level defines how deep we will go in to beans:

		level 0: look in the current context, if not found then look in the parent context and so on (default)
		level 1: look only in the current context
		level 2: look in the current context in union with the parent context
		level 3: look in union of current, parent, parent of parent contexts
		and so on.
		level -1: look in union of all contexts.
	*/
	Bean(typ reflect.Type, level int) []Bean

	/*
		Lookup registered beans in context by name.
		The name is the local package plus name of the interface, for example 'app.UserService'
		Or if bean implements NamedBean interface the name of it.

		Example:
			beans := ctx.Bean("app.UserService")
			beans := ctx.Bean("userService")

		Lookup parent context only for beans that were used in injection inside ctx context.
		If you need to lookup all beans, use the loop with Parent() call.
	*/
	Lookup(name string, level int) []Bean

	/*
		Inject fields in to the obj on runtime that is not part of core context.
		Does not add a new bean in to the core context, so this method is only for one-time use with scope 'runtime'.
		Does not initialize bean and does not destroy it.

		Example:
			type requestProcessor struct {
				app.UserService  `inject`
			}

			rp := new(requestProcessor)
			ctx.Inject(rp)
			required.NotNil(t, rp.UserService)
	*/
	Inject(interface{}) error

	/*
		Returns resource and true if found
		File should come with ResourceSource name prefix.
		Uses default level of lookup for the resource.
	*/
	Resource(path string) (Resource, bool)

	/*
		Returns context placeholder properties
	*/
	Properties() Properties

	/*
		Returns information about context
	*/
	String() string
}

/*
This interface used to provide pre-scanned instances in glue.New method.
When glue sees that instance implements Scanner interface, instead of adding
instance itself to the context, glue it will call the method ScannerBeans() and
add array of instances in to context.

Used for conditional or modular instance discovery.

The common usage is to place scanner in to scan.go file with enumerated list of beans.
Scanner made as a interface to have a state and application can load beans differently
depending on environment variables or other settings.
*/

var ScannerClass = reflect.TypeOf((*Scanner)(nil)).Elem()

type Scanner interface {

	/*
		Returns pre-scanned instances
	*/
	ScannerBeans() []interface{}
}

/*
ChildContext is using to skip and delay initialization of the group of beans until application really needs it.
It gives ability to declare hierarchy of context with lazy loading on demand.

Use method glue.Child(name string, scan... interface{}) to initialize this special bean.
*/

var ChildContextClass = reflect.TypeOf((*ChildContext)(nil)).Elem()

type ChildContext interface {

	/*
		Returns the child context name, this name is not unique.
	*/
	ChildName() string

	/*
		Builds ctx context on the first request or returns existing one for all sequential calls.
	*/
	Object() (Context, error)

	/*
		Close ctx context if it was created. Safe to call twice or more.
		Parent context is owning and responsible to close all ctx contexts created on demand.
	*/
	Close() error
}

/*
The bean object would be created after Object() function call.

ObjectType can be pointer to structure or interface.

All objects for now created are singletons, that means single instance with ObjectType in context.
*/

var FactoryBeanClass = reflect.TypeOf((*FactoryBean)(nil)).Elem()

type FactoryBean interface {

	/*
		returns an object produced by the factory, and this is the object that will be used in context, but not going to be a bean
	*/
	Object() (interface{}, error)

	/*
		returns the type of object that this FactoryBean produces
	*/
	ObjectType() reflect.Type

	/*
		returns the bean name of object that this FactoryBean produces or empty string if name not defined
	*/
	ObjectName() string

	/*
		denotes if the object produced by this FactoryBean is a singleton
	*/
	Singleton() bool
}

/*
Initializing bean context is using to run required method on post-construct injection stage
*/

var InitializingBeanClass = reflect.TypeOf((*InitializingBean)(nil)).Elem()

type InitializingBean interface {

	/*
		Runs this method automatically after initializing and injecting context
	*/

	PostConstruct() error
}

/*
*
This interface uses to select objects that could free resources after closing context
*/
var DisposableBeanClass = reflect.TypeOf((*DisposableBean)(nil)).Elem()

type DisposableBean interface {

	/*
		During close context would be called for each bean in the core.
	*/

	Destroy() error
}

/*
*
This interface used to collect all beans with similar type in map, where the name is the key
*/
var NamedBeanClass = reflect.TypeOf((*NamedBean)(nil)).Elem()

type NamedBean interface {

	/*
		Returns bean name
	*/
	BeanName() string
}

/*
*
This interface used to collect beans in list with specific order
*/
var OrderedBeanClass = reflect.TypeOf((*OrderedBean)(nil)).Elem()

type OrderedBean interface {

	/*
		Returns bean order
	*/
	BeanOrder() int
}

/**
Resource source is using to add bind resources in to the context
*/

var ResourceSourceClass = reflect.TypeOf((*ResourceSource)(nil))

type ResourceSource struct {

	/*
		Used for resource reference based on pattern "name:path"
		ResourceSource instances sharing the same name would be merge and on conflict resource names would generate errors.
	*/
	Name string

	/*
		Known paths
	*/
	AssetNames []string

	/*
		FileSystem to access or serve assets or resources
	*/
	AssetFiles http.FileSystem
}

/*
Property source is serving as a property placeholder of file if it's ending with ".properties", ".props", ".yaml" or ".yml".
*/

var PropertySourceClass = reflect.TypeOf((*PropertySource)(nil))

type PropertySource struct {

	/*
		File to the properties file with prefix name of ResourceSource as "<resource_name>:path" or "file:path" in os.FileSystem.
	*/
	File string

	/*
		Map of properties
	*/
	Map map[string]interface{}
}

var FilePropertySourceClass = reflect.TypeOf((*FilePropertySource)(nil)).Elem()

type FilePropertySource string

var MapPropertySourceClass = reflect.TypeOf((*MapPropertySource)(nil)).Elem()

type MapPropertySource map[string]interface{}

/*
Property Resolver interface used to enhance the Properties interface with additional sources of properties.
*/

var PropertyResolverClass = reflect.TypeOf((*PropertyResolver)(nil))

type PropertyResolver interface {

	/*
		Priority in property resolving, it could be lower or higher than default one.
	*/
	Priority() int

	/*
		Resolves the property
	*/
	GetProperty(key string) (value string, ok bool)
}

/*
Use this bean to parse properties from file and place in context.
Merge properties from multiple PropertySource files in to one Properties bean.
For placeholder properties this bean used as a source of values.

Internal property storage has default priority of property resolver.
The higher priority look first.
*/

const defaultPropertyResolverPriority = 100

var PropertiesClass = reflect.TypeOf((*Properties)(nil))

type Properties interface {
	PropertyResolver

	/*
		Register additional property resolver. It would be sorted by priority.
	*/
	Register(PropertyResolver)
	PropertyResolvers() []PropertyResolver

	/*
		Loads properties from map
	*/
	LoadMap(source map[string]interface{})

	/*
		Loads properties from input stream
	*/
	Load(reader io.Reader) error

	/*
		Saves properties to output stream
	*/
	Save(writer io.Writer) (n int, err error)

	/*
		Parsing content as an UTF-8 string
	*/
	Parse(content string) error

	/*
		Dumps all properties to UTF-8 string
	*/
	Dump() string

	/*
		Extends parent properties
	*/
	Extend(parent Properties)

	/*
		Gets length of the properties
	*/
	Len() int

	/*
		Gets all keys associated with properties
	*/
	Keys() []string

	/*
		Return copy of properties as Map
	*/
	Map() map[string]string

	/*
		Checks if property contains the key
	*/
	Contains(key string) bool

	/*
		Gets property value and true if exist
	*/
	Get(key string) (value string, ok bool)

	/*
		Additional getters with type conversion
	*/
	GetString(key, def string) string
	GetBool(key string, def bool) bool
	GetInt(key string, def int) int
	GetFloat(key string, def float32) float32
	GetDouble(key string, def float64) float64
	GetDuration(key string, def time.Duration) time.Duration
	GetFileMode(key string, def os.FileMode) os.FileMode

	// properties conversion error handler
	GetErrorHandler() func(string, error)
	SetErrorHandler(onError func(string, error))

	/*
		Sets property value
	*/
	Set(key string, value string)

	/*
		Remove property by key
	*/
	Remove(key string) bool

	/*
		Delete all properties and comments
	*/
	Clear()

	/*
		Gets comments associated with property
	*/
	GetComments(key string) []string

	/*
		Sets comments associated with property
	*/
	SetComments(key string, comments []string)

	/*
		ClearComments removes the comments for all keys.
	*/
	ClearComments()
}

/*
*
This interface used to access the specific resource
*/
var ResourceClass = reflect.TypeOf((*Resource)(nil)).Elem()

type Resource interface {
	Open() (http.File, error)
}
