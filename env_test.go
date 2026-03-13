/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

func TestEnvPropertyResolver_DefaultMapping(t *testing.T) {
	os.Setenv("APP_DB_HOST", "localhost")
	defer os.Unsetenv("APP_DB_HOST")

	r := &glue.EnvPropertyResolver{}
	value, ok := r.GetProperty("app.db.host")
	require.True(t, ok)
	require.Equal(t, "localhost", value)
}

func TestEnvPropertyResolver_DashMapping(t *testing.T) {
	os.Setenv("APP_READ_TIMEOUT", "30s")
	defer os.Unsetenv("APP_READ_TIMEOUT")

	r := &glue.EnvPropertyResolver{}
	value, ok := r.GetProperty("app.read-timeout")
	require.True(t, ok)
	require.Equal(t, "30s", value)
}

func TestEnvPropertyResolver_WithPrefix(t *testing.T) {
	os.Setenv("MYAPP_DB_HOST", "db.example.com")
	defer os.Unsetenv("MYAPP_DB_HOST")

	r := &glue.EnvPropertyResolver{Prefix: "MYAPP"}
	value, ok := r.GetProperty("db.host")
	require.True(t, ok)
	require.Equal(t, "db.example.com", value)
}

func TestEnvPropertyResolver_Missing(t *testing.T) {
	os.Unsetenv("NONEXISTENT_KEY_12345")

	r := &glue.EnvPropertyResolver{}
	_, ok := r.GetProperty("nonexistent.key.12345")
	require.False(t, ok)
}

func TestEnvPropertyResolver_CustomKeyMapper(t *testing.T) {
	os.Setenv("custom-key", "custom-value")
	defer os.Unsetenv("custom-key")

	r := &glue.EnvPropertyResolver{
		KeyMapper: func(key string) string {
			return "custom-key"
		},
	}
	value, ok := r.GetProperty("anything")
	require.True(t, ok)
	require.Equal(t, "custom-value", value)
}

func TestEnvPropertyResolver_DefaultPriority(t *testing.T) {
	r := &glue.EnvPropertyResolver{}
	require.Equal(t, 200, r.Priority())
}

func TestEnvPropertyResolver_CustomPriority(t *testing.T) {
	r := &glue.EnvPropertyResolver{ResolverPriority: 500}
	require.Equal(t, 500, r.Priority())
}

func TestEnvPropertyResolver_OverridesFileProperties(t *testing.T) {
	os.Setenv("APP_PORT", "9090")
	defer os.Unsetenv("APP_PORT")

	type config struct {
		Port string `value:"app.port,default=8080"`
	}

	cfg := &config{}
	ctx, err := glue.New(&glue.EnvPropertyResolver{}, cfg)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "9090", cfg.Port)
}

func TestEnvPropertyResolver_FallsBackToDefault(t *testing.T) {
	os.Unsetenv("APP_PORT")

	type config struct {
		Port string `value:"app.port,default=8080"`
	}

	cfg := &config{}
	ctx, err := glue.New(&glue.EnvPropertyResolver{}, cfg)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "8080", cfg.Port)
}

func TestEnvPropertyResolver_WithPropertiesAndEnv(t *testing.T) {
	os.Setenv("APP_DB_HOST", "env-host")
	defer os.Unsetenv("APP_DB_HOST")

	type config struct {
		Host string `value:"app.db.host,default=fallback"`
	}

	props := glue.NewProperties()
	props.Set("app.db.host", "file-host")

	cfg := &config{}
	ctx, err := glue.NewWithOptions(
		[]glue.ContainerOption{glue.WithProperties(props)},
		&glue.EnvPropertyResolver{},
		cfg,
	)
	require.NoError(t, err)
	defer ctx.Close()

	// Env resolver (priority 200) wins over file properties (priority 100)
	require.Equal(t, "env-host", cfg.Host)
}
