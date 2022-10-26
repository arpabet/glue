/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue

import (
	"fmt"
	"github.com/pkg/errors"
	"io/fs"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
)

var (

   durationClass = reflect.TypeOf(time.Millisecond)
   timeClass = reflect.TypeOf(time.Time{})
   osFileModeClass = reflect.TypeOf(os.FileMode(0777))
   fsFileModeClass = reflect.TypeOf(fs.FileMode(0777))
)

type injectionDef struct {

	/**
	Class of that struct
	*/
	class reflect.Type
	/**
	Field number of that struct
	*/
	fieldNum int
	/**
	Field name where injection is going to be happen
	*/
	fieldName string
	/**
	Type of the field that is going to be injected
	*/
	fieldType reflect.Type
	/**
	Field is Slice of beans
	*/
	slice bool
	/**
	Field is Map of beans
	*/
	table bool
	/**
	Lazy injection represented by function
	*/
	lazy bool
	/**
	Optional injection
	*/
	optional bool
	/*
	Injection expects the specific bean to be injected
	*/
	qualifier string
	/**
	Level of how deep we need to search beans for injection

	level 0: look in the current context, if not found then look in the parent context and so on (default)
	level 1: look only in the current context
	level 2: look in the current context in union with the parent context
	level 3: look in union of current, parent, parent of parent contexts
	and so on.
	level -1: look in union of all contexts.
	 */
	level int
}

type injection struct {

	/*
	Bean where injection is going to be happen
	*/
	bean *bean

	/**
	Reflection value of the bean where injection is going to be happen
	*/
	value reflect.Value

	/**
	Injection information
	*/
	injectionDef *injectionDef
}

type propInjectionDef struct {

	/**
	Class of that struct
	*/
	class reflect.Type

	/**
	Field number of that struct
	*/
	fieldNum int

	/**
	Field name where injection is going to be happen
	*/
	fieldName string

	/**
	Type of the field that is going to be injected
	*/
	fieldType reflect.Type

	/**
	Property name of injecting placeholder property
	 */
	propertyName string

	/**
	Default value of the property to inject
	*/
	defaultValue string

	/**
	Layout for date-time property
	 */
	layout  string
}

type propInjection struct {

	/*
	Bean where injection is going to be happen
	*/
	bean *bean

	/**
	Reflection value of the bean where injection is going to be happen
	*/
	value reflect.Value

	/**
	Injection information
	*/
	injectionDef *propInjectionDef
}


/*
	Prepare beans for the specific level of injection
 */
func levelBeans(deep []beanlist, level int) []*bean {

	switch level {
	case -1:
		var candidates []*bean
		for _, entry := range deep {
			candidates = append(candidates, entry.list...)
		}
		return candidates
	case 0:
		// always the first available level, regardless if it current or not
		return deep[0].list
	case 1:
		if deep[0].level == 1 {
			return deep[0].list
		} else {
			return nil
		}
	default:
		var candidates []*bean
		for _, entry := range deep {
			if entry.level > level {
				break
			}
			candidates = append(candidates, entry.list...)
		}
		return candidates
	}

}

/**
	Order beans, all or partially
 */
func orderBeans(candidates []*bean) []*bean {
	var ordered []*bean
	for _, candidate := range candidates {
		if candidate.ordered {
			ordered = append(ordered, candidate)
		}
	}
	n := len(ordered)
	if n > 0 {
		sort.Slice(ordered, func(i, j int) bool {
			return ordered[i].order < ordered[j].order
		})
		if n != len(candidates) {
			var unordered []*bean
			for _, candidate := range candidates {
				if !candidate.ordered {
					unordered = append(unordered, candidate)
				}
			}
			return append(ordered, unordered...)
		}
		return ordered
	} else {
		return candidates
	}
}

/**
Inject value in to the field by using reflection
*/
func (t *injection) inject(deep []beanlist) error {

	list := orderBeans(levelBeans(deep, t.injectionDef.level))

	field := t.value.Field(t.injectionDef.fieldNum)
	if !field.CanSet() {
		return errors.Errorf("field '%s' in class '%v' is not public", t.injectionDef.fieldName, t.injectionDef.class)
	}

	list = t.injectionDef.filterBeans(list)

	if len(list) == 0 {
		if !t.injectionDef.optional {
			if t.injectionDef.qualifier != "" {
				return errors.Errorf("can not find candidates to inject the required field '%s' in class '%v' with qualifier '%s'", t.injectionDef.fieldName, t.injectionDef.class, t.injectionDef.qualifier)
			} else {
				return errors.Errorf("can not find candidates to inject the required field '%s' in class '%v'", t.injectionDef.fieldName, t.injectionDef.class)
			}
		}
		return nil
	}

	if t.injectionDef.slice {

		newSlice := field
		var factoryList []*bean
		for _, impl := range list {
			if impl.beenFactory != nil {
				factoryList = append(factoryList, impl)
			} else {
				newSlice = reflect.Append(newSlice, impl.valuePtr)

				// register dependency that 'inject.bean' is using if it is not lazy
				if !t.injectionDef.lazy && t.bean != impl {
					t.bean.dependencies = append(t.bean.dependencies, impl)
				}

			}
		}
		field.Set(newSlice)

		for _, instance := range factoryList {
			// register factory dependency for 'inject.bean' that is using 'factory'
			t.bean.factoryDependencies = append(t.bean.factoryDependencies,
				&factoryDependency{
					factory: instance.beenFactory,
					injection: func(service *bean) error {
						field.Set(reflect.Append(field, instance.valuePtr))
						return nil
					},
				})
		}

		return nil
	}

	if t.injectionDef.table {

		field.Set(reflect.MakeMap(field.Type()))

		visited := make(map[string]bool)
		for _, impl := range list {
			if impl.beenFactory != nil {
				// register factory dependency for 'inject.bean' that is using 'factory'
				t.bean.factoryDependencies = append(t.bean.factoryDependencies,
					&factoryDependency{
						factory: impl.beenFactory,
						injection: func(service *bean) error {
							if visited[service.name] {
								return errors.Errorf("can not inject duplicates '%s' to the map field '%s' in class '%v' by injecting factory bean '%v'", impl.name, t.injectionDef.fieldName, t.injectionDef.class, service.obj)
							}
							visited[service.name] = true
							field.SetMapIndex(reflect.ValueOf(service.name), service.valuePtr)
							return nil
						},
					})
			} else {
				if visited[impl.name] {
					return errors.Errorf("can not inject duplicates '%s' to the map field '%s' in class '%v' by injecting impl '%v'", impl.name, t.injectionDef.fieldName, t.injectionDef.class, impl.obj)
				}
				visited[impl.name] = true
				field.SetMapIndex(reflect.ValueOf(impl.name), impl.valuePtr)

				// register dependency that 'inject.bean' is using if it is not lazy
				if !t.injectionDef.lazy && t.bean != impl {
					t.bean.dependencies = append(t.bean.dependencies, impl)
				}
			}
		}

		return nil
	}

	if len(list) > 1 {
		return errors.Errorf("field '%s' in class '%v' can not be injected with multiple candidates %+v", t.injectionDef.fieldName, t.injectionDef.class, list)
	}

	impl := list[0]

	if impl.beenFactory != nil {
		if t.injectionDef.lazy {
			return errors.Errorf("lazy injection is not supported of type '%v' through factory '%v' in to '%v'", impl.beenFactory.factoryBean.ObjectType(), impl.beenFactory.factoryClassPtr, t.String())
		}

		// register factory dependency for 'inject.bean' that is using 'factory'
		t.bean.factoryDependencies = append(t.bean.factoryDependencies,
			&factoryDependency{
				factory: impl.beenFactory,
				injection: func(service *bean) error {
					field.Set(service.valuePtr)
					return nil
				},
			})

		return nil
	}

	field.Set(impl.valuePtr)

	// register dependency that 'inject.bean' is using if it is not lazy
	if !t.injectionDef.lazy && t.bean != impl {
		t.bean.dependencies = append(t.bean.dependencies, impl)
	}

	return nil
}

//atomic.StoreUintptr((*uintptr)(unsafe.Pointer(field.Addr().Pointer())), impl.valuePtr.Pointer())
func atomicSet(field reflect.Value, instance reflect.Value) {
	atomic.StoreUintptr((*uintptr)(unsafe.Pointer(field.Addr().Pointer())), instance.Pointer())
}

// runtime injection
func (t *injectionDef) inject(value *reflect.Value, deep []beanlist) error {

	list := orderBeans(levelBeans(deep, t.level))

	field := value.Field(t.fieldNum)

	if !field.CanSet() {
		return errors.Errorf("field '%s' in class '%v' is not public", t.fieldName, t.class)
	}

	list = t.filterBeans(list)

	if len(list) == 0 {
		if !t.optional {
			if t.qualifier != "" {
				return errors.Errorf("can not find candidates to inject the required field '%s' in class '%v' with qualifier '%s'", t.fieldName, t.class, t.qualifier)
			} else {
				return errors.Errorf("can not find candidates to inject the required field '%s' in class '%v'", t.fieldName, t.class)
			}
		}
		return nil
	}

	if t.slice {

		newSlice := field
		for _, bean := range list {
			if !bean.valuePtr.IsValid() {
				newSlice = reflect.Append(newSlice, reflect.Zero(t.fieldType))
			} else {
				newSlice = reflect.Append(newSlice, bean.valuePtr)
			}
		}
		field.Set(newSlice)
		return nil
	}

	if t.table {

		field.Set(reflect.MakeMap(field.Type()))

		visited := make(map[string]bool)
		for _, instance := range list {
			if !instance.valuePtr.IsValid() {
				if visited[instance.name] {
					return errors.Errorf("can not inject duplicates '%s' to the map field '%s' in class '%v'", instance.name, t.fieldName, t.class)
				}
				visited[instance.name] = true
				field.SetMapIndex(reflect.ValueOf(instance.name), instance.valuePtr)
			}
		}

		return nil
	}

	if len(list) > 1 {
		return errors.Errorf("field '%s' in class '%v' can not be injected with multiple candidates %+v", t.fieldName, t.class, list)
	}

	impl := list[0]

	if impl.lifecycle != BeanInitialized {
		return errors.Errorf("field '%s' in class '%v' can not be injected with non-initialized bean %+v", t.fieldName, t.class, impl)
	}

	if impl.beenFactory != nil {

		service, _, err := impl.beenFactory.ctor()
		if err != nil {
			return errors.Errorf("field '%s' in class '%v' can not be injected because of factory bean %+v error, %v", t.fieldName, t.class, impl, err)
		}

		impl = service
	}

	field.Set(impl.valuePtr)

	return nil
}

func (t *injectionDef) filterBeans(list []*bean) []*bean {
	if t.qualifier != "" {
		var candidates []*bean
		for _, b := range list {
			if t.qualifier == b.name {
				candidates = append(candidates, b)
			}
		}
		return candidates
	} else {
		return list
	}
}

/**
User friendly information about class and field
*/

func (t *injection) String() string {
	return t.injectionDef.String()
}

func (t *injectionDef) String() string {
	if t.qualifier != "" {
		return fmt.Sprintf(" %v->%s(%s) ", t.class, t.fieldName, t.qualifier)
	} else {
		return fmt.Sprintf(" %v->%s ", t.class, t.fieldName)
	}
}

// runtime injection
func (t *propInjectionDef) inject(value *reflect.Value, properties Properties) error {

	field := value.Field(t.fieldNum)

	if !field.CanSet() {
		return errors.Errorf("field '%s' in class '%v' is not public", t.fieldName, t.class)
	}

	strValue := properties.GetString(t.propertyName, t.defaultValue)

	v, err := convertProperty(strValue, t.fieldType, t.layout)
	if err != nil {
		return errors.Errorf("property '%s' in class '%v' has convert error, property resolvers %+v, %v", t.fieldName, t.class, properties.PropertyResolvers(), err)
	}

	field.Set(v)
	return nil

}

func convertProperty(s string, t reflect.Type, layout string) (val reflect.Value, err error) {
	var v interface{}

	switch {

	case isArray(t):
		parts := trimSplit(s, ";")
		slice := reflect.MakeSlice(t, 0, len(parts))
		for _, s := range parts {
			val, err := convertProperty(s, t.Elem(), layout)
			if err != nil {
				return slice, err
			}
			slice = reflect.Append(slice, val)
		}
		return slice, err

	case isDuration(t):
		v, err = time.ParseDuration(s)

	case isTime(t):
		if layout == "" {
			layout = time.RFC3339
		}
		v, err = time.Parse(layout, s)

	case isFileMode(t):
		v, err = parseFileMode(s), nil

	case isBool(t):
		v, err = parseBool(s)

	case isString(t):
		v, err = s, nil

	case isFloat(t):
		v, err = strconv.ParseFloat(s, 64)

	case isInt(t):
		v, err = strconv.ParseInt(s, 10, 64)

	case isUint(t):
		v, err = strconv.ParseUint(s, 10, 64)

	default:
		return reflect.Zero(t), fmt.Errorf("unsupported type %s", t)
	}

	if err != nil {
		return reflect.Zero(t), err
	}

	return reflect.ValueOf(v).Convert(t), nil
}

func isBool(t reflect.Type) bool {
	return t.Kind() == reflect.Bool
}

func isString(t reflect.Type) bool {
	return t.Kind() == reflect.String
}

func isFloat(t reflect.Type) bool {
	return t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64
}

func isInt(t reflect.Type) bool {
	return t.Kind() == reflect.Int || t.Kind() == reflect.Int8 || t.Kind() == reflect.Int16 || t.Kind() == reflect.Int32 || t.Kind() == reflect.Int64
}

func isUint(t reflect.Type) bool {
	return t.Kind() == reflect.Uint || t.Kind() == reflect.Uint8 || t.Kind() == reflect.Uint16 || t.Kind() == reflect.Uint32 || t.Kind() == reflect.Uint64
}

func isDuration(t reflect.Type) bool {
	return t == durationClass
}

func isTime(t reflect.Type) bool {
	return t == timeClass
}

func isFileMode(t reflect.Type) bool {
	return t == osFileModeClass || t == fsFileModeClass
}

func isArray(t reflect.Type) bool {
	return t.Kind() == reflect.Array || t.Kind() == reflect.Slice
}

func trimSplit(s string, sep string) []string {
	var a []string
	for _, v := range strings.Split(s, sep) {
		if v = strings.TrimSpace(v); v != "" {
			a = append(a, v)
		}
	}
	return a
}