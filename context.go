/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"fmt"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var DefaultCloseTimeout = time.Minute

type context struct {
	
	/**
	Parent context if exist
	*/
	parent *context

	/**
	Recognized ctx context list
	 */
	children []ChildContext

	/**
		All instances scanned during creation of context.
	    No modifications on runtime allowed.
	*/
	core map[reflect.Type][]*bean

	/**
	List of beans in initialization order that should depose on close
	*/
	disposables []*bean

	/**
	Fast search of beans by faceType and name
	*/
	registry registry

	/**
	Placeholder properties of the context
	 */
	properties Properties

	/**
	Cache bean descriptions for Inject calls in runtime
	*/
	runtimeCache sync.Map // key is reflect.Type (classPtr), value is *beanDef

	/**
	Guarantees that context would be closed once
	*/
	closeOnce sync.Once
}

func New(scan ...interface{}) (Context, error) {
	return createContext(nil, scan)
}

func (t *context) Extend(scan ...interface{}) (Context, error) {
	return createContext(t, scan)
}

func (t *context) Parent() (Context, bool) {
	if t.parent != nil {
		return t.parent, true
	} else {
		return nil, false
	}
}

func createContext(parent *context, scan []interface{}) (ctx *context, err error) {

	prev := runtime.GOMAXPROCS(1)
	defer func() {
		runtime.GOMAXPROCS(prev)
	}()

	core := make(map[reflect.Type][]*bean)
	pointers := make(map[reflect.Type][]*injection)
	interfaces := make(map[reflect.Type][]*injection)
	var propertySources []*PropertySource
	var propertyResolvers []PropertyResolver
	var primaryList []*bean
	var secondaryList []*bean

	ctx = &context{
		parent: parent,
		core:   core,
		registry: registry{
			beansByName: make(map[string][]*bean),
			beansByType: make(map[reflect.Type][]*bean),
			resourceSources: make(map[string]*resourceSource),
		},
		properties: NewProperties(),
	}

	if parent != nil {
		ctx.properties.Extend(parent.properties)
	}

	// add context bean to registry
	ctxBean := &bean{
		obj:      ctx,
		valuePtr: reflect.ValueOf(ctx),
		beanDef: &beanDef{
			classPtr: reflect.TypeOf(ctx),
		},
		lifecycle: BeanInitialized,
	}
	core[ctxBean.beanDef.classPtr] = []*bean {ctxBean}

	// add properties bean to registry
	propertiesBean := &bean{
		obj:      ctx,
		valuePtr: reflect.ValueOf(ctx.properties),
		beanDef: &beanDef{
			classPtr: reflect.TypeOf(ctx.properties),
		},
		lifecycle: BeanInitialized,
	}
	core[propertiesBean.beanDef.classPtr] = []*bean {propertiesBean}

	// scan
	err = forEach("", scan, func(pos string, obj interface{}) (err error) {

		var resolver bool

		switch instance := obj.(type) {
		case ChildContext:
			if verbose != nil {
				verbose.Printf("ChildContext %s\n", instance.Role())
			}
			ctx.children = append(ctx.children, instance)
			// register interest by making a placeholder
			if _, ok := interfaces[ChildContextClass]; !ok {
				interfaces[ChildContextClass] = []*injection{}
			}
		case ResourceSource:
			if verbose != nil {
				verbose.Printf("ResourceSource %s, assets %+v\n", instance.Name, instance.AssetNames)
			}
			if err := ctx.registry.addResourceSource(&instance); err != nil {
				return err
			}
			obj = &instance
		case *ResourceSource:
			if verbose != nil {
				verbose.Printf("ResourceSource %s, assets %+v\n", instance.Name, instance.AssetNames)
			}
			if err := ctx.registry.addResourceSource(instance); err != nil {
				return err
			}
		case PropertySource:
			if verbose != nil {
				verbose.Printf("PropertySource %s %d\n", instance.Path, len(instance.Map))
			}
			ptr := &instance
			propertySources = append(propertySources, ptr)
			obj = ptr
		case *PropertySource:
			if verbose != nil {
				verbose.Printf("PropertySource %s %d\n", instance.Path, len(instance.Map))
			}
			propertySources = append(propertySources, instance)
		case PropertyResolver:
			if verbose != nil {
				verbose.Printf("PropertyResolver Priority %d\n", instance.Priority())
			}
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
			if isFactoryBean {
				elemClassPtr = factoryBean.ObjectType()
			}

			if verbose != nil {
				if isFactoryBean {
					var info string
					if factoryBean.Singleton() {
						info = "singleton"
					} else {
						info = "non-singleton"
					}
					objectName := factoryBean.ObjectName()
					if objectName != "" {
						verbose.Printf("FactoryBean %v produce %s %v with name '%s'\n", classPtr, info, elemClassPtr, objectName)
					} else {
						verbose.Printf("FactoryBean %v produce %s %v\n", classPtr, info, elemClassPtr)
					}
				} else {
					if objBean.qualifier != "" {
						verbose.Printf("Bean %v with name '%s'\n", classPtr, objBean.qualifier)
					} else {
						verbose.Printf("Bean %v\n", classPtr)
					}
				}
			}

			if isFactoryBean {
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
					if verbose != nil {
						var attr []string
						if injectDef.lazy {
							attr = append(attr,  "lazy")
						}
						if injectDef.optional {
							attr = append(attr,  "optional")
						}
						if injectDef.qualifier != "" {
							attr = append(attr,  "bean=" + injectDef.qualifier)
						}
						var attrs string
						if len(attr) > 0 {
							attrs = fmt.Sprintf("[%s]", strings.Join(attr, ","))
						}
						var prefix string
						if injectDef.slice {
							prefix = "[]"
						}
						if injectDef.table {
							prefix = "map[string]"
						}
						verbose.Printf("	Field %s%v %s\n", prefix, injectDef.fieldType, attrs)
					}

					switch injectDef.fieldType.Kind() {
					case reflect.Ptr:
						pointers[injectDef.fieldType] = append(pointers[injectDef.fieldType], &injection{objBean, value, injectDef})
					case reflect.Interface:
						interfaces[injectDef.fieldType] = append(interfaces[injectDef.fieldType], &injection{objBean, value, injectDef})
					case reflect.Func:
						pointers[injectDef.fieldType] = append(pointers[injectDef.fieldType], &injection{objBean, value, injectDef})
					default:
						return errors.Errorf("injecting not a pointer or interface on field type '%v' at position '%s' in %v", injectDef.fieldType, pos, classPtr)
					}
				}
			}

			/*
				Register factory if needed
			*/
			if isFactoryBean {
				f := &factory{
					bean:            objBean,
					factoryObj:      obj,
					factoryClassPtr: classPtr,
					factoryBean:     factoryBean,
				}
				objectName := factoryBean.ObjectName()
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
				f.instances = []*bean {elemBean}
				// we can have singleton or multiple beans in context produced by this factory, let's allocate reference for injections even if those beans are still not exist
				registerBean(core, elemClassPtr, elemBean)
				secondaryList = append(secondaryList, elemBean)
			}

			/*
				Register bean itself
			*/
			registerBean(core, classPtr, objBean)

			/**
				Initialize property resolver beans at first
			 */
			if resolver {
				primaryList = append(primaryList, objBean)
			} else {
				secondaryList = append(secondaryList, objBean)
			}

		case reflect.Func:

			if verbose != nil {
				verbose.Printf("Function %v\n", classPtr)
			}

			/*
				Register function in context
			*/
			objBean := &bean{
				name:     classPtr.String(),
				obj:      obj,
				valuePtr: reflect.ValueOf(obj),
				beanDef: &beanDef{
					classPtr: classPtr,
				},
				lifecycle: BeanInitialized,
			}

			registerBean(core, classPtr, objBean)

		default:
			return errors.Errorf("instance could be a pointer or function, but was '%s' on position '%s' of type '%v'", classPtr.Kind().String(), pos, classPtr)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// direct match
	for requiredType, injects := range pointers {

		if verbose != nil {
			verbose.Println("Object", requiredType, len(injects))
		}

		direct := ctx.findObjectRecursive(requiredType)
		if len(direct) > 0 {

			// register only beans from current context
			if direct[0].level == 1 {
				ctx.registry.addBeanList(requiredType, direct[0].list)
			}

			if verbose != nil {
				verbose.Printf("Inject '%v' by pointer '%+v' in to %+v\n", requiredType, direct, injects)
			}

			for _, inject := range injects {
				if err := inject.inject(direct); err != nil {
					return nil, errors.Errorf("required type '%s' injection error, %v", requiredType, err)
				}
			}

		} else {

			if verbose != nil {
				verbose.Printf("Bean '%v' not found in context\n", requiredType)
			}

			var required []*injection
			for _, inject := range injects {
				if inject.injectionDef.optional {
					if verbose != nil {
						verbose.Printf("Skip optional inject '%v' in to '%v'\n", requiredType, inject)
					}
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

		if verbose != nil {
			verbose.Println("Interface", ifaceType, len(injects))
		}

		candidates := ctx.searchInterfaceCandidatesRecursive(ifaceType)
		if len(candidates) == 0 {

			if verbose != nil {
				verbose.Printf("No found bean candidates for interface '%v' in context\n", ifaceType)
			}

			var required []*injection
			for _, inject := range injects {
				if inject.injectionDef.optional {
					if verbose != nil {
						verbose.Printf("Skip optional inject of interface '%v' in to '%v'\n", ifaceType, inject)
					}
				} else {
					required = append(required, inject)
				}
			}

			if len(required) > 0 {
				return nil, errors.Errorf("can not find candidates for '%v' interface required by '%+v'", ifaceType, required)
			}

			continue
		}

		// register beans that found only in current context
		if candidates[0].level == 1 {
			ctx.registry.addBeanList(ifaceType, candidates[0].list)
		}

		for _, inject := range injects {

			if verbose != nil {
				verbose.Printf("Inject '%v' by implementation '%+v' in to %+v\n", ifaceType, candidates, inject)
			}

			if err := inject.inject(candidates); err != nil {
				return nil, errors.Errorf("interface '%s' injection error, %v", ifaceType, err)
			}

		}

	}

	/**
	Load properties from property sources
	 */
	if len(propertySources) > 0 {
		if err := ctx.loadProperties(propertySources); err != nil {
			return nil, err
		}
	}

	/**
	Register property resolvers from context
	 */
	for _, r := range propertyResolvers {
		ctx.properties.Register(r)
	}

	/**
	PostConstruct beans
	 */
	if err := ctx.postConstruct(primaryList, secondaryList); err != nil {
		ctx.closeWithTimeout(DefaultCloseTimeout)
		return nil, err
	} else {
		return ctx, nil
	}

}

func (t *context) closeWithTimeout(timeout time.Duration) {
	ch := make(chan error)
	go func() {
		ch <- t.Close()
		close(ch)
	}()
	select {
	case e := <- ch:
		if e != nil && verbose != nil {
			verbose.Printf("Close context error, %v\n", e)
		}
	case <- time.After(timeout):
		if verbose != nil {
			verbose.Printf("Close context timeout error.\n")
		}
	}
}

func (t *context) loadProperties(propertySources []*PropertySource) error {

	for _, source := range propertySources {

		if source.Path != "" {

			if resource, ok := t.Resource(source.Path); ok {

				file, err := resource.Open()
				if err != nil {
					return errors.Errorf("i/o error with placeholder properties resource '%s', %v", source, err)
				}

				if isYamlFile(source.Path) {

					holder := make(map[string]interface{})
					err = yaml.NewDecoder(file).Decode(holder)
					if err == nil {
						t.properties.LoadMap(holder)
					}

				} else {
					err = t.properties.Load(file)
				}

				file.Close()
				if err != nil {
					return errors.Errorf("load error of placeholder properties resource '%s', %v", source, err)
				}

			} else {
				return errors.Errorf("placeholder properties resource '%s' is not found", source)
			}
		}

		if source.Map != nil {
			t.properties.LoadMap(source.Map)
		}

	}

	return nil
}

func isYamlFile(fileName string) bool {
	return strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml")
}

func (t *context) findObjectRecursive(requiredType reflect.Type) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		if direct, ok := ctx.core[requiredType]; ok {
			candidates = append(candidates, beanlist{level: level, list: direct})
		}
		level++
	}
	return candidates
}

func (t *context) searchAndCacheObjectRecursive(requiredType reflect.Type) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {

		// first lookup in the registry
		if list, ok := ctx.registry.findByType(requiredType); !ok {
			list = ctx.core[requiredType]
			if len(list) > 0 {
				candidates = append(candidates, beanlist{level: level, list: list})
			}
			// store in cache, even an empty list, so next time we would not come here
			ctx.registry.addBeanList(requiredType, list)

		} else if len(list) > 0 {
			candidates = append(candidates, beanlist{level: level, list: list})
		}

		level++
	}
	return candidates
}

func registerBean(registry map[reflect.Type][]*bean, classPtr reflect.Type, bean *bean) {
	registry[classPtr] = append(registry[classPtr], bean)
}

func forEach(initialPos string, scan []interface{}, cb func(i string, obj interface{}) error) error {
	for j, item := range scan {
		var pos string
		if len(initialPos) > 0 {
			pos = fmt.Sprintf("%s.%d", initialPos, j)
		} else {
			pos = strconv.Itoa(j)
		}
		if item == nil {
			continue
		}
		switch obj := item.(type) {
		case Scanner:
			if err := forEach(pos, obj.Beans(), cb); err != nil {
				return err
			}
		case []interface{}:
			if err := forEach(pos, obj, cb); err != nil {
				return err
			}
		case interface{}:
			if err := cb(pos, obj); err != nil {
				return errors.Errorf("object '%v' error, %v", reflect.ValueOf(item).Type(), err)
			}
		default:
			return errors.Errorf("unknown object type '%v' on position '%s'", reflect.ValueOf(item).Type(), pos)
		}
	}
	return nil
}

func (t *context) Core() []reflect.Type {
	var list []reflect.Type
	for typ := range t.core {
		list = append(list, typ)
	}
	return list
}

func (t *context) Bean(typ reflect.Type, level int) []Bean {
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

func (t *context) Lookup(iface string, level int) []Bean {
	var beanList []Bean
	candidates := t.searchByNameInRepositoryRecursive(iface)
	if len(candidates) > 0 {
		list := orderBeans(levelBeans(candidates, level))
		for _, b := range list {
			beanList = append(beanList, b)
		}
	}
	return beanList
}

func (t *context) Inject(obj interface{}) error {
	if obj == nil {
		return errors.New("null obj is are not allowed")
	}
	classPtr := reflect.TypeOf(obj)
	if classPtr.Kind() != reflect.Ptr {
		return errors.Errorf("non-pointer instances are not allowed, type %v", classPtr)
	}
	valuePtr := reflect.ValueOf(obj)
	value := valuePtr.Elem()
	if bd, err := t.cache(obj, classPtr); err != nil {
		return err
	} else {
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
	}
	return nil
}

// multi-threading safe
func (t *context) getBean(ifaceType reflect.Type) []beanlist {

	switch ifaceType.Kind() {
	case reflect.Ptr, reflect.Func:
		return t.searchAndCacheObjectRecursive(ifaceType)

	case reflect.Interface:
		return t.searchAndCacheInterfaceCandidatesRecursive(ifaceType)

	default:
		return nil
	}
}

func (t *context) searchByNameInRepositoryRecursive(iface string) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		if list, ok := ctx.registry.findByName(iface); ok {
			candidates = append(candidates, beanlist{level: level, list: list})
		}
		level++
	}
	return candidates
}

// multi-threading safe
func (t *context) cache(obj interface{}, classPtr reflect.Type) (*beanDef, error) {
	if bd, ok := t.runtimeCache.Load(classPtr); ok {
		return bd.(*beanDef), nil
	} else {
		b, err := investigate(obj, classPtr)
		if err != nil {
			return nil, err
		}
		t.runtimeCache.Store(classPtr, b.beanDef)
		return b.beanDef, nil
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

func (t *context) constructBeanList(list []*bean, stack []*bean) error {
	for _, bean := range list {
		if err := t.constructBean(bean, stack); err != nil {
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

func (t *context) constructBean(bean *bean, stack []*bean) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("construct bean '%s' with type '%v' recovered with error %v", bean.name, bean.beanDef.classPtr, r)
		}
	}()

	if bean.lifecycle == BeanInitialized {
		return nil
	}

	_, isFactoryBean := bean.obj.(FactoryBean)
	initializer, hasConstructor := bean.obj.(InitializingBean)
	if verbose != nil {
		verbose.Printf("%sConstruct Bean '%s' with type '%v', isFactoryBean=%v, hasFactory=%v, hasObject=%v, hasConstructor=%v\n", indent(len(stack)), bean.name, bean.beanDef.classPtr, isFactoryBean, bean.beenFactory != nil, bean.obj != nil, hasConstructor)
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
		if err := t.constructBean(factoryDep.factory.bean, append(stack, bean)); err != nil {
			return err
		}
		if verbose != nil {
			verbose.Printf("%sFactoryDep (%v).Object()\n", indent(len(stack)+1), factoryDep.factory.factoryClassPtr)
		}
		bean, created, err := factoryDep.factory.ctor()
		if err != nil {
			return errors.Errorf("factory ctor '%v' failed, %v", factoryDep.factory.factoryClassPtr, err)
		}
		if created {
			if verbose != nil {
				verbose.Printf("%sDep Created Bean %s with type '%v'\n", indent(len(stack)+1), bean.name, bean.beanDef.classPtr)
			}
			t.registry.addBean(factoryDep.factory.factoryBean.ObjectType(), bean)
		}
		err = factoryDep.injection(bean)
		if err != nil {
			return errors.Errorf("factory injection '%v' failed, %v", factoryDep.factory.factoryClassPtr, err)
		}
	}

	// construct bean dependencies
	if err := t.constructBeanList(bean.dependencies, append(stack, bean)); err != nil {
		return err
	}

	// check if it is empty element bean
	if bean.beenFactory != nil && bean.obj == nil {
		if err := t.constructBean(bean.beenFactory.bean, append(stack, bean)); err != nil {
			return err
		}
		if verbose != nil {
			verbose.Printf("%s(%v).Object()\n", indent(len(stack)), bean.beenFactory.factoryClassPtr)
		}
		_, _, err := bean.beenFactory.ctor() // always new
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
			if verbose != nil {
				if propertyDef.defaultValue != "" {
					verbose.Printf("%sProperty '%s' default '%s'\n", indent(len(stack)+1), propertyDef.propertyName, propertyDef.defaultValue)
				} else {
					verbose.Printf("%sProperty '%s'\n", indent(len(stack)+1), propertyDef.propertyName)
				}
			}
			err = propertyDef.inject(&value, t.properties)
			if err != nil {
				return errors.Errorf("property '%s' injection in bean '%s' failed, %s, %v", propertyDef.propertyName, bean.name, getStackInfo(reverseStack(append(stack, bean)), " required by "), err)
			}
		}
	}

	if hasConstructor {
		if verbose != nil {
			verbose.Printf("%sPostConstruct Bean '%s' with type '%v'\n", indent(len(stack)), bean.name, bean.beanDef.classPtr)
		}
		if err := initializer.PostConstruct(); err != nil {
			return errors.Errorf("post construct failed %s, %v", getStackInfo(reverseStack(append(stack, bean)), " required by "), err)
		}
	}

	t.addDisposable(bean)
	bean.lifecycle = BeanInitialized
	return nil
}

func (t *context) addDisposable(bean *bean) {
	if _, ok := bean.obj.(DisposableBean); ok {
		t.disposables = append(t.disposables, bean)
	}
}

func (t *context) postConstruct(lists... []*bean) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("post construct recover on error, %v\n", r)
		}
	}()

	for _, list := range lists {
		if err = t.constructBeanList(list, nil); err != nil {
			return err
		}
	}

	return nil
}

// destroy in reverse initialization order
func (t *context) Close() (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("context close recover error: %v", r)
		}
	}()

	var listErr []error
	t.closeOnce.Do(func() {

		for _, child := range t.children {
			if err := child.Close(); err != nil {
				listErr = append(listErr, err)
			}
		}

		n := len(t.disposables)
		for j := n - 1; j >= 0; j-- {
			if err := t.destroyBean(t.disposables[j]); err != nil {
				listErr = append(listErr, err)
			}
		}
	})

	return multipleErr(listErr)
}

func (t *context) destroyBean(b *bean) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("destroy bean '%s' with type '%v' recovered with error: %v", b.name, b.beanDef.classPtr, r)
		}
	}()

	if b.lifecycle != BeanInitialized {
		return nil
	}

	b.lifecycle = BeanDestroying
	if verbose != nil {
		verbose.Printf("Destroy bean '%s' with type '%v'\n", b.name, b.beanDef.classPtr)
	}
	if dis, ok := b.obj.(DisposableBean); ok {
		if e := dis.Destroy(); e != nil {
			err = e
		} else {
			b.lifecycle = BeanDestroyed
		}
	}
	return
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

var errNotFoundInterface = errors.New("not found")

func (t *context) searchInterfaceCandidatesRecursive(ifaceType reflect.Type) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		list := ctx.searchInterfaceCandidates(ifaceType)
		if len(list) > 0 {
			candidates = append(candidates, beanlist{ level: level, list: list })
		}
		level++
	}
	return candidates
}

func (t *context) searchAndCacheInterfaceCandidatesRecursive(ifaceType reflect.Type) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		// first lookup in the registry
		if list, ok := ctx.registry.findByType(ifaceType); !ok {
			list = ctx.searchInterfaceCandidates(ifaceType)
			if len(list) > 0 {
				candidates = append(candidates, beanlist{ level: level, list: list })
			}
			// cache in registry
			// even empty list, so we would not come here again
			ctx.registry.addBeanList(ifaceType, list)
		} else if len(list) > 0 {
			candidates = append(candidates, beanlist{ level: level, list: list })
		}
		level++
	}
	return candidates
}

func (t *context) searchInterfaceCandidates(ifaceType reflect.Type) []*bean {
	var candidates []*bean
	for _, list := range t.core {
		if len(list) > 0 && list[0].beanDef.implements(ifaceType) {
			candidates = append(candidates, list...)
		}
	}
	return candidates
}

func (t *context) Resource(path string) (Resource, bool) {
	idx := strings.IndexByte(path, ':')
	if idx == -1 {
		return nil, false
	}
	source := path[:idx]
	name := path[idx+1:]

	current := t
	for current != nil {
		resource, ok := current.registry.findResource(source, name)
		if ok {
			return resource, ok
		}
		current = current.parent
	}
	return nil, false
}

func (t *context) Properties() Properties {
	return t.properties
}

func (t *context) String() string {
	return fmt.Sprintf("Context [hasParent=%v, types=%d, destructors=%d]", t.parent != nil, len(t.core), len(t.disposables))
}

type childContext struct {
	role  string
	scan  []interface{}

	Parent  Context  `inject`

	extendOnes  sync.Once
	ctx         Context
	err         error

	closeOnes   sync.Once
}

/**
Defines ctx context inside parent context
 */

func Child(role string, scan... interface{}) ChildContext {
	return &childContext{role: role, scan: scan}
}

func (t *childContext) Role() string {
	return t.role
}

func (t *childContext) Object() (ctx Context, err error) {
	t.extendOnes.Do(func() {
		t.ctx, t.err = t.Parent.Extend(t.scan...)
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


func (t *childContext) String() string {
	return fmt.Sprintf("ChildContext [created=%v, role=%s, beans=%d]", t.ctx != nil, t.role, len(t.scan))
}

func (t *context) Children() []ChildContext {
	return t.children
}
