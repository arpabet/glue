/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"go.arpabet.com/glue"
	"github.com/stretchr/testify/require"
	"reflect"
	"strings"
	"testing"
)

type someService struct {
	glue.InitializingBean
	Initialized bool
	testing     *testing.T
}

func (t *someService) PostConstruct() error {
	println("*someService.PostConstruct")
	t.Initialized = true
	return nil
}

func (t *someService) GetProperty() string {
	require.True(t.testing, t.Initialized)
	println("*someService.GetProperty", t)
	return "someProperty"
}

var beanConstructedClass = reflect.TypeOf((*beanConstructed)(nil))

type beanConstructed struct {
	someService *someService
	testing     *testing.T
}

func (t *beanConstructed) Run() error {
	require.NotNil(t.testing, t.someService)
	require.True(t.testing, t.someService.Initialized)
	println("*beanConstructed.Run")
	return nil
}

type factoryBeanExample struct {
	glue.FactoryBean
	testing     *testing.T
	SomeService *someService `inject`
}

func (t *factoryBeanExample) Object() (interface{}, error) {
	require.NotNil(t.testing, t.SomeService)
	someProperty := t.SomeService.GetProperty()
	println("Construct beanConstructed after ", someProperty)
	return &beanConstructed{someService: t.SomeService, testing: t.testing}, nil
}

func (t *factoryBeanExample) ObjectType() reflect.Type {
	return beanConstructedClass
}

func (t *factoryBeanExample) ObjectName() string {
	return ""
}

func (t *factoryBeanExample) Singleton() bool {
	return true
}

type applicationContext struct {
	BeanConstructed *beanConstructed `inject`
}

type repeatedFactoryBeanExample struct {
	glue.FactoryBean
	testing *testing.T
}

func (t *repeatedFactoryBeanExample) Object() (interface{}, error) {
	return &beanConstructed{testing: t.testing}, nil
}

func (t *repeatedFactoryBeanExample) ObjectType() reflect.Type {
	return beanConstructedClass
}

func (t *repeatedFactoryBeanExample) ObjectName() string {
	return ""
}

func (t *repeatedFactoryBeanExample) Singleton() bool {
	return true
}

func TestSingleFactoryBean(t *testing.T) {

	ctx, err := glue.New(
		&someService{testing: t},
		&factoryBeanExample{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(beanConstructedClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))

	require.NotNil(t, b[0])

	b[0].Object().(*beanConstructed).Run()
}

func TestRepeatedFactoryBean(t *testing.T) {

	app := &applicationContext{}
	ctx, err := glue.New(
		&someService{testing: t},
		&factoryBeanExample{testing: t},
		&repeatedFactoryBeanExample{testing: t},
		app,
	)

	require.NotNil(t, err)
	require.Nil(t, ctx)
	require.True(t, strings.Contains(err.Error(), "repeated"))
	println(err.Error())
}

func TestFactoryBean(t *testing.T) {

	app := &applicationContext{}
	ctx, err := glue.New(
		app,
		&factoryBeanExample{testing: t},
		&someService{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, app.BeanConstructed)
	err = app.BeanConstructed.Run()
	require.NoError(t, err)
}

var SomeServiceClass = reflect.TypeOf((*SomeService)(nil)).Elem()

type SomeService interface {
	glue.InitializingBean
	Initialized() bool
	GetProperty() string
}

type someServiceImpl struct {
	initialized bool
	testing     *testing.T
}

func (t *someServiceImpl) PostConstruct() error {
	println("*someServiceImpl.PostConstruct")
	t.initialized = true
	return nil
}

func (t *someServiceImpl) Initialized() bool {
	return t.initialized
}

func (t *someServiceImpl) GetProperty() string {
	require.True(t.testing, t.initialized)
	println("*someServiceImpl.GetProperty", t)
	return "someProperty"
}

var BeanConstructedClass = reflect.TypeOf((*BeanConstructed)(nil)).Elem()

type BeanConstructed interface {
	Run() error
}

type beanConstructedImpl struct {
	someService SomeService
	testing     *testing.T
}

func (t *beanConstructedImpl) Run() error {
	require.NotNil(t.testing, t.someService)
	require.True(t.testing, t.someService.Initialized())
	println("*beanConstructedImpl.Run")
	return nil
}

type factoryBeanImpl struct {
	glue.FactoryBean
	testing     *testing.T
	SomeService SomeService `inject`
}

func (t *factoryBeanImpl) Object() (interface{}, error) {
	require.NotNil(t.testing, t.SomeService)
	someProperty := t.SomeService.GetProperty()
	println("Construct beanConstructedImpl after ", someProperty)
	return &beanConstructedImpl{someService: t.SomeService, testing: t.testing}, nil
}

func (t *factoryBeanImpl) ObjectType() reflect.Type {
	return BeanConstructedClass
}

func (t *factoryBeanImpl) ObjectName() string {
	return "beanConstructed"
}

func (t *factoryBeanImpl) Singleton() bool {
	return true
}

func TestFactoryInterfaceBean(t *testing.T) {

	ctx, err := glue.New(
		&factoryBeanImpl{testing: t},
		&someServiceImpl{testing: t},
		&struct {
			BeanConstructed BeanConstructed `inject:"bean=beanConstructed"`
		}{},
	)
	require.NoError(t, err)
	defer ctx.Close()

	bc := ctx.Bean(BeanConstructedClass, glue.DefaultLevel)
	require.Equal(t, 1, len(bc))

	err = bc[0].Object().(BeanConstructed).Run()
	require.NoError(t, err)
}
