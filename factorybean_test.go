/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
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
	SomeService *someService `inject:""`
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
	BeanConstructed *beanConstructed `inject:""`
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

	b := ctx.Bean(beanConstructedClass, glue.DefaultSearchLevel)
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
	SomeService SomeService `inject:""`
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

	bc := ctx.Bean(BeanConstructedClass, glue.DefaultSearchLevel)
	require.Equal(t, 1, len(bc))

	err = bc[0].Object().(BeanConstructed).Run()
	require.NoError(t, err)
}

type contextFactoryBeanExample struct {
	received context.Context
}

func (t *contextFactoryBeanExample) Object(ctx context.Context) (interface{}, error) {
	t.received = ctx
	return &beanConstructed{}, nil
}

func (t *contextFactoryBeanExample) ObjectType() reflect.Type {
	return beanConstructedClass
}

func (t *contextFactoryBeanExample) ObjectName() string {
	return ""
}

func (t *contextFactoryBeanExample) Singleton() bool {
	return true
}

func TestContextFactoryBean(t *testing.T) {
	f := &contextFactoryBeanExample{}
	ctx := context.WithValue(context.Background(), "factory-test", "ctx-value")

	ctn, err := glue.NewWithContext(ctx, f)
	require.NoError(t, err)
	defer ctn.Close()

	b := ctn.Bean(beanConstructedClass, glue.DefaultSearchLevel)
	require.Equal(t, 1, len(b))
	require.Same(t, ctx, f.received)
	require.Equal(t, "ctx-value", f.received.Value("factory-test"))
}

// --- Test: FactoryBean-produced bean lifecycle (Spring-compatible behavior) ---
//
// Per Spring Framework conventions, the container manages the FactoryBean's own
// lifecycle (PostConstruct/Destroy), but does NOT call lifecycle hooks on the
// object produced by FactoryBean.Object(). The produced bean is transient from
// the container's perspective — if it needs initialization or cleanup, the
// FactoryBean itself is responsible for managing that.

// lifecycleProducedBean is a bean produced by a FactoryBean that implements
// both InitializingBean and DisposableBean.
type lifecycleProducedBean struct {
	postConstructCalled int32
	destroyCalled       int32
}

func (t *lifecycleProducedBean) PostConstruct() error {
	atomic.AddInt32(&t.postConstructCalled, 1)
	return nil
}

func (t *lifecycleProducedBean) Destroy() error {
	atomic.AddInt32(&t.destroyCalled, 1)
	return nil
}

var lifecycleProducedBeanClass = reflect.TypeOf((*lifecycleProducedBean)(nil))

// lifecycleFactory is a FactoryBean that produces lifecycleProducedBean instances.
// The factory itself also implements InitializingBean and DisposableBean to verify
// the container DOES manage the factory's own lifecycle.
type lifecycleFactory struct {
	glue.FactoryBean
	produced                *lifecycleProducedBean
	factoryPostConstructed  int32
	factoryDestroyed        int32
}

func (t *lifecycleFactory) PostConstruct() error {
	atomic.AddInt32(&t.factoryPostConstructed, 1)
	return nil
}

func (t *lifecycleFactory) Destroy() error {
	atomic.AddInt32(&t.factoryDestroyed, 1)
	return nil
}

func (t *lifecycleFactory) Object() (interface{}, error) {
	t.produced = &lifecycleProducedBean{}
	return t.produced, nil
}

func (t *lifecycleFactory) ObjectType() reflect.Type {
	return lifecycleProducedBeanClass
}

func (t *lifecycleFactory) ObjectName() string {
	return ""
}

func (t *lifecycleFactory) Singleton() bool {
	return true
}

// TestFactoryBeanProducedBeanLifecycle verifies that the container does NOT call
// PostConstruct or Destroy on the object produced by a FactoryBean — matching
// Spring Framework behavior where the produced bean's lifecycle is the
// responsibility of the FactoryBean, not the container.
func TestFactoryBeanProducedBeanLifecycle(t *testing.T) {

	f := &lifecycleFactory{}

	// Need a holder to trigger factory bean construction
	holder := &struct {
		Produced *lifecycleProducedBean `inject:""`
	}{}

	ctx, err := glue.New(f, holder)
	require.NoError(t, err)

	// Verify the produced bean was created
	require.NotNil(t, holder.Produced)
	require.NotNil(t, f.produced)
	require.Same(t, f.produced, holder.Produced)

	// Factory's own PostConstruct SHOULD have been called
	require.Equal(t, int32(1), atomic.LoadInt32(&f.factoryPostConstructed),
		"container should call PostConstruct on the FactoryBean itself")

	// Produced bean's PostConstruct should NOT have been called by the container
	require.Equal(t, int32(0), atomic.LoadInt32(&holder.Produced.postConstructCalled),
		"container should NOT call PostConstruct on the FactoryBean-produced bean")

	// Close the container
	err = ctx.Close()
	require.NoError(t, err)

	// Factory's own Destroy SHOULD have been called
	require.Equal(t, int32(1), atomic.LoadInt32(&f.factoryDestroyed),
		"container should call Destroy on the FactoryBean itself")

	// Produced bean's Destroy should NOT have been called by the container
	require.Equal(t, int32(0), atomic.LoadInt32(&holder.Produced.destroyCalled),
		"container should NOT call Destroy on the FactoryBean-produced bean")
}
