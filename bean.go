/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	DefaultLevel                          = CurrentContextWithParentFallbackLevel
	CurrentContextWithParentFallbackLevel = 0
	CurrentContextLevel                   = 1
	CurrentContextWithParentLevel         = 2
	CurrentContextWithTwoParentsLevel     = 3
	UnlimitedLevel                        = -1
)

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
	Fields that are going to be injected
	*/
	fields []*injectionDef

	/**
	Properties that are going to be injected
	*/
	properties []*propInjectionDef
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
	Factory of the bean if exist
	*/
	beenFactory *factory

	/**
	Instance to the bean, could be empty if beenFactory exist
	*/
	obj interface{}

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
	return fmt.Sprintf("context{level=%d, beans=%v}", t.level, t.list)
}

func (t *bean) String() string {
	pointer := uintptr(unsafe.Pointer(&t.obj))
	if t.beenFactory != nil {
		objectName := t.beenFactory.factoryBean.ObjectName()
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

func (t *bean) Object() interface{} {
	return t.obj
}

func (t *bean) FactoryBean() (Bean, bool) {
	if t.beenFactory != nil {
		return t.beenFactory.bean, true
	} else {
		return nil, false
	}
}

func (t *bean) Reload() error {
	t.ctorMu.Lock()
	defer t.ctorMu.Unlock()

	t.lifecycle = BeanDestroying
	if dis, ok := t.obj.(DisposableBean); ok {
		if err := dis.Destroy(); err != nil {
			return err
		}
	}
	t.lifecycle = BeanConstructing
	if t.beenFactory != nil {
		return errors.Errorf("bean '%s' was created by factory bean '%v and can not be reloaded", t.name, t.beenFactory.factoryClassPtr)
	} else {
		if init, ok := t.obj.(InitializingBean); ok {
			if err := init.PostConstruct(); err != nil {
				return err
			}
		}
	}
	t.lifecycle = BeanInitialized
	return nil
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
	Bean associated with Factory in context
	*/
	bean *bean

	/**
	Instance to the factory bean
	*/
	factoryObj interface{}

	/**
	Factory bean type
	*/
	factoryClassPtr reflect.Type

	/**
	Factory bean interface
	*/
	factoryBean FactoryBean

	/**
	Created bean instances by this factory
	*/
	instances []*bean
}

func (t *factory) String() string {
	return t.factoryClassPtr.String()
}

func (t *factory) ctor() (*bean, bool, error) {
	var b *bean
	var singleton bool

	if len(t.instances) == 0 {
		return nil, false, errors.Errorf("internal: element bean collection is empty for factory '%v'", t.factoryClassPtr)
	}

	if t.factoryBean.Singleton() {
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

	obj, err := t.factoryBean.Object()
	if err != nil {
		return nil, false, errors.Errorf("factory bean '%v' failed to create bean '%v', %v", t.factoryClassPtr, t.factoryBean.ObjectType(), err)
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
Investigate bean by using reflection
*/
func investigate(obj interface{}, classPtr reflect.Type) (*bean, error) {
	var fields []*injectionDef
	var properties []*propInjectionDef
	var anonymousFields []reflect.Type
	valuePtr := reflect.ValueOf(obj)
	value := valuePtr.Elem()
	class := classPtr.Elem()
	for j := 0; j < class.NumField(); j++ {
		field := class.Field(j)

		if field.Anonymous {
			anonymousFields = append(anonymousFields, field.Type)
			switch field.Type {
			case NamedBeanClass:
				stub := &namedBeanStub{name: classPtr.String()}
				stubValuePtr := reflect.ValueOf(stub)
				value.Field(j).Set(stubValuePtr)
			case OrderedBeanClass:
				stub := &orderedBeanStub{}
				stubValuePtr := reflect.ValueOf(stub)
				value.Field(j).Set(stubValuePtr)
			case InitializingBeanClass:
				stub := &initializingBeanStub{name: classPtr.String()}
				stubValuePtr := reflect.ValueOf(stub)
				value.Field(j).Set(stubValuePtr)
			case DisposableBeanClass:
				stub := &disposableBeanStub{name: classPtr.String()}
				stubValuePtr := reflect.ValueOf(stub)
				value.Field(j).Set(stubValuePtr)
			case FactoryBeanClass:
				stub := &factoryBeanStub{name: classPtr.String(), elemType: classPtr}
				stubValuePtr := reflect.ValueOf(stub)
				value.Field(j).Set(stubValuePtr)
			case ContextClass:
				return nil, errors.Errorf("exposing by anonymous field '%s' in '%v' interface glue.Context is not allowed", field.Name, classPtr)
			}
		}

		if valueTag, hasValueTag := field.Tag.Lookup("value"); hasValueTag {
			if field.Anonymous {
				return nil, errors.Errorf("injection to anonymous field '%s' in '%v' is not allowed", field.Name, classPtr)
			}
			var propertyName string
			var defaultValue string
			var hasDefaultValue bool
			var layout string
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
						layout = strings.TrimSpace(kv[1])
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
				layout:          layout,
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
			level := DefaultLevel
			if hasInjectTag {
				pairs := strings.Split(injectTag, ",")
				for _, pair := range pairs {
					p := strings.TrimSpace(pair)
					kv := strings.SplitN(p, "=", 2)
					switch strings.TrimSpace(kv[0]) {
					case "bean":
						if len(kv) > 1 {
							qualifier = strings.TrimSpace(kv[1])
						}
					case "optional":
						optional = true
					case "lazy":
						lazy = true
					case "level":
						if len(kv) > 1 {
							level, _ = strconv.Atoi(kv[1])
						}
					}
				}
			}
			kind := field.Type.Kind()
			fieldType := field.Type
			var fieldSlice, fieldMap bool
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
			if kind != reflect.Ptr && kind != reflect.Interface && kind != reflect.Func {
				return nil, errors.Errorf("not a pointer, interface or function field type '%v' on position %d in %v with 'inject' tag", field.Type, j, classPtr)
			}
			def := &injectionDef{
				class:     class,
				fieldNum:  j,
				fieldName: field.Name,
				fieldType: fieldType,
				lazy:      lazy,
				slice:     fieldSlice,
				table:     fieldMap,
				optional:  optional,
				qualifier: qualifier,
				level:     level,
			}
			fields = append(fields, def)
		}
	}
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
	return &bean{
		name:      name,
		qualifier: qualifier,
		ordered:   ordered,
		order:     order,
		obj:       obj,
		valuePtr:  valuePtr,
		beanDef: &beanDef{
			classPtr:        classPtr,
			anonymousFields: anonymousFields,
			fields:          fields,
			properties:      properties,
		},
		lifecycle: BeanCreated,
	}, nil
}

func isSomeoneImplements(iface reflect.Type, list []reflect.Type) bool {
	for _, el := range list {
		if el.Implements(iface) {
			return true
		}
	}
	return false
}
