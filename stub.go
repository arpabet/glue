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

/**
Named Bean Stub is using to replace empty field in struct that has glue.NamedBean type
*/

type namedBeanStub struct {
	name string
}

func newNamedBeanStub(name string) NamedBean {
	return &namedBeanStub{name: name}
}

func (t *namedBeanStub) BeanName() string {
	return t.name
}

/**
Ordered Bean Stub is using to replace empty field in struct that has glue.OrderedBean type
*/

type orderedBeanStub struct {
}

func newOrderedBeanStub() OrderedBean {
	return &orderedBeanStub{}
}

func (t *orderedBeanStub) BeanOrder() int {
	return 0
}

type profileBeanStub struct {
}

func newProfileBeanStub() ProfileBean {
	return &profileBeanStub{}
}

func (t *profileBeanStub) BeanProfile() string { return "*" }

type conditionalBeanStub struct {
}

func newConditionalBeanStub() ConditionalBean {
	return &conditionalBeanStub{}
}

func (t *conditionalBeanStub) ShouldRegisterBean() bool { return true }

type scopedBeanStub struct {
}

func newScopedBeanStub() ScopedBean {
	return &scopedBeanStub{}
}

func (t *scopedBeanStub) BeanScope() BeanScope { return ScopeSingleton }

/**
Initializing Bean Stub is using to replace empty field in struct that has glue.InitializingBean type
*/

type initializingBeanStub struct {
	name string
}

func newInitializingBeanStub(name string) InitializingBean {
	return &initializingBeanStub{name: name}
}

func (t *initializingBeanStub) PostConstruct() error {
	return errors.Errorf("bean '%s' does not implement PostConstruct method, but has anonymous field InitializingBean", t.name)
}

type contextInitializingBeanStub struct {
	name string
}

func newContextInitializingBeanStub(name string) ContextInitializingBean {
	return &contextInitializingBeanStub{name: name}
}

func (t *contextInitializingBeanStub) PostConstruct(context.Context) error {
	return errors.Errorf("bean '%s' does not implement PostConstruct(ctx) method, but has anonymous field ContextInitializingBean", t.name)
}

/**
Disposable Bean Stub is using to replace empty field in struct that has glue.DisposableBean type
*/

type disposableBeanStub struct {
	name string
}

func newDisposableBeanStub(name string) DisposableBean {
	return &disposableBeanStub{name: name}
}

func (t *disposableBeanStub) Destroy() error {
	return errors.Errorf("bean '%s' does not implement Destroy method, but has anonymous field DisposableBean", t.name)
}

type contextDisposableBeanStub struct {
	name string
}

func newContextDisposableBeanStub(name string) ContextDisposableBean {
	return &contextDisposableBeanStub{name: name}
}

func (t *contextDisposableBeanStub) Destroy(context.Context) error {
	return errors.Errorf("bean '%s' does not implement Destroy(ctx) method, but has anonymous field ContextDisposableBean", t.name)
}

/**
Factory Bean Stub is using to replace empty field in struct that has glue.FactoryBean type
*/

type factoryBeanStub struct {
	name     string
	elemType reflect.Type
}

func newFactoryBeanStub(name string, elemType reflect.Type) FactoryBean {
	return &factoryBeanStub{name: name, elemType: elemType}
}

func (t *factoryBeanStub) Object() (any, error) {
	return nil, errors.Errorf("bean '%s' does not implement Object method, but has anonymous field FactoryBean", t.name)
}

func (t *factoryBeanStub) ObjectType() reflect.Type {
	return t.elemType
}

func (t *factoryBeanStub) ObjectName() string {
	return ""
}

func (t *factoryBeanStub) Singleton() bool {
	return true
}

type contextFactoryBeanStub struct {
	name     string
	elemType reflect.Type
}

func newContextFactoryBeanStub(name string, elemType reflect.Type) ContextFactoryBean {
	return &contextFactoryBeanStub{name: name, elemType: elemType}
}

func (t *contextFactoryBeanStub) Object(context.Context) (any, error) {
	return nil, errors.Errorf("bean '%s' does not implement Object(ctx) method, but has anonymous field ContextFactoryBean", t.name)
}

func (t *contextFactoryBeanStub) ObjectType() reflect.Type {
	return t.elemType
}

func (t *contextFactoryBeanStub) ObjectName() string {
	return ""
}

func (t *contextFactoryBeanStub) Singleton() bool {
	return true
}
