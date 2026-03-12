/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	DefaultSearchLevel         = SearchFallback
	SearchFallback             = 0  // current, otherwise nearest parent match
	SearchCurrent              = 1  // current only
	SearchCurrentAndParent     = 2  // current + direct parent
	SearchCurrentAndTwoParents = 3  // current + parent + grandparent
	SearchCurrentAndAllParents = -1 // all visible ancestors
)

// stubField describes an anonymous field that needs a stub set on each instance.
type stubField struct {
	fieldIndex int
	fieldType  reflect.Type
}

type beanDef struct {
	/**
	Class of the pointer to the struct or interface
	*/
	classPtr reflect.Type

	/**
	Anonymous fields expose their interfaces though the bean itself.
	This is confusing on injection, because this bean is an encapsulator, not an implementation.

	Skip those fields.
	*/
	anonymousFields []reflect.Type

	/**
	Anonymous fields that require stub initialization on each instance
	*/
	stubs []stubField

	/**
	Fields that are going to be injected
	*/
	fields []*injectionDef

	/**
	Properties that are going to be injected
	*/
	properties []*propInjectionDef
}

// globalBeanDefCache caches parsed beanDef by classPtr.
// Since Go types are immutable, beanDef is safe to share across containers.
var globalBeanDefCache sync.Map // key is reflect.Type (classPtr), value is *beanDef

// cachedBeanDef returns a cached beanDef for the given classPtr, parsing it if needed.
func cachedBeanDef(classPtr reflect.Type) (*beanDef, error) {
	if bd, ok := globalBeanDefCache.Load(classPtr); ok {
		return bd.(*beanDef), nil
	}
	bd, err := parseBeanDef(classPtr)
	if err != nil {
		return nil, err
	}
	actual, _ := globalBeanDefCache.LoadOrStore(classPtr, bd)
	return actual.(*beanDef), nil
}

// applyStubs sets stub values on the instance's anonymous fields.
func (t *beanDef) applyStubs(value reflect.Value) {
	for _, s := range t.stubs {
		var stub any
		switch s.fieldType {
		case NamedBeanClass:
			stub = newNamedBeanStub(t.classPtr.String())
		case OrderedBeanClass:
			stub = newOrderedBeanStub()
		case ProfileBeanClass:
			stub = newProfileBeanStub()
		case ConditionalBeanClass:
			stub = newConditionalBeanStub()
		case ScopedBeanClass:
			stub = newScopedBeanStub()
		case InitializingBeanClass:
			stub = newInitializingBeanStub(t.classPtr.String())
		case ContextInitializingBeanClass:
			stub = newContextInitializingBeanStub(t.classPtr.String())
		case DisposableBeanClass:
			stub = newDisposableBeanStub(t.classPtr.String())
		case ContextDisposableBeanClass:
			stub = newContextDisposableBeanStub(t.classPtr.String())
		case FactoryBeanClass:
			stub = newFactoryBeanStub(t.classPtr.String(), t.classPtr)
		case ContextFactoryBeanClass:
			stub = newContextFactoryBeanStub(t.classPtr.String(), t.classPtr)
		default:
			continue
		}
		value.Field(s.fieldIndex).Set(reflect.ValueOf(stub))
	}
}

type bean struct {
	/**
	Name of the bean
	*/
	name string

	/**
	Qualifier of the bean
	*/
	qualifier string

	/**
	Order of the bean
	*/
	ordered bool
	order   int

	/**
	Primary flag - true if this is the primary bean for its type
	*/
	primary bool

	/**
	Factory of the bean if exist
	*/
	beenFactory *factory

	/**
	Instance to the bean, could be empty if beenFactory exist
	*/
	obj any

	/**
	Reflect instance to the pointer or interface of the bean, could be empty if beenFactory exist
	*/
	valuePtr reflect.Value

	/**
	Bean description
	*/
	beanDef *beanDef

	/**
	Bean lifecycle
	*/
	lifecycle BeanLifecycle

	/**
	List of beans that should initialize before current bean
	*/
	dependencies []*bean

	/**
	List of factory beans that should initialize before current bean
	*/
	factoryDependencies []*factoryDependency

	/**
	Next bean in the list
	*/
	next *bean

	/**
	Constructor mutex for the bean
	*/
	ctorMu sync.Mutex
}

type beanlist struct {
	level int
	list  []*bean
}

func (t beanlist) String() string {
	return fmt.Sprintf("container{level=%d, beans=%v}", t.level, t.list)
}

func (t *bean) String() string {
	pointer := uintptr(unsafe.Pointer(&t.obj))
	if t.beenFactory != nil {
		objectName := t.beenFactory.objectName()
		if objectName != "" {
			return fmt.Sprintf("<FactoryBean %s->%s(%s)>(%x)", t.beenFactory.factoryClassPtr, t.beanDef.classPtr, objectName, pointer)
		} else {
			return fmt.Sprintf("<FactoryBean %s->%s>(%x)", t.beenFactory.factoryClassPtr, t.beanDef.classPtr, pointer)
		}
	} else if t.qualifier != "" {
		return fmt.Sprintf("<Bean %s(%s)>(%x)", t.beanDef.classPtr, t.qualifier, pointer)
	} else {
		return fmt.Sprintf("<Bean %s>(%x)", t.beanDef.classPtr, pointer)
	}
}

func (t *bean) Name() string {
	return t.name
}

func (t *bean) Class() reflect.Type {
	return t.beanDef.classPtr
}

func (t *bean) Implements(ifaceType reflect.Type) bool {
	return t.beanDef.implements(ifaceType)
}

func (t *bean) Object() any {
	return t.obj
}

func (t *bean) FactoryBean() (Bean, bool) {
	if t.beenFactory != nil {
		return t.beenFactory.bean, true
	} else {
		return nil, false
	}
}

func (t *bean) Lifecycle() BeanLifecycle {
	return t.lifecycle
}

/*
*
Check if bean definition can implement interface type
*/
func (t *beanDef) implements(ifaceType reflect.Type) bool {
	if isSomeoneImplements(ifaceType, t.anonymousFields) {
		return false
	}
	return t.classPtr.Implements(ifaceType)
}

type factory struct {
	/**
	Bean associated with Factory in container
	*/
	bean *bean

	/**
	Instance to the factory bean
	*/
	factoryObj any

	/**
	Factory bean type
	*/
	factoryClassPtr reflect.Type

	/**
	Factory bean interface
	*/
	factoryBean FactoryBean

	/**
	Context-aware factory bean interface
	*/
	contextFactoryBean ContextFactoryBean

	/**
	Created bean instances by this factory
	*/
	instances []*bean
}

func (t *factory) String() string {
	return t.factoryClassPtr.String()
}

func (t *factory) objectType() reflect.Type {
	if t.contextFactoryBean != nil {
		return t.contextFactoryBean.ObjectType()
	}
	return t.factoryBean.ObjectType()
}

func (t *factory) objectName() string {
	if t.contextFactoryBean != nil {
		return t.contextFactoryBean.ObjectName()
	}
	return t.factoryBean.ObjectName()
}

func (t *factory) singleton() bool {
	if t.contextFactoryBean != nil {
		return t.contextFactoryBean.Singleton()
	}
	return t.factoryBean.Singleton()
}

func (t *factory) ctor(ctx context.Context) (*bean, bool, error) {
	var b *bean
	var singleton bool

	if len(t.instances) == 0 {
		return nil, false, errors.Errorf("internal: element bean collection is empty for factory '%v'", t.factoryClassPtr)
	}

	if t.singleton() {
		if t.instances[0].obj == nil {
			b = t.instances[0]
			singleton = true
		} else {
			return t.instances[0], false, nil
		}
	} else {
		if t.instances[0].obj == nil {
			b = t.instances[0]
		} else {
			// append next element, since it is not a singleton
			b = &bean{
				name:        t.instances[0].beanDef.classPtr.String(),
				beenFactory: t.instances[0].beenFactory,
				beanDef:     t.instances[0].beanDef,
			}
			t.instances = append(t.instances, b)
		}
	}

	var (
		obj any
		err error
	)
	if contextFactoryBean, ok := t.factoryObj.(ContextFactoryBean); ok {
		obj, err = contextFactoryBean.Object(ctx)
	} else {
		obj, err = t.factoryBean.Object()
	}
	if err != nil {
		return nil, false, errors.Errorf("factory bean '%v' failed to create bean '%v', %v", t.factoryClassPtr, t.objectType(), err)
	}

	b.obj = obj
	b.lifecycle = BeanInitialized
	if namedBean, ok := obj.(NamedBean); ok {
		b.name = namedBean.BeanName()
	}
	b.valuePtr = reflect.ValueOf(obj)

	return b, !singleton, nil
}

type factoryDependency struct {

	/*
		Reference on factory bean used to produce instance
	*/

	factory *factory

	/*
		Injection function where we need to inject produced instance
	*/
	injection func(instance *bean) error
}

/*
*
parseBeanDef parses the type-level metadata from classPtr using reflection.
This is pure type analysis with no instance-specific logic, so the result
can be cached globally and reused across all instances of the same type.
*/
func parseBeanDef(classPtr reflect.Type) (*beanDef, error) {
	var fields []*injectionDef
	var properties []*propInjectionDef
	var anonymousFields []reflect.Type
	var stubs []stubField
	class := classPtr.Elem()
	for j := 0; j < class.NumField(); j++ {
		field := class.Field(j)

		if field.Anonymous {
			anonymousFields = append(anonymousFields, field.Type)
			switch field.Type {
			case NamedBeanClass,
				OrderedBeanClass,
				ProfileBeanClass,
				ConditionalBeanClass,
				ScopedBeanClass,
				InitializingBeanClass,
				ContextInitializingBeanClass,
				DisposableBeanClass,
				ContextDisposableBeanClass,
				FactoryBeanClass,
				ContextFactoryBeanClass:
				stubs = append(stubs, stubField{fieldIndex: j, fieldType: field.Type})
			case ContainerClass:
				return nil, errors.Errorf("exposing by anonymous field '%s' in '%v' interface glue.Container is not allowed", field.Name, classPtr)
			}
		}

		if valueTag, hasValueTag := field.Tag.Lookup("value"); hasValueTag {
			if field.Anonymous {
				return nil, errors.Errorf("injection to anonymous field '%s' in '%v' is not allowed", field.Name, classPtr)
			}
			var propertyName string
			var defaultValue string
			var hasDefaultValue bool
			var timeFormat string
			pairs := strings.Split(valueTag, ",")
			for i, pair := range pairs {
				p := strings.TrimSpace(pair)
				if i == 0 {
					// property name
					propertyName = p
					continue
				}
				kv := strings.SplitN(p, "=", 2)
				switch strings.TrimSpace(kv[0]) {
				case "default":
					if len(kv) > 1 {
						defaultValue = strings.TrimSpace(kv[1])
						hasDefaultValue = true
					}
				case "layout":
					if len(kv) > 1 {
						timeFormat = strings.TrimSpace(kv[1])
					}
				}
			}
			if propertyName == "" {
				return nil, errors.Errorf("empty property name in field '%s' with type '%v' on position %d in %v with 'value' tag", field.Name, field.Type, j, classPtr)
			}
			def := &propInjectionDef{
				class:           class,
				fieldNum:        j,
				fieldName:       field.Name,
				fieldType:       field.Type,
				propertyName:    propertyName,
				defaultValue:    defaultValue,
				hasDefaultValue: hasDefaultValue,
				timeFormat:      timeFormat,
			}
			if field.Type.Kind() == reflect.Func {
				ft := field.Type
				if err := validateDynamicValueFunc(field.Name, classPtr, ft); err != nil {
					return nil, err
				}
				funcReturnsError := ft.NumOut() == 2
				funcTakesContext := ft.NumIn() == 1
				if !funcReturnsError && !funcTakesContext && !hasDefaultValue {
					return nil, errors.Errorf("dynamic value field '%s' in '%v': func() T requires a 'default' option since it cannot return an error", field.Name, classPtr)
				}
				def.dynamic = true
				def.funcTakesContext = funcTakesContext
				def.funcReturnsError = funcReturnsError
				def.funcReturnType = ft.Out(0)
			}
			properties = append(properties, def)
			continue
		}

		injectTag, hasInjectTag := field.Tag.Lookup("inject")
		if field.Tag == "inject" || hasInjectTag {
			if field.Anonymous {
				return nil, errors.Errorf("injection to anonymous field '%s' in '%v' is not allowed", field.Name, classPtr)
			}
			var qualifier string
			var optional bool
			var lazy bool
			var scopeStr string
			level := DefaultSearchLevel
			if hasInjectTag {
				pairs := strings.Split(injectTag, ",")
				for _, pair := range pairs {
					p := strings.TrimSpace(pair)
					kv := strings.SplitN(p, "=", 2)
					switch strings.TrimSpace(kv[0]) {
					case "bean", "qualifier":
						if len(kv) > 1 {
							qualifier = strings.TrimSpace(kv[1])
						}
					case "optional":
						optional = true
					case "lazy":
						lazy = true
					case "level", "search":
						if len(kv) > 1 {
							level, _ = strconv.Atoi(kv[1])
						}
					case "scope":
						if len(kv) > 1 {
							scopeStr = strings.TrimSpace(kv[1])
						}
					default:
						// shorthand: bare name (no "=") treated as qualifier; "-" is the no-op marker
						if len(kv) == 1 && p != "" && p != "-" {
							qualifier = p
						}
					}
				}
			}

			// Parse scope
			var scope BeanScope
			switch scopeStr {
			case "":
				scope = ScopeSingleton
			case "singleton":
				scope = ScopeSingleton
			case "prototype":
				scope = ScopePrototype
			case "request":
				scope = ScopeRequest
			default:
				return nil, errors.Errorf("unknown scope '%s' in field '%s' of '%v'", scopeStr, field.Name, classPtr)
			}

			// Validate scoped injection: must be a function with correct signature
			var scopeProviderTakesContext bool
			var scopeReturnType reflect.Type
			if scope != ScopeSingleton {
				ft := field.Type
				if ft.Kind() != reflect.Func {
					return nil, errors.Errorf("field '%s' in '%v' with scope=%s must be a function type, got %v", field.Name, classPtr, scopeStr, ft)
				}
				if err := validateScopeProviderFunc(field.Name, classPtr, ft, scope); err != nil {
					return nil, err
				}
				scopeProviderTakesContext = ft.NumIn() == 1
				scopeReturnType = ft.Out(0)
			}

			kind := field.Type.Kind()
			fieldType := field.Type
			var fieldSlice, fieldMap bool
			if scope == ScopeSingleton {
				switch kind {
				case reflect.Slice:
					fieldSlice = true
					fieldType = field.Type.Elem()
					kind = fieldType.Kind()
				case reflect.Map:
					fieldMap = true
					if field.Type.Key().Kind() != reflect.String {
						return nil, errors.Errorf("map must have string key to be injected for field type '%v' on position %d in %v with 'inject' tag", field.Type, j, classPtr)
					}
					fieldType = field.Type.Elem()
					kind = fieldType.Kind()
				}
				if kind != reflect.Ptr && kind != reflect.Interface {
					return nil, errors.Errorf("not a pointer or interface field type '%v' on position %d in %v with 'inject' tag", field.Type, j, classPtr)
				}
			} else {
				// scoped providers must be functions; signature checked earlier
				if kind != reflect.Func {
					return nil, errors.Errorf("scoped field '%s' in '%v' must be a function, got '%v'", field.Name, classPtr, field.Type)
				}
			}
			def := &injectionDef{
				class:                     class,
				fieldNum:                  j,
				fieldName:                 field.Name,
				fieldType:                 fieldType,
				lazy:                      lazy,
				isSlice:                   fieldSlice,
				isMap:                     fieldMap,
				optional:                  optional,
				qualifier:                 qualifier,
				level:                     level,
				scope:                     scope,
				scopeProviderTakesContext: scopeProviderTakesContext,
				scopeReturnType:           scopeReturnType,
			}
			fields = append(fields, def)
		}
	}
	return &beanDef{
		classPtr:        classPtr,
		anonymousFields: anonymousFields,
		stubs:           stubs,
		fields:          fields,
		properties:      properties,
	}, nil
}

/*
*
Investigate bean by using cached type-level metadata and instance-specific attributes.
*/
func investigate(obj any, classPtr reflect.Type) (*bean, error) {
	bd, err := cachedBeanDef(classPtr)
	if err != nil {
		return nil, err
	}

	valuePtr := reflect.ValueOf(obj)
	value := valuePtr.Elem()
	bd.applyStubs(value)

	name := classPtr.String()
	var qualifier string
	if namedBean, ok := obj.(NamedBean); ok {
		name = namedBean.BeanName()
		qualifier = name
	}
	ordered := false
	var order int
	if orderedBean, ok := obj.(OrderedBean); ok {
		ordered = true
		order = orderedBean.BeanOrder()
	}
	primary := false
	if primaryBean, ok := obj.(PrimaryBean); ok {
		primary = primaryBean.IsPrimaryBean()
	}
	return &bean{
		name:      name,
		qualifier: qualifier,
		ordered:   ordered,
		order:     order,
		primary:   primary,
		obj:       obj,
		valuePtr:  valuePtr,
		beanDef:   bd,
		lifecycle: BeanCreated,
	}, nil
}

// validateDynamicValueFunc checks that ft is one of the three supported signatures:
//
//	func() T
//	func() (T, error)
//	func(container.Container) (T, error)
func validateDynamicValueFunc(fieldName string, classPtr reflect.Type, ft reflect.Type) error {
	bad := func(msg string) error {
		return errors.Errorf("dynamic value field '%s' in '%v': %s", fieldName, classPtr, msg)
	}
	switch ft.NumIn() {
	case 0:
		// func() T  or  func() (T, error)
	case 1:
		if !ft.In(0).Implements(contextType) {
			return bad("single parameter must be container.Container")
		}
	default:
		return bad("must have 0 or 1 (container.Container) parameters")
	}
	switch ft.NumOut() {
	case 1:
		if ft.NumIn() != 0 {
			return bad("func(container.Container) must return (T, error), not just T")
		}
	case 2:
		if ft.Out(1) != errorType {
			return bad("second return value must be error")
		}
	default:
		return bad("must return 1 or 2 values")
	}
	return nil
}

// validateScopeProviderFunc checks that ft matches the expected provider signature for the given scope.
//
//	scope=prototype: func() (T, error) or func(context.Context) (T, error)
//	scope=request:   func(context.Context) (T, error)
func validateScopeProviderFunc(fieldName string, classPtr reflect.Type, ft reflect.Type, scope BeanScope) error {
	bad := func(msg string) error {
		return errors.Errorf("scoped field '%s' in '%v' (scope=%s): %s", fieldName, classPtr, scope, msg)
	}

	// Must return exactly (T, error)
	if ft.NumOut() != 2 {
		return bad("must return (T, error)")
	}
	if ft.Out(1) != errorType {
		return bad("second return value must be error")
	}

	switch scope {
	case ScopePrototype:
		switch ft.NumIn() {
		case 0:
			// func() (T, error) — OK
		case 1:
			if !ft.In(0).Implements(contextType) {
				return bad("single parameter must be context.Context")
			}
		default:
			return bad("must have 0 or 1 (context.Context) parameters")
		}
	case ScopeRequest:
		if ft.NumIn() != 1 {
			return bad("must have exactly 1 parameter (context.Context)")
		}
		if !ft.In(0).Implements(contextType) {
			return bad("parameter must be context.Context")
		}
	}
	return nil
}

func isSomeoneImplements(iface reflect.Type, list []reflect.Type) bool {
	for _, el := range list {
		if el.Implements(iface) {
			return true
		}
	}
	return false
}
