/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

// --- service interface and implementation ---

type decUserService interface {
	GetUser(id string) string
}

var decUserServiceClass = reflect.TypeOf((*decUserService)(nil)).Elem()

type decUserServiceImpl struct{}

func (s *decUserServiceImpl) GetUser(id string) string {
	return "user:" + id
}

// --- logging decorator ---

type decLoggingWrapper struct {
	delegate decUserService
	calls    []string
}

func (s *decLoggingWrapper) GetUser(id string) string {
	s.calls = append(s.calls, "GetUser:"+id)
	return s.delegate.GetUser(id)
}

type decLoggingDecorator struct {
	logging *decLoggingWrapper
}

func (d *decLoggingDecorator) DecorateType() reflect.Type {
	return decUserServiceClass
}

func (d *decLoggingDecorator) Decorate(original any) (any, error) {
	d.logging = &decLoggingWrapper{delegate: original.(decUserService)}
	return d.logging, nil
}

// --- consumer ---

type decConsumer struct {
	Svc decUserService `inject:""`
}

func TestDecorator_Basic(t *testing.T) {
	dec := &decLoggingDecorator{}
	consumer := &decConsumer{}

	ctx, err := glue.New(
		&decUserServiceImpl{},
		dec,
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	result := consumer.Svc.GetUser("42")
	require.Equal(t, "user:42", result)
	require.Equal(t, []string{"GetUser:42"}, dec.logging.calls)
}

func TestDecorator_NoDecorators(t *testing.T) {
	consumer := &decConsumer{}
	ctx, err := glue.New(&decUserServiceImpl{}, consumer)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "user:99", consumer.Svc.GetUser("99"))
}

// --- ordered decorators ---

type decPrefixDecorator struct {
	prefix string
}

func (d *decPrefixDecorator) DecorateType() reflect.Type { return decUserServiceClass }
func (d *decPrefixDecorator) BeanOrder() int             { return 1 }

func (d *decPrefixDecorator) Decorate(original any) (any, error) {
	return &decPrefixWrapper{delegate: original.(decUserService), prefix: d.prefix}, nil
}

type decPrefixWrapper struct {
	delegate decUserService
	prefix   string
}

func (s *decPrefixWrapper) GetUser(id string) string {
	return s.prefix + s.delegate.GetUser(id)
}

func TestDecorator_Ordered(t *testing.T) {
	consumer := &decConsumer{}

	dec1 := &decPrefixDecorator{prefix: "[A]"}
	dec2 := &decPrefixDecorator{prefix: "[B]"}

	ctx, err := glue.New(
		&decUserServiceImpl{},
		dec1,
		dec2,
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	result := consumer.Svc.GetUser("1")
	require.Equal(t, "[B][A]user:1", result)
}

func TestDecorator_DecorateError(t *testing.T) {
	errDec := &decErrorDecorator{}

	_, err := glue.New(&decUserServiceImpl{}, errDec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "decoration failed")
}

type decErrorDecorator struct{}

func (d *decErrorDecorator) DecorateType() reflect.Type { return decUserServiceClass }
func (d *decErrorDecorator) Decorate(original any) (any, error) {
	return nil, fmt.Errorf("decoration failed")
}
