/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

type exprService struct {
	LogDir    string        `value:"app.log.dir"`
	Port      int           `value:"app.port"`
	GetLogDir func() string `value:"app.log.dir,default=/tmp/${app.name}"`
}

func TestPropertyExpressionsResolveAndKeepRawGet(t *testing.T) {
	props := glue.NewProperties()
	props.Set("app.name", "myapp")
	props.Set("app.log.dir", "/var/log/${app.name}")
	props.Set("app.port", "${APP_PORT:8080}")

	raw, ok := props.Get("app.log.dir")
	require.True(t, ok)
	require.Equal(t, "/var/log/${app.name}", raw)

	logDir, ok, err := props.Resolve("app.log.dir")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "/var/log/myapp", logDir)

	port, ok, err := props.Resolve("app.port")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "8080", port)

	text, err := props.ResolveText("dir=${app.log.dir};port=${app.port}")
	require.NoError(t, err)
	require.Equal(t, "dir=/var/log/myapp;port=8080", text)

	require.Equal(t, "/var/log/myapp", props.GetString("app.log.dir", ""))
	require.Equal(t, 8080, props.GetInt("app.port", 0))
}

func TestPropertyExpressionsDetectCycles(t *testing.T) {
	props := glue.NewProperties()
	props.Set("a", "${b}")
	props.Set("b", "${c}")
	props.Set("c", "${a}")

	_, _, err := props.Resolve("a")
	require.Error(t, err)
	require.Contains(t, err.Error(), "circular property reference")
}

func TestPropertyExpressionsDriveStaticAndDynamicValueInjection(t *testing.T) {
	svc := &exprService{}

	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"app.name":    "myapp",
			"app.log.dir": "/var/log/${app.name}",
			"app.port":    "${APP_PORT:8080}",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "/var/log/myapp", svc.LogDir)
	require.Equal(t, 8080, svc.Port)
	require.Equal(t, "/var/log/myapp", svc.GetLogDir())

	ctx.Properties().Set("app.name", "other")
	require.Equal(t, "/var/log/other", svc.GetLogDir())

	port, err := glue.GetProperty[int](ctx, "app.port")
	require.NoError(t, err)
	require.Equal(t, 8080, port)
}
