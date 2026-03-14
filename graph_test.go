/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

type graphServiceA struct {
	B *graphServiceB `inject:""`
}

type graphServiceB struct {
	C *graphServiceC `inject:""`
}

type graphServiceC struct {
}

func TestGraph_BasicDependencies(t *testing.T) {
	a := &graphServiceA{}
	b := &graphServiceB{}
	c := &graphServiceC{}

	ctx, err := glue.New(a, b, c)
	require.NoError(t, err)
	defer ctx.Close()

	dot := ctx.Graph()

	require.True(t, strings.Contains(dot, "digraph glue {"))
	require.True(t, strings.Contains(dot, "rankdir=LR;"))
	require.True(t, strings.Contains(dot, "}\n"))

	// A depends on B
	require.True(t, strings.Contains(dot, "\"*glue_test.graphServiceA\" -> \"*glue_test.graphServiceB\""))
	// B depends on C
	require.True(t, strings.Contains(dot, "\"*glue_test.graphServiceB\" -> \"*glue_test.graphServiceC\""))
}

func TestGraph_EmptyContainer(t *testing.T) {
	ctx, err := glue.New()
	require.NoError(t, err)
	defer ctx.Close()

	dot := ctx.Graph()

	require.True(t, strings.Contains(dot, "digraph glue {"))
	require.True(t, strings.Contains(dot, "}\n"))
	// No dependency edges for built-in beans (container + properties have no inject deps)
	require.False(t, strings.Contains(dot, "->"))
}

type graphNamedService struct {
	glue.NamedBean
}

func (s *graphNamedService) BeanName() string {
	return "myService"
}

type graphConsumer struct {
	Service *graphNamedService `inject:""`
}

func TestGraph_NamedBeans(t *testing.T) {
	svc := &graphNamedService{}
	consumer := &graphConsumer{}

	ctx, err := glue.New(svc, consumer)
	require.NoError(t, err)
	defer ctx.Close()

	dot := ctx.Graph()

	// Consumer depends on the named service, should use qualifier name
	require.True(t, strings.Contains(dot, "\"myService\""))
}

type graphMultiDep struct {
	B *graphServiceB `inject:""`
	C *graphServiceC `inject:""`
}

func TestGraph_MultipleDependencies(t *testing.T) {
	multi := &graphMultiDep{}
	b := &graphServiceB{}
	c := &graphServiceC{}

	ctx, err := glue.New(multi, b, c)
	require.NoError(t, err)
	defer ctx.Close()

	dot := ctx.Graph()

	// multi depends on both B and C
	require.True(t, strings.Contains(dot, "\"*glue_test.graphMultiDep\" -> \"*glue_test.graphServiceB\""))
	require.True(t, strings.Contains(dot, "\"*glue_test.graphMultiDep\" -> \"*glue_test.graphServiceC\""))
	// B depends on C
	require.True(t, strings.Contains(dot, "\"*glue_test.graphServiceB\" -> \"*glue_test.graphServiceC\""))
}
