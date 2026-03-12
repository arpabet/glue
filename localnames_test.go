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

// --- Test types for localNames ---

type isolatedService struct {
	Value string
}

func (t *isolatedService) BeanName() string {
	return "isolatedService"
}

type anotherIsolatedService struct {
	Value string
}

// --- Factory bean producing a named bean that nobody injects ---

type isolatedProduct struct {
	Label string
}

var isolatedProductClass = reflect.TypeOf((*isolatedProduct)(nil))

type isolatedProductFactory struct {
	glue.FactoryBean
}

func (t *isolatedProductFactory) Object() (any, error) {
	return &isolatedProduct{Label: "from-factory"}, nil
}

func (t *isolatedProductFactory) ObjectType() reflect.Type {
	return isolatedProductClass
}

func (t *isolatedProductFactory) ObjectName() string {
	return "isolatedProduct"
}

func (t *isolatedProductFactory) Singleton() bool {
	return true
}

// TestLookupNamedBeanWithoutInjection verifies that Lookup finds a NamedBean
// even when no other bean injects it.
func TestLookupNamedBeanWithoutInjection(t *testing.T) {
	svc := &isolatedService{Value: "hello"}

	ctx, err := glue.New(svc)
	require.NoError(t, err)
	defer ctx.Close()

	// Lookup by the custom bean name should work even though nobody injects this bean
	list := ctx.Lookup("isolatedService", glue.DefaultSearchLevel)
	require.Equal(t, 1, len(list))
	require.Equal(t, svc, list[0].Object())
}

// TestLookupDefaultNameWithoutInjection verifies that Lookup finds a bean by its
// default type-based name (classPtr.String()) when it has no NamedBean and is not injected.
func TestLookupDefaultNameWithoutInjection(t *testing.T) {
	svc := &anotherIsolatedService{Value: "world"}

	ctx, err := glue.New(svc)
	require.NoError(t, err)
	defer ctx.Close()

	// Default name is the classPtr string, e.g. "*glue_test.anotherIsolatedService"
	typeName := reflect.TypeOf(svc).String()
	list := ctx.Lookup(typeName, glue.DefaultSearchLevel)
	require.Equal(t, 1, len(list))
	require.Equal(t, svc, list[0].Object())
}

// TestLookupFactoryProducedBeanWithoutInjection verifies that Lookup finds a
// factory-produced bean by its ObjectName even when nobody injects it.
func TestLookupFactoryProducedBeanWithoutInjection(t *testing.T) {
	factory := &isolatedProductFactory{}

	ctx, err := glue.New(factory)
	require.NoError(t, err)
	defer ctx.Close()

	// The factory-produced bean should be findable by name
	list := ctx.Lookup("isolatedProduct", glue.DefaultSearchLevel)
	require.Equal(t, 1, len(list))

	product := list[0].Object().(*isolatedProduct)
	require.Equal(t, "from-factory", product.Label)

	// The factory itself should also be findable by its type name
	factoryTypeName := reflect.TypeOf(factory).String()
	list = ctx.Lookup(factoryTypeName, glue.DefaultSearchLevel)
	require.Equal(t, 1, len(list))
}

// TestLookupStillWorksForInjectedBeans verifies backward compatibility:
// beans that ARE injected by other beans remain findable via Lookup.
func TestLookupStillWorksForInjectedBeans(t *testing.T) {
	svc := &isolatedService{Value: "injected"}
	consumer := &struct {
		Svc *isolatedService `inject:""`
	}{}

	ctx, err := glue.New(svc, consumer)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Lookup("isolatedService", glue.DefaultSearchLevel)
	require.Equal(t, 1, len(list))
	require.Equal(t, svc, list[0].Object())
	require.Equal(t, svc, consumer.Svc)
}

// TestLookupNotFound verifies that Lookup returns empty for unknown names.
func TestLookupNotFound(t *testing.T) {
	svc := &isolatedService{Value: "exists"}

	ctx, err := glue.New(svc)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Lookup("nonExistentBean", glue.DefaultSearchLevel)
	require.Equal(t, 0, len(list))
}

// TestLookupABCWithoutHolder verifies that named beans "a", "b", "c" are all
// discoverable via Lookup even when no holder or consumer injects them.
// This is the core scenario that localNames fixes.
func TestLookupABCWithoutHolder(t *testing.T) {
	ctx, err := glue.New(
		&elementX{name: "a"},
		&elementX{name: "b"},
		&elementX{name: "c"},
	)
	require.NoError(t, err)
	defer ctx.Close()

	for _, name := range []string{"a", "b", "c"} {
		list := ctx.Lookup(name, glue.DefaultSearchLevel)
		require.Equal(t, 1, len(list), "expected to find bean %q", name)
		require.Equal(t, name, list[0].Object().(*elementX).BeanName())
	}
}

// TestLookupABCWithHolder verifies that the "a", "b", "c" pattern still works
// when a holder injects them as a slice — backward compatibility with the
// existing collection_test.go pattern.
func TestLookupABCWithHolder(t *testing.T) {
	holder := &holderX{testing: t}
	ctx, err := glue.New(
		&elementX{name: "a"},
		&elementX{name: "b"},
		&elementX{name: "c"},
		holder,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, 3, len(holder.Array))

	for _, name := range []string{"a", "b", "c"} {
		list := ctx.Lookup(name, glue.DefaultSearchLevel)
		require.Equal(t, 1, len(list), "expected to find bean %q", name)
		require.Equal(t, name, list[0].Object().(*elementX).BeanName())
	}
}
