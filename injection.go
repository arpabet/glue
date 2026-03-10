/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

var (
	durationClass   = reflect.TypeOf(time.Millisecond)
	timeClass       = reflect.TypeOf(time.Time{})
	osFileModeClass = reflect.TypeOf(os.FileMode(0777))
	fsFileModeClass = reflect.TypeOf(fs.FileMode(0777))
)

type injectionDef struct {

	/*
		Class of that struct
	*/
	class reflect.Type
	/*
		Field number of that struct
	*/
	fieldNum int
	/*
		Field name where injection is going to be happen
	*/
	fieldName string
	/*
		Type of the field that is going to be injected
	*/
	fieldType reflect.Type
	/*
		Field is Slice of beans
	*/
	isSlice bool
	/*
		Field is Map of beans
	*/
	isMap bool
	/*
		Lazy injection represented by function
	*/
	lazy bool
	/*
		Optional injection
	*/
	optional bool
	/*
		Injection expects the specific bean to be injected
	*/
	qualifier string
	/*
		Level of how deep we need to search beans for injection

		level 0: look in the current container, if not found then look in the parent container and so on (default)
		level 1: look only in the current container
		level 2: look in the current container in union with the parent container
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

	/*
		Reflection value of the bean where injection is going to be happen
	*/
	value reflect.Value

	/*
		Injection information
	*/
	injectionDef *injectionDef
}

type propInjectionDef struct {

	/*
		Class of that struct
	*/
	class reflect.Type

	/*
		Field number of that struct
	*/
	fieldNum int

	/*
		Field name where injection is going to be happen
	*/
	fieldName string

	/*
		Type of the field that is going to be injected
	*/
	fieldType reflect.Type

	/*
		Property name of injecting placeholder property
	*/
	propertyName string

	/*
		Default value of the property to inject
	*/
	defaultValue string

	/*
		Flag set if default value exist
	*/
	hasDefaultValue bool

	/*
		Time Format for date-time property
	*/
	timeFormat string

	/*
		dynamic is true when the field type is a function — property is resolved lazily on each call
	*/
	dynamic bool

	/*
		funcReturnType is T in func() T, func() (T, error), or func(context.Context) (T, error)
	*/
	funcReturnType reflect.Type

	/*
		funcReturnsError is true for func() (T, error) and func(context.Context) (T, error) signatures
	*/
	funcReturnsError bool

	/*
		funcTakesContext is true for func(context.Context) (T, error) signature
	*/
	funcTakesContext bool
}

var (
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
)

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

/*
*
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

func selectSingleCandidate(fieldName string, class reflect.Type, list []*bean) (*bean, error) {
	if len(list) == 0 {
		return nil, nil
	}
	if len(list) == 1 {
		return list[0], nil
	}

	primaryIdx := -1
	for i := range list {
		if list[i].primary {
			if primaryIdx != -1 {
				return nil, errors.Errorf(
					"field '%s' in class '%v' cannot be injected: multiple candidates %+v contain multiple primary beans",
					fieldName, class, list,
				)
			}
			primaryIdx = i
		}
	}

	if primaryIdx == -1 {
		return nil, errors.Errorf(
			"field '%s' in class '%v' cannot be injected with multiple candidates %+v",
			fieldName, class, list,
		)
	}

	return list[primaryIdx], nil
}

/*
*
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

	if t.injectionDef.isSlice {

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
						field.Set(reflect.Append(field, service.valuePtr))
						return nil
					},
				})
		}

		return nil
	}

	if t.injectionDef.isMap {

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

	impl, err := selectSingleCandidate(t.injectionDef.fieldName, t.injectionDef.class, list)
	if err != nil {
		return err
	}

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

// atomic.StoreUintptr((*uintptr)(unsafe.Pointer(field.Addr().Pointer())), impl.valuePtr.Pointer())
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

	if t.isSlice {

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

	if t.isMap {

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

	impl, err := selectSingleCandidate(t.fieldName, t.class, list)
	if err != nil {
		return err
	}

	if impl.lifecycle != BeanInitialized {
		return errors.Errorf("field '%s' in class '%v' can not be injected with non-initialized bean %+v", t.fieldName, t.class, impl)
	}

	if impl.beenFactory != nil {

			service, _, err := impl.beenFactory.ctor(context.Background())
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

/*
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

	if t.dynamic {
		return t.injectDynamic(field, properties)
	}

	var strValue string
	if value, ok := properties.Get(t.propertyName); ok {
		strValue = value
	} else if t.hasDefaultValue {
		strValue = t.defaultValue
	} else {
		return errors.Errorf("property '%s' in class '%v' does not have the default value, and did not find in property resolvers %+v", t.fieldName, t.class, properties.PropertyResolvers())
	}

	v, err := convertProperty(strValue, t.fieldType, t.timeFormat)
	if err != nil {
		return errors.Errorf("property '%s' in class '%v' has convert error, property resolvers %+v, %v", t.fieldName, t.class, properties.PropertyResolvers(), err)
	}

	field.Set(v)
	return nil

}

func (t *propInjectionDef) injectDynamic(field reflect.Value, properties Properties) error {
	propertyName := t.propertyName
	defaultValue := t.defaultValue
	hasDefaultValue := t.hasDefaultValue
	timeFormat := t.timeFormat
	returnType := t.funcReturnType

	resolve := func() (string, bool) {
		if val, ok := properties.Get(propertyName); ok {
			return val, true
		}
		if hasDefaultValue {
			return defaultValue, true
		}
		return "", false
	}

	convert := func(s string) (reflect.Value, error) {
		return convertProperty(s, returnType, timeFormat)
	}

	zeroReturn := reflect.Zero(returnType)
	zeroError := reflect.Zero(errorType)

	var fn reflect.Value

	switch {
	case t.funcTakesContext:
		// func(container.Container) (T, error)
		fn = reflect.MakeFunc(t.fieldType, func(args []reflect.Value) []reflect.Value {
			str, ok := resolve()
			if !ok {
				return []reflect.Value{zeroReturn, reflect.ValueOf(fmt.Errorf("property '%s' not found and has no default value", propertyName))}
			}
			val, err := convert(str)
			if err != nil {
				return []reflect.Value{zeroReturn, reflect.ValueOf(err)}
			}
			return []reflect.Value{val, zeroError}
		})

	case t.funcReturnsError:
		// func() (T, error)
		fn = reflect.MakeFunc(t.fieldType, func(args []reflect.Value) []reflect.Value {
			str, ok := resolve()
			if !ok {
				return []reflect.Value{zeroReturn, reflect.ValueOf(fmt.Errorf("property '%s' not found and has no default value", propertyName))}
			}
			val, err := convert(str)
			if err != nil {
				return []reflect.Value{zeroReturn, reflect.ValueOf(err)}
			}
			return []reflect.Value{val, zeroError}
		})

	default:
		// func() T — default= is guaranteed at construction time, so resolve() always returns true.
		fn = reflect.MakeFunc(t.fieldType, func(args []reflect.Value) []reflect.Value {
			str, ok := resolve()
			if !ok {
				return []reflect.Value{zeroReturn}
			}
			val, err := convert(str)
			if err != nil {
				panic(fmt.Sprintf("property '%s' convert error: %v", propertyName, err))
			}
			return []reflect.Value{val}
		})
	}

	field.Set(fn)
	return nil
}

func convertProperty(s string, t reflect.Type, timeFormat string) (val reflect.Value, err error) {
	var v interface{}

	switch {

	case isArray(t):
		parts := trimSplit(s, ";")
		slice := reflect.MakeSlice(t, 0, len(parts))
		for _, s := range parts {
			val, err := convertProperty(s, t.Elem(), timeFormat)
			if err != nil {
				return slice, err
			}
			slice = reflect.Append(slice, val)
		}
		return slice, err

	case isDuration(t):
		v, err = time.ParseDuration(s)

	case isTime(t):
		if timeFormat == "" {
			timeFormat = time.RFC3339
		}
		v, err = time.Parse(timeFormat, s)

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
