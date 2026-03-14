/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var DefaultCloseTimeout = time.Minute

type container struct {

	/**
	Parent container if exist
	*/
	parent *container

	/**
	Recognized ctx container list
	*/
	children []ChildContainer

	/**
		All instances scanned during creation of container.
	    No modifications on runtime allowed.
	*/
	core map[reflect.Type][]*bean

	/**
	All bean names scanned during creation of container.
	No modifications on runtime allowed.
	*/
	localNames map[string][]*bean

	/**
	List of beans in initialization order that should depose on close
	*/
	disposables []*bean

	/**
	Mutable cache for interface-to-implementation lookups
	*/
	ifaceCache interfaceCache

	/**
	Resource sources registered during container creation.
	No modifications on runtime allowed.
	*/
	resourceSources resourceCache

	/**
	Placeholder properties of the container
	*/
	properties Properties

	/*
		Never null container logger
	*/
	logger ContainerLogger

	/**
	Guarantees that container would be closed once
	*/
	closeOnce sync.Once
}

func New(scan ...any) (Container, error) {
	return NewWithOptions(nil, scan...)
}

func NewWithProfiles(activeProfiles []string, scan ...any) (Container, error) {
	return NewWithOptions([]ContainerOption{WithProfiles(activeProfiles...)}, scan...)
}

func NewWithContext(ctx context.Context, scan ...any) (Container, error) {
	return NewWithOptions([]ContainerOption{WithContext(ctx)}, scan...)
}

func NewWithProperties(ctx context.Context, properties Properties, scan ...any) (Container, error) {
	return NewWithOptions([]ContainerOption{
		WithContext(ctx),
		WithProperties(properties),
	}, scan...)
}

func NewWithOptions(options []ContainerOption, scan ...any) (Container, error) {
	return createContainer(nil, buildContainerOptions(options), scan)
}

func defaultContainerOptions() ContainerOptions {
	return ContainerOptions{
		Context:    context.Background(),
		Properties: NewProperties(),
	}
}

func buildContainerOptions(options []ContainerOption) ContainerOptions {
	opts := defaultContainerOptions()
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if opts.Properties == nil {
		opts.Properties = NewProperties()
	}
	if opts.ActiveProfiles != nil {
		opts.ActiveProfiles = append([]string(nil), opts.ActiveProfiles...)
	}
	if opts.Logger == nil && verbose != nil {
		opts.Logger = logAdapter{log: verbose}
	}
	return opts
}

func (t *container) Extend(scan ...any) (Container, error) {
	return t.ExtendWithOptions(nil, scan...)
}

func (t *container) ExtendWithContext(ctx context.Context, scan ...any) (Container, error) {
	return t.ExtendWithOptions([]ContainerOption{WithContext(ctx)}, scan...)
}

func (t *container) ExtendWithOptions(options []ContainerOption, scan ...any) (Container, error) {

	properties := NewProperties()
	properties.Extend(t.properties)

	opts := buildContainerOptions(options)
	overrideProperties := false
	for _, opt := range options {
		if opt == nil {
			continue
		}
		probe := ContainerOptions{}
		opt(&probe)
		if probe.Properties != nil {
			overrideProperties = true
			break
		}
	}
	if !overrideProperties {
		opts.Properties = properties
	}

	return createContainer(t, opts, scan)
}

func (t *container) Parent() (Container, bool) {
	if t.parent != nil {
		return t.parent, true
	} else {
		return nil, false
	}
}

func getActiveProfiles(properties Properties) []string {
	var profiles []string
	if properties == nil {
		return profiles
	}
	if commaListStr, ok := properties.Get(ActiveProfilesProperty); ok {
		for _, part := range strings.Split(commaListStr, ",") {
			profile := strings.TrimSpace(part)
			if profile != "" {
				profiles = append(profiles, profile)
			}
		}
	}
	return profiles
}

func createContainer(parent *container, options ContainerOptions, scan []any) (c *container, err error) {

	core := make(map[reflect.Type][]*bean)
	localNames := make(map[string][]*bean)
	pointers := make(map[reflect.Type][]*injection)
	interfaces := make(map[reflect.Type][]*injection)

	var propertySources []*PropertySource
	var propertyResolvers []PropertyResolver
	var primaryList []*bean
	var secondaryList []*bean

	activeProfiles := options.ActiveProfiles
	if len(activeProfiles) == 0 {
		activeProfiles = getActiveProfiles(options.Properties)
	}

	active := make(map[string]struct{}, len(activeProfiles))
	for _, profile := range activeProfiles {
		profile = strings.TrimSpace(profile)
		if profile != "" {
			active[profile] = struct{}{}
		}
	}

	if options.Logger == nil {
		options.Logger = nullLogger{}
	}

	c = &container{
		parent:          parent,
		core:            core,
		localNames:      localNames,
		ifaceCache:      ctorInterfaceCache(),
		resourceSources: ctorResourceCache(),
		properties:      options.Properties,
		logger:          options.Logger,
	}

	// add container bean to core
	ctnBean := &bean{
		obj:      c,
		valuePtr: reflect.ValueOf(c),
		beanDef: &beanDef{
			classPtr: reflect.TypeOf(c),
		},
		lifecycle: BeanInitialized,
	}
	core[ctnBean.beanDef.classPtr] = []*bean{ctnBean}

	// add properties bean to core
	propertiesBean := &bean{
		obj:      c,
		valuePtr: reflect.ValueOf(c.properties),
		beanDef: &beanDef{
			classPtr: reflect.TypeOf(c.properties),
		},
		lifecycle: BeanInitialized,
	}
	core[propertiesBean.beanDef.classPtr] = []*bean{propertiesBean}

	// scan
	err = forEach(active, "", scan, func(pos string, obj any) (err error) {

		var resolver bool

		switch instance := obj.(type) {
		case ChildContainer:
			c.logger.Printf("ChildContainer %s\n", instance.ChildName())
			c.children = append(c.children, instance)
			// register interrest by making a placeholder
			if _, ok := interfaces[ChildContainerClass]; !ok {
				interfaces[ChildContainerClass] = []*injection{}
			}
		case ResourceSource:
			c.logger.Printf("ResourceSource %s, assets %+v\n", instance.Name, instance.AssetNames)
			ptr := &instance
			if err := c.resourceSources.addResourceSource(ptr); err != nil {
				return err
			}
			obj = ptr
		case *ResourceSource:
			c.logger.Printf("ResourceSource %s, assets %+v\n", instance.Name, instance.AssetNames)
			if err := c.resourceSources.addResourceSource(instance); err != nil {
				return err
			}
		case PropertySource:
			c.logger.Printf("PropertySource %s %d\n", instance.File, len(instance.Map))
			ptr := &instance
			propertySources = append(propertySources, ptr)
			obj = ptr
		case *PropertySource:
			c.logger.Printf("PropertySource %s %d\n", instance.File, len(instance.Map))
			propertySources = append(propertySources, instance)
		case FilePropertySource:
			fileName := string(instance)
			c.logger.Printf("FilePropertySource %s\n", fileName)
			// does not do to the container, since it is not a pointer or interface, instead the &PropertySource object would be created
			ps := &PropertySource{File: fileName}
			propertySources = append(propertySources, ps)
			obj = ps
		case MapPropertySource:
			c.logger.Printf("MapPropertySource %d\n", len(instance))
			// does not do to the container, since it is not a pointer or interface, instead the &PropertySource object would be created
			ps := &PropertySource{Map: instance}
			propertySources = append(propertySources, ps)
			obj = ps
		case PropertyResolver:
			c.logger.Printf("PropertyResolver Priority %d\n", instance.Priority())
			propertyResolvers = append(propertyResolvers, instance)
			resolver = true
		default:
		}

		classPtr := reflect.TypeOf(obj)

		defer func() {
			if r := recover(); r != nil {
				err = errors.Errorf("recover from object scan '%s' on error %v\n", classPtr.String(), r)
			}
		}()

		switch classPtr.Kind() {
		case reflect.Struct:
			// auto-wrap struct value to pointer
			ptr := reflect.New(classPtr)
			ptr.Elem().Set(reflect.ValueOf(obj))
			obj = ptr.Interface()
			classPtr = reflect.TypeOf(obj)
			fallthrough
		case reflect.Ptr:
			/**
			New bean from object
			*/
			objBean, err := investigate(obj, classPtr)
			if err != nil {
				return err
			}

			var elemClassPtr reflect.Type
			factoryBean, isFactoryBean := obj.(FactoryBean)
			contextFactoryBean, isContextFactoryBean := obj.(ContextFactoryBean)
			if isContextFactoryBean {
				elemClassPtr = contextFactoryBean.ObjectType()
			} else if isFactoryBean {
				elemClassPtr = factoryBean.ObjectType()
			}
			if c.logger.Enabled() {
				if isFactoryBean || isContextFactoryBean {
					var info string
					if (isContextFactoryBean && contextFactoryBean.Singleton()) || (!isContextFactoryBean && factoryBean.Singleton()) {
						info = "singleton"
					} else {
						info = "non-singleton"
					}
					objectName := elemClassPtr.String()
					if isContextFactoryBean {
						if name := contextFactoryBean.ObjectName(); name != "" {
							objectName = name
						}
					} else if name := factoryBean.ObjectName(); name != "" {
						objectName = name
					}
					if objectName != "" {
						c.logger.Printf("FactoryBean %v produce %s %v with name '%s'\n", classPtr, info, elemClassPtr, objectName)
					} else {
						c.logger.Printf("FactoryBean %v produce %s %v\n", classPtr, info, elemClassPtr)
					}
				} else {
					if objBean.qualifier != "" {
						c.logger.Printf("Bean %v with name '%s'\n", classPtr, objBean.qualifier)
					} else {
						c.logger.Printf("Bean %v\n", classPtr)
					}
				}
			}

			if isFactoryBean || isContextFactoryBean {
				elemClassKind := elemClassPtr.Kind()
				if elemClassKind != reflect.Ptr && elemClassKind != reflect.Interface {
					return errors.Errorf("factory bean '%v' on position '%s' can produce ptr or interface, but object type is '%v'", classPtr, pos, elemClassPtr)
				}
			}

			/**
			Enumerate injection fields
			*/
			if len(objBean.beanDef.fields) > 0 {
				value := objBean.valuePtr.Elem()
				for _, injectDef := range objBean.beanDef.fields {
					if c.logger.Enabled() {
						var attr []string
						if injectDef.lazy {
							attr = append(attr, "lazy")
						}
						if injectDef.optional {
							attr = append(attr, "optional")
						}
						if injectDef.qualifier != "" {
							attr = append(attr, "bean="+injectDef.qualifier)
						}
						var attrs string
						if len(attr) > 0 {
							attrs = fmt.Sprintf("[%s]", strings.Join(attr, ","))
						}
						var prefix string
						if injectDef.isSlice {
							prefix = "[]"
						}
						if injectDef.isMap {
							prefix = "map[string]"
						}
						c.logger.Printf("	Field %s%v %s\n", prefix, injectDef.fieldType, attrs)
					}

					if injectDef.scope != ScopeSingleton {
						// Scoped injection: resolve by the return type of the provider function, not the function type itself
						lookupType := injectDef.scopeReturnType
						switch lookupType.Kind() {
						case reflect.Ptr:
							pointers[lookupType] = append(pointers[lookupType], &injection{objBean, value, injectDef, c})
						case reflect.Interface:
							interfaces[lookupType] = append(interfaces[lookupType], &injection{objBean, value, injectDef, c})
						default:
							return errors.Errorf("scoped field '%s' in '%v': return type '%v' must be pointer or interface", injectDef.fieldName, classPtr, lookupType)
						}
					} else {
						switch injectDef.fieldType.Kind() {
						case reflect.Ptr:
							pointers[injectDef.fieldType] = append(pointers[injectDef.fieldType], &injection{objBean, value, injectDef, c})
						case reflect.Interface:
							interfaces[injectDef.fieldType] = append(interfaces[injectDef.fieldType], &injection{objBean, value, injectDef, c})
						default:
							return errors.Errorf("injecting not a pointer or interface on field type '%v' at position '%s' in %v", injectDef.fieldType, pos, classPtr)
						}
					}
				}
			}

			/*
				Register factory if needed
			*/
			if isFactoryBean || isContextFactoryBean {
				f := &factory{
					bean:               objBean,
					factoryObj:         obj,
					factoryClassPtr:    classPtr,
					factoryBean:        factoryBean,
					contextFactoryBean: contextFactoryBean,
				}
				objectName := f.objectName()
				if objectName == "" {
					objectName = elemClassPtr.String()
				}
				elemBean := &bean{
					name:        objectName,
					beenFactory: f,
					beanDef: &beanDef{
						classPtr: elemClassPtr,
					},
					lifecycle: BeanAllocated,
				}
				f.instances = []*bean{elemBean}
				// we can have singleton or multiple beans in container produced by this factory, let's allocate reference for injections even if those beans are still not exist
				registerBean(core, localNames, elemClassPtr, elemBean)
				secondaryList = append(secondaryList, elemBean)
			}

			/*
				Register bean itself
			*/
			registerBean(core, localNames, classPtr, objBean)

			/**
			Initialize property resolver beans at first
			*/
			if resolver {
				primaryList = append(primaryList, objBean)
			} else {
				secondaryList = append(secondaryList, objBean)
			}

		default:
			return errors.Errorf("instance must be a pointer to a struct or interface (got '%s' at position '%s' of type '%v')", classPtr.Kind().String(), pos, classPtr)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// direct match
	for requiredType, injects := range pointers {

		c.logger.Println("Object", requiredType, len(injects))

		direct := c.findObjectRecursive(requiredType)
		if len(direct) > 0 {

			c.logger.Printf("Inject '%v' by pointer '%+v' in to %+v\n", requiredType, direct, injects)

			for _, inject := range injects {
				if err := inject.inject(direct); err != nil {
					return nil, errors.Errorf("required type '%s' injection error, %v", requiredType, err)
				}
			}

		} else {

			c.logger.Printf("Bean '%v' not found in container\n", requiredType)

			var required []*injection
			for _, inject := range injects {
				if inject.injectionDef.optional {
					c.logger.Printf("Skip optional inject '%v' in to '%v'\n", requiredType, inject)
				} else {
					required = append(required, inject)
				}
			}

			if len(required) > 0 {
				return nil, errors.Errorf("can not find candidates for '%v' reference bean required by '%+v'", requiredType, required)
			}

		}
	}

	// interface match
	for ifaceType, injects := range interfaces {

		c.logger.Println("Interface", ifaceType, len(injects))

		candidates := c.searchAndCacheInterfaceCandidatesRecursive(ifaceType)
		if len(candidates) == 0 {

			c.logger.Printf("No found bean candidates for interface '%v' in container\n", ifaceType)

			var required []*injection
			for _, inject := range injects {
				if inject.injectionDef.optional {
					c.logger.Printf("Skip optional inject of interface '%v' in to '%v'\n", ifaceType, inject)
				} else {
					required = append(required, inject)
				}
			}

			if len(required) > 0 {
				return nil, errors.Errorf("can not find candidates for '%v' interface required by '%+v'", ifaceType, required)
			}

			continue
		}

		for _, inject := range injects {

			c.logger.Printf("Inject '%v' by implementation '%+v' in to %+v\n", ifaceType, candidates, inject)

			if err := inject.inject(candidates); err != nil {
				return nil, errors.Errorf("interface '%s' injection error, %v", ifaceType, err)
			}

		}

	}

	/**
	Load properties from property sources
	*/
	if len(propertySources) > 0 {
		if err := c.loadProperties(propertySources); err != nil {
			return nil, err
		}
	}

	/**
	Register property resolvers from container
	*/
	for _, r := range propertyResolvers {
		c.properties.Register(r)
	}

	/**
	PostConstruct beans
	*/
	if err := c.postConstruct(options.Context, primaryList, secondaryList); err != nil {
		c.closeWithTimeout(DefaultCloseTimeout)
		return nil, err
	} else {
		return c, nil
	}

}

func (t *container) closeWithTimeout(timeout time.Duration) {
	ch := make(chan error)
	go func() {
		ch <- t.Close()
		close(ch)
	}()
	select {
	case e := <-ch:
		if e != nil {
			t.logger.Printf("Close container error, %v\n", e)
		}
	case <-time.After(timeout):
		t.logger.Printf("Close container timeout error.\n")
	}
}

func (t *container) loadPropertiesFromFile(filePath string, file io.Reader) error {

	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {

		holder := make(map[string]any)
		if err := yaml.NewDecoder(file).Decode(holder); err != nil {
			return errors.Errorf("failed to load properties form yaml file '%s', %v", filePath, err)
		}
		t.properties.LoadMap(holder)
		return nil

	} else if strings.HasSuffix(filePath, ".json") {

		data, err := io.ReadAll(file)
		if err != nil {
			return errors.Errorf("failed to read json file '%s', %v", filePath, err)
		}
		holder := make(map[string]any)
		if err := json.Unmarshal(data, &holder); err != nil {
			return errors.Errorf("failed to parse json file '%s', %v", filePath, err)
		}
		t.properties.LoadMap(holder)
		return nil

	} else if strings.HasSuffix(filePath, ".properties") {
		if err := t.properties.Load(file); err != nil {
			return errors.Errorf("failed to load properties form properties file '%s', %v", filePath, err)
		}
		return nil
	} else {
		return errors.Errorf("unsupported properties file '%s'", filePath)
	}
}

func (t *container) loadProperties(propertySources []*PropertySource) error {

	for _, source := range propertySources {

		if source.File != "" {

			if strings.HasPrefix(source.File, "file:") {

				filePath := source.File[len("file:"):]
				file, err := os.Open(filePath)
				if err != nil {
					return errors.Errorf("i/o error with placeholder properties file '%s', %v", filePath, err)
				}
				err = t.loadPropertiesFromFile(filePath, file)
				file.Close()
				if err != nil {
					return errors.Errorf("load error of placeholder properties file '%s', %v", filePath, err)
				}

			} else if resource, ok := t.Resource(source.File); ok {

				file, err := resource.Open()
				if err != nil {
					return errors.Errorf("i/o error with placeholder properties resource '%s', %v", source, err)
				}
				err = t.loadPropertiesFromFile(source.File, file)
				file.Close()
				if err != nil {
					return errors.Errorf("load error of placeholder properties resource '%s', %v", source.File, err)
				}

			} else {
				return errors.Errorf("placeholder properties resource '%s' was not found", source.File)
			}
		}

		if source.Map != nil {
			t.properties.LoadMap(source.Map)
		}

	}

	return nil
}

func registerBean(core map[reflect.Type][]*bean, localNames map[string][]*bean, classPtr reflect.Type, b *bean) {
	core[classPtr] = append(core[classPtr], b)
	localNames[b.name] = append(localNames[b.name], b)
}

func forEach(active map[string]struct{}, initialPos string, scan []any, cb func(i string, obj any) error) error {
	// Use a map to track visited objects by their pointer address
	visited := make(map[uintptr]bool)

	// Call helper function with visited map
	return forEachRecursive(active, initialPos, scan, cb, visited)
}

/*
	Profile expression syntax:

	"" - no profiles
	"*" - all profiles
	"dev" — active when "dev" profile is active
	"!prod" — active when "prod" profile is NOT active
	"dev|staging" — active when either "dev" or "staging" is active
	"dev&local" — active when both "dev" and "local" are active
*/

func isProfileActive(active map[string]struct{}, profileExpression string) bool {
	profileExpression = strings.TrimSpace(profileExpression)
	if profileExpression == "" {
		return false
	}
	if profileExpression == "*" {
		return true
	}

	for _, orPart := range strings.Split(profileExpression, "|") {
		orPart = strings.TrimSpace(orPart)
		if orPart == "" {
			continue
		}

		matched := true
		for _, andPart := range strings.Split(orPart, "&") {
			andPart = strings.TrimSpace(andPart)
			if andPart == "" {
				matched = false
				break
			}

			negated := strings.HasPrefix(andPart, "!")
			if negated {
				andPart = strings.TrimSpace(andPart[1:])
				if andPart == "" {
					matched = false
					break
				}
			}

			_, ok := active[andPart]
			if negated {
				ok = !ok
			}
			if !ok {
				matched = false
				break
			}
		}

		if matched {
			return true
		}
	}

	return false
}

func forEachRecursive(active map[string]struct{}, initialPos string, scan []any, cb func(i string, obj any) error, visited map[uintptr]bool) error {
	for j, item := range scan {

		if item == nil {
			continue
		}

		// Check if this is a pointer type that we can track
		if isPointer(item) {
			// Get the memory address as uintptr
			addr := reflect.ValueOf(item).Pointer()

			// Skip if already visited
			if visited[addr] {
				continue
			}

			// Mark as visited
			visited[addr] = true
		}

		if profileBean, ok := item.(ProfileBean); ok {
			if !isProfileActive(active, profileBean.BeanProfile()) {
				continue
			}
		}

		if conditionalBean, ok := item.(ConditionalBean); ok {
			if !conditionalBean.ShouldRegisterBean() {
				continue
			}
		}

		var pos string
		if len(initialPos) > 0 {
			pos = fmt.Sprintf("%s.%d", initialPos, j)
		} else {
			pos = strconv.Itoa(j)
		}

		switch obj := item.(type) {
		case Scanner:
			if err := forEachRecursive(active, pos, obj.ScannerBeans(), cb, visited); err != nil {
				return err
			}
		case []any:
			if err := forEachRecursive(active, pos, obj, cb, visited); err != nil {
				return err
			}
		case any:
			if err := cb(pos, obj); err != nil {
				return errors.Errorf("object '%v' error, %v", reflect.ValueOf(item).Type(), err)
			}
		default:
			return errors.Errorf("unknown object type '%v' on position '%s'", reflect.ValueOf(item).Type(), pos)
		}
	}
	return nil
}

// Helper function to check if an object is a pointer or interface that can be tracked
func isPointer(obj any) bool {
	if obj == nil {
		return false
	}

	kind := reflect.ValueOf(obj).Kind()
	return kind == reflect.Ptr ||
		kind == reflect.Map ||
		kind == reflect.Chan ||
		kind == reflect.Func ||
		kind == reflect.UnsafePointer
}

func (t *container) Core() []reflect.Type {
	var list []reflect.Type
	for typ := range t.core {
		list = append(list, typ)
	}
	return list
}

func (t *container) Bean(typ reflect.Type, level int) []Bean {
	var beanList []Bean
	candidates := t.getBean(typ)
	if len(candidates) > 0 {
		list := orderBeans(levelBeans(candidates, level))
		for _, b := range list {
			beanList = append(beanList, b)
		}
	}
	return beanList
}

func (t *container) Lookup(name string, level int) []Bean {
	var beanList []Bean
	candidates := t.searchByNameRecursive(name)
	if len(candidates) > 0 {
		list := orderBeans(levelBeans(candidates, level))
		for _, b := range list {
			beanList = append(beanList, b)
		}
	}
	return beanList
}

func (t *container) Inject(obj any) error {
	if obj == nil {
		return errors.New("null obj is are not allowed")
	}
	classPtr := reflect.TypeOf(obj)
	if classPtr.Kind() != reflect.Ptr {
		return errors.Errorf("non-pointer instances are not allowed, type %v", classPtr)
	}
	bd, err := cachedBeanDef(classPtr)
	if err != nil {
		return err
	}
	valuePtr := reflect.ValueOf(obj)
	value := valuePtr.Elem()
	for _, inject := range bd.fields {
		impl := t.getBean(inject.fieldType)
		if len(impl) == 0 {
			if inject.optional {
				continue
			}
			return errors.Errorf("implementation not found for field '%s' with type '%v'", inject.fieldName, inject.fieldType)
		}
		if err := inject.inject(&value, impl); err != nil {
			return err
		}
	}
	for _, inject := range bd.properties {
		if err := inject.inject(&value, t.properties); err != nil {
			return err
		}
	}
	return nil
}

// multi-threading safe, runtime lookup
func (t *container) getBean(ifaceType reflect.Type) []beanlist {

	switch ifaceType.Kind() {
	case reflect.Ptr:
		// immutable
		return t.findObjectRecursive(ifaceType)

	case reflect.Interface:
		// mutable since some new interfaces unknown on container creation could be added later, therefore needed cache and multi-threading support
		return t.searchAndCacheInterfaceCandidatesRecursive(ifaceType)

	default:
		return nil
	}
}

func getStackInfo(stack []*bean, delim string) string {
	var out strings.Builder
	n := len(stack)
	for i := 0; i < n; i++ {
		if i > 0 {
			out.WriteString(delim)
		}
		out.WriteString(stack[i].beanDef.classPtr.String())
	}
	return out.String()
}

func reverseStack(stack []*bean) []*bean {
	var out []*bean
	n := len(stack)
	for j := n - 1; j >= 0; j-- {
		out = append(out, stack[j])
	}
	return out
}

func (t *container) constructBeanList(ctx context.Context, list []*bean, stack []*bean) error {
	for _, bean := range list {
		if err := t.constructBean(ctx, bean, stack); err != nil {
			return err
		}
	}
	return nil
}

func indent(n int) string {
	if n == 0 {
		return ""
	}
	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, ' ', ' ')
	}
	return string(out)
}

func (t *container) constructBean(ctx context.Context, bean *bean, stack []*bean) (err error) {

	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			err = errors.Errorf("construct bean '%s' with type '%v' recovered with error %v, stacktrace: %s", bean.name, bean.beanDef.classPtr, r, stack)
		}
	}()

	if bean.lifecycle == BeanInitialized {
		return nil
	}

	_, isFactoryBean := bean.obj.(FactoryBean)
	initializerWithContext, hasConstructorWithContext := bean.obj.(ContextInitializingBean)
	initializer, hasConstructor := bean.obj.(InitializingBean)
	if t.logger.Enabled() {
		t.logger.Printf("%sConstruct Bean '%s' with type '%v', isFactoryBean=%v, hasFactory=%v, hasObject=%v, hasConstructor=%v\n", indent(len(stack)), bean.name, bean.beanDef.classPtr, isFactoryBean, bean.beenFactory != nil, bean.obj != nil, hasConstructor)
	}

	if bean.lifecycle == BeanConstructing {
		for i, b := range stack {
			if b == bean {
				// cycle dependency detected
				return errors.Errorf("detected cycle dependency %s", getStackInfo(append(stack[i:], bean), "->"))
			}
		}
	}
	bean.lifecycle = BeanConstructing
	bean.ctorMu.Lock()
	defer func() {
		bean.ctorMu.Unlock()
	}()

	for _, factoryDep := range bean.factoryDependencies {
		if err := t.constructBean(ctx, factoryDep.factory.bean, append(stack, bean)); err != nil {
			return err
		}
		if t.logger.Enabled() {
			t.logger.Printf("%sFactoryDep (%v).Object()\n", indent(len(stack)+1), factoryDep.factory.factoryClassPtr)
		}
		bean, created, err := factoryDep.factory.ctor(ctx)
		if err != nil {
			return errors.Errorf("factory ctor '%v' failed, %v", factoryDep.factory.factoryClassPtr, err)
		}
		if created {
			if t.logger.Enabled() {
				t.logger.Printf("%sDep Created Bean %s with type '%v' singleton=%v\n", indent(len(stack)+1), bean.name, bean.beanDef.classPtr, factoryDep.factory.singleton())
			}
		}
		err = factoryDep.injection(bean)
		if err != nil {
			return errors.Errorf("factory injection '%v' failed, %v", factoryDep.factory.factoryClassPtr, err)
		}
	}

	// construct bean dependencies
	if err := t.constructBeanList(ctx, bean.dependencies, append(stack, bean)); err != nil {
		return err
	}

	// check if it is empty element bean
	if bean.beenFactory != nil && bean.obj == nil {
		if err := t.constructBean(ctx, bean.beenFactory.bean, append(stack, bean)); err != nil {
			return err
		}
		if t.logger.Enabled() {
			t.logger.Printf("%s(%v).Object()\n", indent(len(stack)), bean.beenFactory.factoryClassPtr)
		}
		_, _, err := bean.beenFactory.ctor(ctx) // always new
		if err != nil {
			return errors.Errorf("factory ctor '%v' failed, %v", bean.beenFactory.factoryClassPtr, err)
		}
		if bean.obj == nil {
			return errors.Errorf("bean '%v' was not created by factory ctor '%v'", bean, bean.beenFactory.factoryClassPtr)
		}
		return nil
	}

	// inject properties
	if len(bean.beanDef.properties) > 0 {
		value := bean.valuePtr.Elem()
		for _, propertyDef := range bean.beanDef.properties {
			if t.logger.Enabled() {
				if propertyDef.defaultValue != "" {
					t.logger.Printf("%sProperty '%s' default '%s'\n", indent(len(stack)+1), propertyDef.propertyName, propertyDef.defaultValue)
				} else {
					t.logger.Printf("%sProperty '%s'\n", indent(len(stack)+1), propertyDef.propertyName)
				}
			}
			err = propertyDef.inject(&value, t.properties)
			if err != nil {
				return errors.Errorf("property '%s' injection in bean '%s' failed, %s, %v", propertyDef.propertyName, bean.name, getStackInfo(reverseStack(append(stack, bean)), " required by "), err)
			}
		}
	}

	if hasConstructorWithContext || hasConstructor {
		if t.logger.Enabled() {
			t.logger.Printf("%sPostConstruct Bean '%s' with type '%v'\n", indent(len(stack)), bean.name, bean.beanDef.classPtr)
		}
		if hasConstructorWithContext {
			if err := initializerWithContext.PostConstruct(ctx); err != nil {
				return errors.Errorf("post construct failed %s, %v", getStackInfo(reverseStack(append(stack, bean)), " required by "), err)
			}
		} else {
			if err := initializer.PostConstruct(); err != nil {
				return errors.Errorf("post construct failed %s, %v", getStackInfo(reverseStack(append(stack, bean)), " required by "), err)
			}
		}
	}

	if bean.beenFactory == nil {
		// add disposable only for managed beans, not produced. Spring Framework pattern.
		t.addDisposable(bean)
	}

	bean.lifecycle = BeanInitialized
	return nil
}

func (t *container) addDisposable(bean *bean) {
	if _, ok := bean.obj.(ContextDisposableBean); ok {
		t.disposables = append(t.disposables, bean)
	} else if _, ok := bean.obj.(DisposableBean); ok {
		t.disposables = append(t.disposables, bean)
	}
}

func (t *container) postConstruct(ctx context.Context, lists ...[]*bean) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("post construct recover on error, %v\n", r)
		}
	}()

	for _, list := range lists {
		if err = t.constructBeanList(ctx, list, nil); err != nil {
			return err
		}
	}

	return nil
}

// Close - destroy in reverse initialization order
func (t *container) Close() (err error) {
	return t.CloseWithContext(context.Background())
}

// CloseWithContext - destroy in reverse initialization order with context
func (t *container) CloseWithContext(ctx context.Context) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("container close recover error: %v", r)
		}
	}()

	var listErr []error
	t.closeOnce.Do(func() {

		for _, child := range t.children {
			if err := child.CloseWithContext(ctx); err != nil {
				listErr = append(listErr, err)
			}
		}

		n := len(t.disposables)
		for j := n - 1; j >= 0; j-- {
			if err := t.destroyBean(ctx, t.disposables[j]); err != nil {
				listErr = append(listErr, err)
			}
		}
	})

	return multipleErr(listErr)
}

func (t *container) destroyBean(ctx context.Context, b *bean) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("destroy bean '%s' with type '%v' recovered with error: %v", b.name, b.beanDef.classPtr, r)
		}
	}()

	if b.lifecycle != BeanInitialized {
		return nil
	}

	b.lifecycle = BeanDestroying
	t.logger.Printf("Destroying bean '%s' with type '%v'\n", b.name, b.beanDef.classPtr)
	if dis, ok := b.obj.(ContextDisposableBean); ok {
		if e := dis.Destroy(ctx); e != nil {
			err = e
		} else {
			b.lifecycle = BeanDestroyed
		}
	} else if dis, ok := b.obj.(DisposableBean); ok {
		if e := dis.Destroy(); e != nil {
			err = e
		} else {
			b.lifecycle = BeanDestroyed
		}
	}
	return
}

func (t *container) Reload(b Bean) error {
	return t.ReloadWithContext(context.Background(), b)
}

func (t *container) ReloadWithContext(ctx context.Context, b Bean) error {
	bb, ok := b.(*bean)
	if !ok {
		return errors.Errorf("unsupported bean type %T", b)
	}

	bb.ctorMu.Lock()
	defer bb.ctorMu.Unlock()

	if bb.beenFactory != nil {
		return errors.Errorf("bean '%s' was created by factory bean '%v' and can not be reloaded", bb.name, bb.beenFactory.factoryClassPtr)
	}

	// destroy
	bb.lifecycle = BeanDestroying
	if dis, ok := bb.obj.(ContextDisposableBean); ok {
		if err := dis.Destroy(ctx); err != nil {
			return err
		}
	} else if dis, ok := bb.obj.(DisposableBean); ok {
		if err := dis.Destroy(); err != nil {
			return err
		}
	}

	// re-resolve static value: properties (skip dynamic — they already read live values)
	bb.lifecycle = BeanConstructing
	if len(bb.beanDef.properties) > 0 {
		value := bb.valuePtr.Elem()
		for _, propDef := range bb.beanDef.properties {
			if propDef.dynamic {
				continue
			}
			if err := propDef.inject(&value, t.properties); err != nil {
				return errors.Errorf("reload property '%s' in bean '%s' failed, %v", propDef.propertyName, bb.name, err)
			}
		}
	}

	// post-construct
	if init, ok := bb.obj.(ContextInitializingBean); ok {
		if err := init.PostConstruct(ctx); err != nil {
			return err
		}
	} else if init, ok := bb.obj.(InitializingBean); ok {
		if err := init.PostConstruct(); err != nil {
			return err
		}
	}

	bb.lifecycle = BeanInitialized
	return nil
}

func multipleErr(err []error) error {
	switch len(err) {
	case 0:
		return nil
	case 1:
		return err[0]
	default:
		return errors.Errorf("multiple errors, %v", err)
	}
}

func (t *container) Resource(path string) (Resource, bool) {
	idx := strings.IndexByte(path, ':')
	if idx == -1 {
		return nil, false
	}
	source := path[:idx]
	name := path[idx+1:]

	current := t
	for current != nil {
		resource, ok := current.resourceSources.findResource(source, name)
		if ok {
			return resource, ok
		}
		current = current.parent
	}
	return nil, false
}

func (t *container) Properties() Properties {
	return t.properties
}

func (t *container) String() string {
	return fmt.Sprintf("Container [hasParent=%v, types=%d, destructors=%d]", t.parent != nil, len(t.core), len(t.disposables))
}

type childContext struct {
	name string
	scan []any

	Parent Container `inject:""`

	extendOnes sync.Once
	ctx        Container
	err        error

	closeOnes sync.Once
}

/**
Defines ctx container inside parent container
*/

func Child(name string, scan ...any) ChildContainer {
	return &childContext{name: name, scan: scan}
}

func (t *childContext) ChildName() string {
	return t.name
}

func (t *childContext) Object() (ctx Container, err error) {
	return t.ObjectWithContext(context.Background())
}

func (t *childContext) ObjectWithContext(ctx context.Context) (ctn Container, err error) {
	t.extendOnes.Do(func() {
		t.ctx, t.err = t.Parent.ExtendWithContext(ctx, t.scan...)
	})
	return t.ctx, t.err
}

func (t *childContext) Close() (err error) {
	t.closeOnes.Do(func() {
		if t.ctx != nil {
			err = t.ctx.Close()
		}
	})
	return
}

func (t *childContext) CloseWithContext(ctx context.Context) (err error) {
	t.closeOnes.Do(func() {
		if t.ctx != nil {
			err = t.ctx.CloseWithContext(ctx)
		}
	})
	return
}

func (t *childContext) String() string {
	return fmt.Sprintf("ChildContainer [created=%v, name=%s, beans=%d]", t.ctx != nil, t.name, len(t.scan))
}

func (t *container) Children() []ChildContainer {
	return t.children
}
