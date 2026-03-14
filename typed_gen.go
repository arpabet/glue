//go:build go1.18

/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
)

func beanType[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func GetBean[T any](c Container) (T, error) {
	var zero T
	typ := beanType[T]()
	beans := c.Bean(typ, DefaultSearchLevel)
	if len(beans) == 0 {
		return zero, errors.Errorf("bean '%s' not found", typ)
	}
	if len(beans) > 1 {
		return zero, errors.Errorf("bean '%s' is ambiguous", typ)
	}
	obj := beans[0].Object()
	if obj == nil {
		return zero, errors.Errorf("bean '%s' is not initialized", typ)
	}
	value, ok := obj.(T)
	if !ok {
		return zero, errors.Errorf("bean '%s' of type '%T' cannot be converted to '%s'", typ, obj, typ)
	}
	return value, nil
}

func MustGetBean[T any](c Container) T {
	value, err := GetBean[T](c)
	if err != nil {
		panic(err)
	}
	return value
}

func GetBeans[T any](c Container) []T {
	var list []T
	typ := beanType[T]()
	beans := c.Bean(typ, DefaultSearchLevel)
	for _, b := range beans {
		if value, ok := b.Object().(T); ok {
			list = append(list, value)
		}
	}
	return list
}

func GetProperty[T any](c Container, key string) (T, error) {
	var zero T
	props := c.Properties()
	value, ok, err := props.Resolve(key)
	if err != nil {
		return zero, err
	}
	if !ok {
		return zero, errors.Errorf("property '%s' not found", key)
	}
	return convertPropertyValue[T](value)
}

func GetPropertyOr[T any](c Container, key string, def T) T {
	value, err := GetProperty[T](c, key)
	if err != nil {
		return def
	}
	return value
}

func SingletonFactory[T any](factory func() (T, error)) FactoryBean {
	return &genericFactory[T]{fn: factory, singleton: true}
}

func PrototypeFactory[T any](factory func() (T, error)) FactoryBean {
	return &genericFactory[T]{fn: factory, singleton: false}
}

func RequestFactory[T any](factory func() (T, error)) FactoryBean {
	return &genericFactory[T]{fn: factory, singleton: false}
}

type genericFactory[T any] struct {
	fn        func() (T, error)
	singleton bool
}

func (t *genericFactory[T]) Object() (any, error) {
	return t.fn()
}

func (t *genericFactory[T]) ObjectType() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func (t *genericFactory[T]) ObjectName() string {
	return t.ObjectType().String()
}

func (t *genericFactory[T]) Singleton() bool {
	return t.singleton
}

func SingletonContextFactory[T any](factory func(context.Context) (T, error)) ContextFactoryBean {
	return &genericContextFactory[T]{fn: factory, singleton: true}
}

func PrototypeContextFactory[T any](factory func(context.Context) (T, error)) ContextFactoryBean {
	return &genericContextFactory[T]{fn: factory, singleton: false}
}

func RequestContextFactory[T any](factory func(context.Context) (T, error)) ContextFactoryBean {
	return &genericContextFactory[T]{fn: factory, singleton: false}
}

type genericContextFactory[T any] struct {
	fn        func(context.Context) (T, error)
	singleton bool
}

func (t *genericContextFactory[T]) Object(ctx context.Context) (any, error) {
	return t.fn(ctx)
}

func (t *genericContextFactory[T]) ObjectType() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func (t *genericContextFactory[T]) ObjectName() string {
	return t.ObjectType().String()
}

func (t *genericContextFactory[T]) Singleton() bool {
	return t.singleton
}
