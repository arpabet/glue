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

type elementX struct {
	glue.NamedBean
	name string
}

func (t *elementX) BeanName() string {
	return t.name
}

type orderedElementX struct {
	glue.NamedBean
	name string
}

func (t *orderedElementX) BeanName() string {
	return t.name
}

func (t *orderedElementX) BeanOrder() int {
	return int(t.name[0] - 'a')
}

var holderXClass = reflect.TypeOf((*holderX)(nil)) // *holderX
type holderX struct {
	Array   []*elementX `inject`
	testing *testing.T
}

var holderMapClass = reflect.TypeOf((*holderMap)(nil)) // *holderMap
type holderMap struct {
	Map     map[string]*elementX `inject`
	testing *testing.T
}

var orderedHolderXClass = reflect.TypeOf((*orderedHolderX)(nil)) // *orderedHolderX
type orderedHolderX struct {
	Array   []*orderedElementX `inject`
	testing *testing.T
}

func TestArrayRequiredByPointer(t *testing.T) {

	_, err := glue.New(
		&holderX{testing: t},
	)
	require.NotNil(t, err)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "can not find candidates"))

}

func TestMapRequiredByPointer(t *testing.T) {

	_, err := glue.New(
		&holderMap{testing: t},
	)
	require.NotNil(t, err)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "can not find candidates"))

}

func TestArrayByPointer(t *testing.T) {

	// initialization order
	ctx, err := glue.New(
		&elementX{name: "a"},
		&elementX{name: "b"},
		&elementX{name: "c"},
		&holderX{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(holderXClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))

	holder := b[0].Object().(*holderX)
	require.NotNil(t, holder.Array)
	require.Equal(t, 3, len(holder.Array))

	// preserve initialization order for non-ordered beans
	require.Equal(t, "a", holder.Array[0].name)
	require.Equal(t, "b", holder.Array[1].name)
	require.Equal(t, "c", holder.Array[2].name)

	el := ctx.Lookup("a", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "a", el[0].Object().(*elementX).BeanName())

	el = ctx.Lookup("b", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "b", el[0].Object().(*elementX).BeanName())

	el = ctx.Lookup("c", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "c", el[0].Object().(*elementX).BeanName())

}

func TestOrderedArrayByPointer(t *testing.T) {

	// initialization order
	ctx, err := glue.New(
		&orderedElementX{name: "c"},
		&orderedElementX{name: "a"},
		&orderedElementX{name: "b"},
		&orderedHolderX{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(orderedHolderXClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))

	holder := b[0].Object().(*orderedHolderX)
	require.NotNil(t, holder.Array)
	require.Equal(t, 3, len(holder.Array))

	// preserve initialization order for non-ordered beans
	require.Equal(t, "a", holder.Array[0].name)
	require.Equal(t, "b", holder.Array[1].name)
	require.Equal(t, "c", holder.Array[2].name)

	el := ctx.Lookup("a", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "a", el[0].Object().(*orderedElementX).BeanName())
	require.Equal(t, 0, el[0].Object().(*orderedElementX).BeanOrder())

	el = ctx.Lookup("b", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "b", el[0].Object().(*orderedElementX).BeanName())
	require.Equal(t, 1, el[0].Object().(*orderedElementX).BeanOrder())

	el = ctx.Lookup("c", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "c", el[0].Object().(*orderedElementX).BeanName())
	require.Equal(t, 2, el[0].Object().(*orderedElementX).BeanOrder())

}

func TestMapByPointer(t *testing.T) {

	ctx, err := glue.New(
		&elementX{name: "a"},
		&elementX{name: "b"},
		&elementX{name: "c"},
		&holderMap{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(holderMapClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))

	holder := b[0].Object().(*holderMap)
	require.NotNil(t, holder.Map)
	require.Equal(t, 3, len(holder.Map))

	require.Equal(t, "a", holder.Map["a"].BeanName())
	require.Equal(t, "b", holder.Map["b"].BeanName())
	require.Equal(t, "c", holder.Map["c"].BeanName())

	el := ctx.Lookup("a", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "a", el[0].Object().(*elementX).BeanName())

	el = ctx.Lookup("b", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "b", el[0].Object().(*elementX).BeanName())

	el = ctx.Lookup("c", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "c", el[0].Object().(*elementX).BeanName())

}

func TestMapDuplicatesByPointer(t *testing.T) {

	_, err := glue.New(
		&elementX{name: "a"},
		&elementX{name: "a"},
		&elementX{name: "b"},
		&holderMap{testing: t},
	)

	require.NotNil(t, err)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "duplicates"))

}

var ElementClass = reflect.TypeOf((*Element)(nil)).Elem()

type Element interface {
	glue.NamedBean
}

var HolderClass = reflect.TypeOf((*Holder)(nil)).Elem()

type Holder interface {
	Elements() []Element
}

type elementImpl struct {
	name string
}

func (t *elementImpl) BeanName() string {
	return t.name
}

type orderedElementImpl struct {
	name string
}

func (t *orderedElementImpl) BeanName() string {
	return t.name
}

func (t *orderedElementImpl) BeanOrder() int {
	return int(t.name[0] - 'a')
}

type holderImpl struct {
	Array   []Element `inject`
	testing *testing.T
}

func (t *holderImpl) Elements() []Element {
	require.NotNil(t.testing, t.Array)
	return t.Array
}

type holderMapImpl struct {
	Map     map[string]Element `inject`
	testing *testing.T
}

func (t *holderMapImpl) Elements() []Element {
	require.NotNil(t.testing, t.Map)
	var list []Element
	for _, value := range t.Map {
		list = append(list, value)
	}
	return list
}

func TestArrayRequiredByInterface(t *testing.T) {

	_, err := glue.New(
		&holderImpl{testing: t},
	)
	require.NotNil(t, err)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "can not find candidates"))

}

func TestMapRequiredByInterface(t *testing.T) {

	_, err := glue.New(
		&holderMapImpl{testing: t},
	)
	require.NotNil(t, err)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "can not find candidates"))

}

func TestArrayByInterface(t *testing.T) {

	// initialization order
	ctx, err := glue.New(
		&elementImpl{name: "a"},
		&elementImpl{name: "b"},
		&elementImpl{name: "c"},
		&holderImpl{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(HolderClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))
	holder := b[0].Object().(Holder)

	require.Equal(t, 3, len(holder.Elements()))

	// preserve initialization order for non-ordered beans
	require.Equal(t, "a", holder.Elements()[0].BeanName())
	require.Equal(t, "b", holder.Elements()[1].BeanName())
	require.Equal(t, "c", holder.Elements()[2].BeanName())

	el := ctx.Lookup("a", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "a", el[0].Object().(Element).BeanName())

	el = ctx.Lookup("b", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "b", el[0].Object().(Element).BeanName())

	el = ctx.Lookup("c", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "c", el[0].Object().(Element).BeanName())

}

func TestOrderedArrayByInterface(t *testing.T) {

	ctx, err := glue.New(
		&orderedElementImpl{name: "c"},
		&orderedElementImpl{name: "a"},
		&orderedElementImpl{name: "b"},
		&holderImpl{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(HolderClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))
	holder := b[0].Object().(Holder)

	require.Equal(t, 3, len(holder.Elements()))

	// use BeanOrder function to sort elements in array
	require.Equal(t, "a", holder.Elements()[0].BeanName())
	require.Equal(t, "b", holder.Elements()[1].BeanName())
	require.Equal(t, "c", holder.Elements()[2].BeanName())

	el := ctx.Lookup("a", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "a", el[0].Object().(Element).BeanName())

	el = ctx.Lookup("b", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "b", el[0].Object().(Element).BeanName())

	el = ctx.Lookup("c", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "c", el[0].Object().(Element).BeanName())

}

func TestMapByInterface(t *testing.T) {

	// initialization order
	ctx, err := glue.New(
		&elementImpl{name: "a"},
		&elementImpl{name: "b"},
		&elementImpl{name: "c"},
		&holderMapImpl{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(HolderClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))
	holder := b[0].Object().(Holder)

	require.Equal(t, 3, len(holder.Elements()))

	el := ctx.Lookup("a", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "a", el[0].Object().(Element).BeanName())

	el = ctx.Lookup("b", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "b", el[0].Object().(Element).BeanName())

	el = ctx.Lookup("c", glue.DefaultLevel)
	require.Equal(t, 1, len(el))
	require.Equal(t, "c", el[0].Object().(Element).BeanName())

}

func TestMapDuplicatesByInterface(t *testing.T) {

	// initialization order
	_, err := glue.New(
		&elementImpl{name: "a"},
		&elementImpl{name: "a"},
		&elementImpl{name: "b"},
		&holderMapImpl{testing: t},
	)

	require.NotNil(t, err)
	println(err.Error())
	require.True(t, strings.Contains(err.Error(), "duplicates"))

}

type specificHolderImpl struct {
	Array   []Element `inject:"bean=a"`
	testing *testing.T
}

func (t *specificHolderImpl) Elements() []Element {
	return t.Array
}

func TestArraySpecificByInterface(t *testing.T) {

	ctx, err := glue.New(
		&elementImpl{name: "a"},
		&elementImpl{name: "a"},
		&elementImpl{name: "b"},
		&specificHolderImpl{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(HolderClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))
	holder := b[0].Object().(Holder)

	require.Equal(t, 2, len(holder.Elements()))

}

type specificMapHolderImpl struct {
	Map     map[string]Element `inject:"bean=a"`
	testing *testing.T
}

func (t *specificMapHolderImpl) Elements() []Element {
	require.NotNil(t.testing, t.Map)
	var list []Element
	for _, value := range t.Map {
		list = append(list, value)
	}
	return list
}

func TestMapSpecificByInterface(t *testing.T) {

	ctx, err := glue.New(
		&elementImpl{name: "a"},
		&elementImpl{name: "b"},
		&elementImpl{name: "b"},
		&specificMapHolderImpl{testing: t},
	)
	require.NoError(t, err)
	defer ctx.Close()

	b := ctx.Bean(HolderClass, glue.DefaultLevel)
	require.Equal(t, 1, len(b))
	holder := b[0].Object().(Holder)

	require.Equal(t, 1, len(holder.Elements()))

}
