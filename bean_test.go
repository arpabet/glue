/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
	"reflect"
	"strings"
	"testing"
)

var FirstBeanClass = reflect.TypeOf((*firstBean)(nil)) // *firstBean
type firstBean struct {
}

var SecondBeanClass = reflect.TypeOf((*secondBean)(nil)) // *secondBean
type secondBean struct {
	FirstBean *firstBean `inject:"-"`
	testing   *testing.T
}

func (t *secondBean) Run() {
	require.NotNil(t.testing, t.FirstBean)
}

func TestBeanByPointer(t *testing.T) {

	ctx, err := glue.New(
		&firstBean{},
		&secondBean{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	second := ctx.Bean(SecondBeanClass, glue.DefaultLevel)
	require.Equal(t, 1, len(second))

	second[0].Object().(*secondBean).Run()

}

func TestMultipleBeanByPointer(t *testing.T) {

	ctx, err := glue.New(
		&firstBean{},
		&firstBean{},
		&secondBean{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	// Now glue check if repeated instances added to the context
	// if so, it would add only unique instance, therefore we would have
	// context with first and second beans only and no error.

	first := ctx.Bean(FirstBeanClass, glue.DefaultLevel)
	require.Equal(t, 1, len(first))

	second := ctx.Bean(SecondBeanClass, glue.DefaultLevel)
	require.Equal(t, 1, len(second))

}

func TestSearchBeanByPointerNotFound(t *testing.T) {

	ctx, err := glue.New(
		&firstBean{},
	)
	require.NoError(t, err)
	defer ctx.Close()

	second := ctx.Bean(SecondBeanClass, glue.DefaultLevel)
	require.Equal(t, 0, len(second))

}

func TestBeanByStruct(t *testing.T) {

	ctx, err := glue.New(
		firstBean{},
		&secondBean{testing: t},
	)
	require.Error(t, err)
	require.Nil(t, ctx)
	require.True(t, strings.Contains(err.Error(), "could be a pointer or function"))

}

var FirstServiceClass = reflect.TypeOf((*FirstService)(nil)).Elem()

type FirstService interface {
	First()
}

var SecondServiceClass = reflect.TypeOf((*SecondService)(nil)).Elem()

type SecondService interface {
	Second()
}

type firstServiceImpl struct {
	testing *testing.T
}

func (t *firstServiceImpl) First() {
	require.True(t.testing, true)
}

type secondServiceImpl struct {
	FirstService FirstService `inject:""`
	testing      *testing.T
}

func (t *secondServiceImpl) Second() {
	require.NotNil(t.testing, t.FirstService)
}

func TestBeanByInterface(t *testing.T) {

	ctx, err := glue.New(
		&firstServiceImpl{testing: t},
		&secondServiceImpl{testing: t},

		&struct {
			FirstService  FirstService  `inject:""`
			SecondService SecondService `inject:""`
		}{},
	)

	require.NoError(t, err)
	defer ctx.Close()

	firstService := ctx.Bean(FirstServiceClass, glue.DefaultLevel)
	require.Equal(t, 1, len(firstService))

	firstService[0].Object().(FirstService).First()

	secondService := ctx.Bean(SecondServiceClass, glue.DefaultLevel)
	require.Equal(t, 1, len(secondService))

	secondService[0].Object().(SecondService).Second()

}

type firstService2Impl struct {
	testing *testing.T
}

func (t *firstService2Impl) First() {
	require.True(t.testing, true)
}

func TestMultipleBeansByInterface(t *testing.T) {

	ctx, err := glue.New(
		&firstServiceImpl{testing: t},
		&firstService2Impl{testing: t},

		&struct {
			FirstService FirstService `inject:"-"`
		}{},
	)

	require.Error(t, err)
	require.Nil(t, ctx)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "multiple candidates"))

}

func TestSpecificBeanByInterface(t *testing.T) {

	ctx, err := glue.New(
		&firstServiceImpl{testing: t},
		&firstService2Impl{testing: t},

		&struct {
			FirstService FirstService `inject:"bean=*glue_test.firstServiceImpl"`
		}{},
	)

	require.NoError(t, err)
	defer ctx.Close()

	firstService := ctx.Bean(FirstServiceClass, glue.DefaultLevel)
	require.Equal(t, 2, len(firstService))

	firstService[0].Object().(FirstService).First()

}

func TestNotFoundSpecificBeanByInterface(t *testing.T) {

	ctx, err := glue.New(
		&firstServiceImpl{testing: t},
		&firstService2Impl{testing: t},

		&struct {
			FirstService FirstService `inject:"bean=*glue_test.unknownBean"`
		}{},
	)

	require.Error(t, err)
	require.Nil(t, ctx)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "can not find candidates"))

}
