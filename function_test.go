/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue_test

import (
	"github.com/stretchr/testify/require"
	"github.com/schwid/glue"
	"reflect"
	"testing"
)

type functionHolder struct {
	Int         func() int               `inject`
	StringArray func() []string          `inject`
	SomeMap     func() map[string]string `inject`
}

func TestPrimitiveFunctions(t *testing.T) {

	glue.Verbose(true)

	holder := &functionHolder{}

	ctx, err := glue.New(
		holder,
		func() int { return 123 },
		func() []string { return []string{"a", "b"} },
		func() map[string]string { return map[string]string{"a": "b"} },
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, 123, holder.Int())

	arr := holder.StringArray()
	require.Equal(t, 2, len(arr))
	require.Equal(t, "a", arr[0])
	require.Equal(t, "b", arr[1])

	m := holder.SomeMap()
	require.Equal(t, 1, len(m))
	require.Equal(t, "b", m["a"])

}

type ClientBeans func() []interface{}

var ClientBeansClass = reflect.TypeOf((ClientBeans)(nil))

type ServerBeans func() []interface{}

var ServerBeansClass = reflect.TypeOf((ServerBeans)(nil))

type funcServiceImpl struct {
	ClientBeans ClientBeans `inject`
	ServerBeans ServerBeans `inject`
}

func TestFunctions(t *testing.T) {

	println(ClientBeansClass.String())
	println(ServerBeansClass.String())

	clientBeanInstance := &struct{}{}

	clientBeans := ClientBeans(func() []interface{} {
		println("clientBeans call")
		return []interface{}{clientBeanInstance}
	})

	serverBeans := ServerBeans(func() []interface{} {
		println("serverBeans call")
		return nil
	})

	glue.Verbose(true)

	srv := &funcServiceImpl{}

	ctx, err := glue.New(
		clientBeans,
		serverBeans,
		srv,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, srv.ClientBeans)
	require.NotNil(t, srv.ServerBeans)

	list := ctx.Bean(ClientBeansClass, glue.DefaultLevel)
	require.Equal(t, 1, len(list))
	cbs := list[0].Object().(ClientBeans)

	require.Equal(t, reflect.ValueOf(clientBeans).Pointer(), reflect.ValueOf(cbs).Pointer())

	cb := cbs()
	require.Equal(t, 1, len(cb))

	require.Equal(t, clientBeanInstance, cb[0])

	list = ctx.Bean(ServerBeansClass, glue.DefaultLevel)
	require.Equal(t, 1, len(list))
	sbs := list[0].Object().(ServerBeans)

	require.Equal(t, reflect.ValueOf(serverBeans).Pointer(), reflect.ValueOf(sbs).Pointer())
	require.Nil(t, sbs())
}
