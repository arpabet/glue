/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

// Test interfaces and implementations for PrimaryBean testing
type Service interface {
	GetName() string
}

type primaryServiceImpl struct {
	name string
}

func (s *primaryServiceImpl) GetName() string {
	return s.name
}

func (s *primaryServiceImpl) IsPrimaryBean() bool {
	return true
}

type secondaryServiceImpl struct {
	name string
}

func (s *secondaryServiceImpl) GetName() string {
	return s.name
}

func (s *secondaryServiceImpl) IsPrimaryBean() bool {
	return false
}

type thirdServiceImpl struct {
	name string
}

func (s *thirdServiceImpl) GetName() string {
	return s.name
}

// consumer uses Service interface - should get primary bean by default
type consumer struct {
	Service Service `inject:""`
}

func TestPrimaryBeanInjection(t *testing.T) {
	consumerBean := &consumer{}

	ctx, err := glue.New(
		&primaryServiceImpl{name: "primary"},
		&secondaryServiceImpl{name: "secondary"},
		&thirdServiceImpl{name: "third"},
		consumerBean,
	)
	require.NoError(t, err)
	defer ctx.Close()

	// Get all Service implementations
	serviceType := reflect.TypeOf((*Service)(nil)).Elem()
	beans := ctx.Bean(serviceType, glue.DefaultSearchLevel)
	require.Equal(t, 3, len(beans))

	// Find the consumer bean
	//beans = ctx.Lookup("*glue_test.consumer", glue.DefaultSearchLevel)
	//require.Equal(t, 1, len(beans))

	// Consumer should have the primary service injected
	//consumerBean := beans[0].Object().(*consumer)
	require.Equal(t, "primary", consumerBean.Service.GetName())
}

func TestNoPrimaryBean(t *testing.T) {
	ctx, err := glue.New(
		&secondaryServiceImpl{name: "secondary"},
		&thirdServiceImpl{name: "third"},
		&consumer{},
	)
	require.Error(t, err)
	require.Nil(t, ctx)
	require.Contains(t, err.Error(), "multiple candidates")
}
