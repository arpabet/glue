/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
	"reflect"
	"testing"
)

type beanA struct {
}

var BeanBClass = reflect.TypeOf((*beanB)(nil)) // *beanB
type beanB struct {
	BeanA   *beanA `inject:"optional"`
	testing *testing.T
}

func TestOptionalBeanByPointer(t *testing.T) {

	ctx, err := glue.New(
		&beanB{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(BeanBClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))

	require.Nil(t, b[0].Object().(*beanB).BeanA)
}

var BeanAServiceClass = reflect.TypeOf((*BeanAService)(nil)).Elem()

type BeanAService interface {
	A()
}

var BeanBServiceClass = reflect.TypeOf((*BeanBService)(nil)).Elem()

type BeanBService interface {
	B()
}

type beanBServiceImpl struct {
	BeanAService BeanAService `inject:"optional"`
	testing      *testing.T
}

func (t *beanBServiceImpl) B() {
}

func TestOptionalBeanByInterface(t *testing.T) {

	ctx, err := glue.New(
		&beanBServiceImpl{testing: t},
		&struct {
			BeanBService BeanBService `inject:""`
		}{},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(BeanBServiceClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))

	require.Nil(t, b[0].Object().(*beanBServiceImpl).BeanAService)
}
