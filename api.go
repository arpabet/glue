/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"context"
	"io"
	"net/http"
	"os"
	"reflect"
	"time"
)

type BeanLifecycle int32

// ActiveProfilesProperty - use mostly in Child containers
var ActiveProfilesProperty = "glue.profiles.active"

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

/*
BeanScope defines the lifecycle scope of a bean.
*/

type BeanScope int

const (
	// ScopeSingleton is the default scope — one instance per container.
	ScopeSingleton BeanScope = iota

	// ScopePrototype creates a new instance on every resolution.
	// Injected as func() (T, error) or func(context.Context) (T, error).
	ScopePrototype

	// ScopeRequest creates one instance per request scope.
	// Injected as func(context.Context) (T, error) where context carries the RequestScope.
	ScopeRequest
)

func (s BeanScope) String() string {
	switch s {
	case ScopeSingleton:
		return "singleton"
	case ScopePrototype:
		return "prototype"
	case ScopeRequest:
		return "request"
	default:
		return "unknown"
	}
}

var BeanClass = reflect.TypeOf((*Bean)(nil)).Elem()

/*
	Bean interface is the major component in this framework that represents the atomic object that has relations to other objects
*/

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
	Object() any

	/*
		Returns factory bean of exist only beans created by FactoryBean interface
	*/
	FactoryBean() (Bean, bool)

	/*
		Returns current bean lifecycle
	*/
	Lifecycle() BeanLifecycle

	/*
		Returns information about the bean
	*/
	String() string
}

type ContainerLogger interface {

	// Enabled - return true if log is enabled
	Enabled() bool

	// Printf calls l.Output to print to the logger.
	// Arguments are handled in the manner of [fmt.Printf].
	Printf(format string, v ...any)

	// Println calls l.Output to print to the logger.
	// Arguments are handled in the manner of [fmt.Println].
	Println(v ...any)
}

var ContainerClass = reflect.TypeOf((*Container)(nil)).Elem()

type ContainerOptions struct {
	Context        context.Context
	Properties     Properties
	ActiveProfiles []string
	Logger         ContainerLogger
}

type ContainerOption func(*ContainerOptions)

func WithContext(ctx context.Context) ContainerOption {
	return func(opts *ContainerOptions) {
		opts.Context = ctx
	}
}

func WithProperties(properties Properties) ContainerOption {
	return func(opts *ContainerOptions) {
		opts.Properties = properties
	}
}

func WithProfiles(profiles ...string) ContainerOption {
	return func(opts *ContainerOptions) {
		if len(profiles) == 0 {
			opts.ActiveProfiles = nil
			return
		}
		opts.ActiveProfiles = append([]string(nil), profiles...)
	}
}

func WithLogger(log ContainerLogger) ContainerOption {
	return func(opts *ContainerOptions) {
		opts.Logger = log
	}
}

/**
Container interface is why this framework exist, maintains the set of beans and relations between them.
*/

type Container interface {
	/*
		Parent - gets the parent container if exist
	*/
	Parent() (Container, bool)

	/*
		Extend - creates a new container on top of beans from the current container
	*/
	Extend(scan ...any) (Container, error)

	/*
		ExtendWithContext - creates a new container on top of beans from the current container with context
	*/
	ExtendWithContext(ctx context.Context, scan ...any) (Container, error)

	/*
		ExtendWithOptions - creates a new container on top of beans from the current container with options
	*/
	ExtendWithOptions(options []ContainerOption, scan ...any) (Container, error)

	/*
		Children - Returns list of ctx container inside the current container only
	*/
	Children() []ChildContainer

	/*
		Close - Destroy all beans that implement interface DisposableBean
	*/
	Close() error

	/*
		CloseWithContext - Destroy all beans that implement interface DisposableBean with context
	*/
	CloseWithContext(ctx context.Context) error

	/*
		Reload - Re-resolve static value properties and reinitialize bean by calling
		Destroy then PostConstruct. Dynamic func() properties are not affected since
		they already read live values. Inject fields are not re-resolved.

		Can not be used for beans created by FactoryBean.
	*/
	Reload(bean Bean) error

	/*
		ReloadWithContext - same as Reload but with provided context for context-aware lifecycle interfaces
	*/
	ReloadWithContext(ctx context.Context, bean Bean) error

	/*
		Core - Get list of all registered instances on creation of container with scope 'core'
	*/
	Core() []reflect.Type

	/*
		Bean - Gets obj by type, that is a pointer to the structure or interface.

		Example:
			package app
			type UserService interface {
			}

			list := ctx.Bean(reflect.TypeOf((*app.UserService)(nil)).Elem(), 0)

		Lookup level defines how deep we will go in to beans:

		level 0: look in the current container, if not found then look in the parent container and so on (default)
		level 1: look only in the current container
		level 2: look in the current container in union with the parent container
		level 3: look in union of current, parent, parent of parent contexts
		and so on.
		level -1: look in union of all contexts.
	*/
	Bean(typ reflect.Type, level int) []Bean

	/*
		Lookup registered beans in container by name.
		The name is the local package plus name of the interface, for example 'app.UserService'
		Or if bean implements NamedBean interface the name of it.

		Example:
			beans := ctx.Bean("app.UserService")
			beans := ctx.Bean("userService")

		Lookup parent container only for beans that were used in injection inside ctx container.
		If you need to lookup all beans, use the loop with Parent() call.
	*/
	Lookup(name string, level int) []Bean

	/*
		Inject fields in to the obj on runtime that is not part of core container.
		Does not add a new bean in to the core container, so this method is only for one-time use with scope 'runtime'.
		Does not initialize bean and does not destroy it.

		Example:
			type requestProcessor struct {
				app.UserService  `inject:""`
			}

			rp := new(requestProcessor)
			ctx.Inject(rp)
			required.NotNil(t, rp.UserService)
	*/
	Inject(any) error

	/*
		Returns resource and true if found
		File should come with ResourceSource name prefix.
		Uses default level of lookup for the resource.
	*/
	Resource(path string) (Resource, bool)

	/*
		Returns container placeholder properties
	*/
	Properties() Properties

	/*
		Returns information about container
	*/
	String() string
}

var ScopedBeanClass = reflect.TypeOf((*ScopedBean)(nil)).Elem()

/*
ScopedBean is optionally implemented by beans to declare their scope.
Beans that do not implement ScopedBean are treated as singletons.
*/
type ScopedBean interface {

	/*
		BeanScope returns the scope of this bean.
	*/
	BeanScope() BeanScope
}

/*
RequestScope holds per-request scoped bean instances. It is stored in a context.Context
and retrieved by scope=request provider functions generated by the container.

Create a new RequestScope for each logical request and attach it to the context
using WithRequestScope. The container-generated provider functions will automatically
resolve the RequestScope from the context, cache instances within it, and destroy
request-scoped disposables when the scope is closed.
*/
type RequestScope interface {
	Close() error
	CloseWithContext(ctx context.Context) error
}

var ProfileBeanClass = reflect.TypeOf((*ProfileBean)(nil)).Elem()

/*
	    ProfileBean is implemented by beans that should only be registered when specific profiles are active.

		Default property for active profiles: glue.profiles.active

		Profile expression syntax:

		"" - no profiles
		"*" - all profiles
		"dev" — active when "dev" profile is active
		"!prod" — active when "prod" profile is NOT active
		"dev|staging" — active when either "dev" or "staging" is active
		"dev&local" — active when both "dev" and "local" are active
*/
type ProfileBean interface {

	/*
		BeanProfile - returns the string that represents pattern of current profile where bean should be included in container
	*/
	BeanProfile() string // e.g., "dev", "!prod", "dev|staging"
}

var ConditionalBeanClass = reflect.TypeOf((*ConditionalBean)(nil)).Elem()

/*
ConditionalBean is optionally implemented by beans. If Condition() returns false,

	the bean is not registered in the context.

	ProfileBean is checked first (cheap string match), then ConditionalBean
	(may do I/O or complex logic). Both can be implemented on the same bean.
*/
type ConditionalBean interface {

	/*
	   ShouldRegisterBean returns true if this bean should be registered
	   in the container. Called during scanning, before injection an initialization in PostConstruct.
	*/
	ShouldRegisterBean() bool
}

var ScannerClass = reflect.TypeOf((*Scanner)(nil)).Elem()

/*
Scanner interface used to provide pre-scanned instances in glue.New method.
When glue sees that instance implements Scanner interface, instead of adding
instance itself to the container, glue it will call the method ScannerBeans() and
add array of instances in to container.

Used for conditional or modular instance discovery.

The common usage is to place scanner in to scan.go file with enumerated list of beans.
Scanner made as a interface to have a state and application can load beans differently
depending on environment variables or other settings.
*/

type Scanner interface {

	/*
		ScannerBeans - returns pre-scanned instances
	*/
	ScannerBeans() []any
}

/*
	ProfileScanner - Interface that joins ProfileBean and Scanner
*/

type ProfileScanner interface {
	ProfileBean
	Scanner
}

type profileScanner struct {
	profile string
	beans   []any
}

func (t *profileScanner) BeanProfile() string {
	return t.profile
}

func (t *profileScanner) ScannerBeans() []any {
	return t.beans
}

// IfProfile Shortcut method for container creation
func IfProfile(profile string, scan ...any) ProfileScanner {
	return &profileScanner{profile: profile, beans: scan}
}

/*
ChildContainer is using to skip and delay initialization of the group of beans until application really needs it.
It gives ability to declare hierarchy of container with lazy loading on demand.

Use method glue.Child(name string, scan... any) to initialize this special bean.
*/

var ChildContainerClass = reflect.TypeOf((*ChildContainer)(nil)).Elem()

type ChildContainer interface {

	/*
		Returns the child container name, this name is not unique.
	*/
	ChildName() string

	/*
		Builds ctx container on the first request or returns existing one for all sequential calls.
	*/
	Object() (Container, error)

	/*
		Builds ctx container on the first request using the provided context or returns existing one for all sequential calls.
	*/
	ObjectWithContext(ctx context.Context) (Container, error)

	/*
		Close ctx container if it was created. Safe to call twice or more.
		Parent container is owning and responsible to close all ctx contexts created on demand.
	*/
	Close() error

	/*
		CloseWithContext - closes ctx container if it was created with context
	*/
	CloseWithContext(ctx context.Context) error
}

/*
The bean object would be created after Object() function call.

ObjectType can be pointer to structure or interface.

All objects for now created are singletons, that means single instance with ObjectType in container.
*/

var FactoryBeanClass = reflect.TypeOf((*FactoryBean)(nil)).Elem()

type FactoryBean interface {

	/*
		returns an object produced by the factory, and this is the object that will be used in container, but not going to be a bean
	*/
	Object() (any, error)

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

var ContextFactoryBeanClass = reflect.TypeOf((*ContextFactoryBean)(nil)).Elem()

type ContextFactoryBean interface {

	/*
		returns an object produced by the factory using the current container construction context
	*/
	Object(ctx context.Context) (any, error)

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

var InitializingBeanClass = reflect.TypeOf((*InitializingBean)(nil)).Elem()

/*
InitializingBean interface is using to run required method on post-construct injection stage
*/

type InitializingBean interface {

	/*
		PostConstruct - Runs this method automatically after initializing and injecting container
	*/

	PostConstruct() error
}

var ContextInitializingBeanClass = reflect.TypeOf((*ContextInitializingBean)(nil)).Elem()

/*
ContextInitializingBean interface is using to run required method on post-construct injection stage with context
*/

type ContextInitializingBean interface {

	/*
		PostConstructWithContext - Runs this method automatically after initializing and injecting container with context
	*/

	PostConstruct(ctx context.Context) error
}

var DisposableBeanClass = reflect.TypeOf((*DisposableBean)(nil)).Elem()

/*
DisposableBean uses to select objects that could free resources after closing container
*/

type DisposableBean interface {

	/*
		Destroy - close container would be called for each bean in the container
	*/

	Destroy() error
}

var ContextDisposableBeanClass = reflect.TypeOf((*ContextDisposableBean)(nil)).Elem()

/*
ContextDisposableBean uses to select objects that could free resources after closing container with context
*/

type ContextDisposableBean interface {

	/*
		DestroyWithContext - close container would be called for each bean in the container with context
	*/

	Destroy(ctx context.Context) error
}

var NamedBeanClass = reflect.TypeOf((*NamedBean)(nil)).Elem()

/*
NamedBean interface used to collect all beans with similar type in map, where the name is the key
*/

type NamedBean interface {

	/*
		BeanName - returns unique bean name (qualifier)
	*/
	BeanName() string
}

var OrderedBeanClass = reflect.TypeOf((*OrderedBean)(nil)).Elem()

/*
OrderedBean interface used to collect beans in list with specific order
*/

type OrderedBean interface {

	/*
		BeanOrder - returns bean order using for acceding sorting of beans before injecting to collection
	*/
	BeanOrder() int
}

var PrimaryBeanClass = reflect.TypeOf((*PrimaryBean)(nil)).Elem()

/*
PrimaryBean interface used to mark a bean as primary among multiple implementations of the same interface.
When multiple beans implement the same interface, the primary bean will be injected by default.
This does not affect collection and order of the beans.
If two+ primary beans found for one single field injection the ambiguation error would be returned.
*/

type PrimaryBean interface {

	/*
		IsPrimaryBean - returns true if this bean should be considered the primary implementation
	*/
	IsPrimaryBean() bool
}

var ResourceSourceClass = reflect.TypeOf((*ResourceSource)(nil))

/**
ResourceSource is using to add bind resources in to the container
*/

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

var PropertySourceClass = reflect.TypeOf((*PropertySource)(nil))

/*
PropertySource is serving as a property placeholder of file if it's ending with ".properties", ".props", ".yaml" or ".yml".
*/

type PropertySource struct {

	/*
		File to the properties file with prefix name of ResourceSource as "<resource_name>:path" or "file:path" in os.FileSystem.
	*/
	File string

	/*
		Map of properties
	*/
	Map map[string]any
}

var FilePropertySourceClass = reflect.TypeOf((*FilePropertySource)(nil)).Elem()

type FilePropertySource string

var MapPropertySourceClass = reflect.TypeOf((*MapPropertySource)(nil)).Elem()

type MapPropertySource map[string]any

var PropertyResolverClass = reflect.TypeOf((*PropertyResolver)(nil))

/*
PropertyResolver interface used to enhance the Properties interface with additional sources of properties.
*/

type PropertyResolver interface {

	/*
		Priority in property resolving, it could be lower or higher than default one.
	*/
	Priority() int

	/*
		GetProperty - Resolves the property
	*/
	GetProperty(key string) (value string, ok bool)
}

/*
Use this bean to parse properties from file and place in container.
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
	LoadMap(source map[string]any)

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
		Delete all properties
	*/
	Clear()
}

/*
*
This interface used to access the specific resource
*/
var ResourceClass = reflect.TypeOf((*Resource)(nil)).Elem()

type Resource interface {
	Open() (http.File, error)
}
