/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue

import (
	"github.com/pkg/errors"
	"reflect"
)

/**
Named Bean Stub is using to replace empty field in struct that has glue.NamedBean type
*/

type namedBeanStub struct {
	name string
}

func (t *namedBeanStub) BeanName() string {
	return t.name
}

/**
Ordered Bean Stub is using to replace empty field in struct that has glue.OrderedBean type
*/

type orderedBeanStub struct {
}

func (t *orderedBeanStub) BeanOrder() int {
	return 0
}

/**
Initializing Bean Stub is using to replace empty field in struct that has glue.InitializingBean type
*/

type initializingBeanStub struct {
	name string
}

func (t *initializingBeanStub) PostConstruct() error {
	return errors.Errorf("bean '%s' does not implement PostConstruct method, but has anonymous field InitializingBean", t.name)
}

/**
Disposable Bean Stub is using to replace empty field in struct that has glue.DisposableBean type
*/

type disposableBeanStub struct {
	name string
}

func (t *disposableBeanStub) Destroy() error {
	return errors.Errorf("bean '%s' does not implement Destroy method, but has anonymous field DisposableBean", t.name)
}

/**
Factory Bean Stub is using to replace empty field in struct that has glue.FactoryBean type
*/

type factoryBeanStub struct {
	name     string
	elemType reflect.Type
}

func (t *factoryBeanStub) Object() (interface{}, error) {
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
