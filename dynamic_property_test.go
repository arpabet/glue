/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

// --- service used by all dynamic-property tests ---

type dynService struct {
	// static (existing behaviour)
	Port int `value:"app.port,default=8080"`

	// func() T — panics if missing and no default
	GetHost func() string `value:"app.host,default=localhost"`

	// func() (T, error) — returns error if missing and no default
	GetSecret func() (string, error) `value:"db.password"`

	// func(context.Context) (T, error) — container-aware variant
	GetConn func(context.Context) (string, error) `value:"db.conn,default=mem://"`
}

func newDynContext(t *testing.T, props map[string]any) (glue.Container, *dynService) {
	t.Helper()
	svc := &dynService{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: props},
		svc,
	)
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })
	return ctx, svc
}

// --- tests ---

func TestDynamicProperty_StaticStillWorks(t *testing.T) {
	_, svc := newDynContext(t, map[string]any{"app.port": "9090"})
	require.Equal(t, 9090, svc.Port)
}

func TestDynamicProperty_FuncT_Default(t *testing.T) {
	_, svc := newDynContext(t, nil)
	require.Equal(t, "localhost", svc.GetHost())
}

func TestDynamicProperty_FuncT_FromProps(t *testing.T) {
	_, svc := newDynContext(t, map[string]any{"app.host": "example.com"})
	require.Equal(t, "example.com", svc.GetHost())
}

func TestDynamicProperty_FuncT_LiveUpdate(t *testing.T) {
	ctx, svc := newDynContext(t, nil)
	ctx.Properties().Set("app.host", "live.com")
	require.Equal(t, "live.com", svc.GetHost())
}

func TestDynamicProperty_FuncError_Missing(t *testing.T) {
	_, svc := newDynContext(t, nil) // db.password not set, no default
	_, err := svc.GetSecret()
	require.Error(t, err)
	require.Contains(t, err.Error(), "db.password")
}

func TestDynamicProperty_FuncError_Present(t *testing.T) {
	_, svc := newDynContext(t, map[string]any{"db.password": "***"})
	val, err := svc.GetSecret()
	require.NoError(t, err)
	require.Equal(t, "***", val)
}

func TestDynamicProperty_FuncContext_Default(t *testing.T) {
	_, svc := newDynContext(t, nil)
	val, err := svc.GetConn(context.Background())
	require.NoError(t, err)
	require.Equal(t, "mem://", val)
}

func TestDynamicProperty_FuncContext_FromProps(t *testing.T) {
	_, svc := newDynContext(t, map[string]any{"db.conn": "postgres://localhost/mydb"})
	val, err := svc.GetConn(context.Background())
	require.NoError(t, err)
	require.Equal(t, "postgres://localhost/mydb", val)
}

func TestDynamicProperty_InvalidFunc_WithParams(t *testing.T) {
	type bad struct {
		F func(string) string `value:"x"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "container.Container")
}

func TestDynamicProperty_InvalidFunc_BadSecondReturn(t *testing.T) {
	type bad struct {
		F func() (string, int) `value:"x"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "error")
}

func TestDynamicProperty_InvalidFunc_NoReturn(t *testing.T) {
	type bad struct {
		F func() `value:"x"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "1 or 2 values")
}

func TestDynamicProperty_FuncT_RequiresDefault(t *testing.T) {
	type bad struct {
		F func() string `value:"app.host"` // no default= option
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "default")
}
