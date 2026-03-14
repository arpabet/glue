/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"reflect"
	"sort"

	"github.com/pkg/errors"
)

func (t *container) applyDecorators() error {
	// collect all Decorator beans
	var decorators []Decorator
	for _, beans := range t.core {
		for _, b := range beans {
			if d, ok := b.obj.(Decorator); ok {
				decorators = append(decorators, d)
			}
		}
	}

	if len(decorators) == 0 {
		return nil
	}

	// sort by OrderedBean if implemented
	sort.SliceStable(decorators, func(i, j int) bool {
		oi, iOrdered := decorators[i].(OrderedBean)
		oj, jOrdered := decorators[j].(OrderedBean)
		if iOrdered && jOrdered {
			return oi.BeanOrder() < oj.BeanOrder()
		}
		return false
	})

	for _, d := range decorators {
		targetType := d.DecorateType()
		if targetType == nil {
			continue
		}

		t.logger.Printf("Decorator %T for %v\n", d, targetType)

		for _, beans := range t.core {
			for _, b := range beans {
				if b.obj == nil {
					continue
				}
				// skip decorator beans themselves
				if _, isDecorator := b.obj.(Decorator); isDecorator {
					continue
				}

				beanType := reflect.TypeOf(b.obj)
				if !beanType.Implements(targetType) {
					continue
				}

				oldObj := b.obj

				decorated, err := d.Decorate(oldObj)
				if err != nil {
					return errors.Errorf("decorator %T failed for bean '%s': %v", d, b.name, err)
				}
				if decorated == nil {
					return errors.Errorf("decorator %T returned nil for bean '%s'", d, b.name)
				}

				decoratedType := reflect.TypeOf(decorated)
				if !decoratedType.Implements(targetType) {
					return errors.Errorf("decorated value from %T does not implement %v", d, targetType)
				}

				t.logger.Printf("  Decorated bean '%s' (%v -> %v)\n", b.name, beanType, decoratedType)
				b.obj = decorated
				b.valuePtr = reflect.ValueOf(decorated)

				// update already-injected interface fields in all consumer beans
				t.updateInjectedFields(targetType, oldObj, decorated)
			}
		}
	}

	return nil
}

// updateInjectedFields walks all beans and replaces injected interface fields
// that still point to oldObj with the decorated value.
func (t *container) updateInjectedFields(ifaceType reflect.Type, oldObj, newObj any) {
	oldVal := reflect.ValueOf(oldObj)

	for _, beans := range t.core {
		for _, b := range beans {
			if b.beanDef == nil || len(b.beanDef.fields) == 0 {
				continue
			}
			if !b.valuePtr.IsValid() || b.valuePtr.IsNil() {
				continue
			}

			structVal := b.valuePtr.Elem()

			for _, f := range b.beanDef.fields {
				if f.fieldType.Kind() != reflect.Interface {
					continue
				}
				if !f.fieldType.Implements(ifaceType) && f.fieldType != ifaceType {
					continue
				}

				field := structVal.Field(f.fieldNum)
				if !field.IsValid() || field.IsNil() {
					continue
				}

				// compare the underlying pointer
				if field.Elem().Pointer() == oldVal.Pointer() {
					field.Set(reflect.ValueOf(newObj))
				}
			}
		}
	}
}
