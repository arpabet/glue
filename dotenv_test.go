/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestDotEnvPropertyResolver_BasicLookup(t *testing.T) {
	path := writeDotEnv(t, "APP_DB_HOST=dotenv-host\nAPP_PORT=3000\n")

	r := &glue.DotEnvPropertyResolver{Path: path}
	value, ok := r.GetProperty("app.db.host")
	require.True(t, ok)
	require.Equal(t, "dotenv-host", value)

	value, ok = r.GetProperty("app.port")
	require.True(t, ok)
	require.Equal(t, "3000", value)
}

func TestDotEnvPropertyResolver_DashMapping(t *testing.T) {
	path := writeDotEnv(t, "APP_READ_TIMEOUT=30s\n")

	r := &glue.DotEnvPropertyResolver{Path: path}
	value, ok := r.GetProperty("app.read-timeout")
	require.True(t, ok)
	require.Equal(t, "30s", value)
}

func TestDotEnvPropertyResolver_Missing(t *testing.T) {
	path := writeDotEnv(t, "APP_PORT=3000\n")

	r := &glue.DotEnvPropertyResolver{Path: path}
	_, ok := r.GetProperty("nonexistent.key")
	require.False(t, ok)
}

func TestDotEnvPropertyResolver_CustomKeyMapper(t *testing.T) {
	path := writeDotEnv(t, "custom-key=custom-value\n")

	r := &glue.DotEnvPropertyResolver{
		Path: path,
		KeyMapper: func(key string) string {
			return "custom-key"
		},
	}
	value, ok := r.GetProperty("anything")
	require.True(t, ok)
	require.Equal(t, "custom-value", value)
}

func TestDotEnvPropertyResolver_MatchKey(t *testing.T) {
	path := writeDotEnv(t, "APP_PORT=9090\n")

	r := &glue.DotEnvPropertyResolver{
		Path:     path,
		MatchKey: glue.OnlyEnvStyle,
	}

	value, ok := r.GetProperty("APP_PORT")
	require.True(t, ok)
	require.Equal(t, "9090", value)

	_, ok = r.GetProperty("app.port")
	require.False(t, ok)
}

func TestDotEnvPropertyResolver_DefaultPriority(t *testing.T) {
	r := &glue.DotEnvPropertyResolver{}
	require.Equal(t, 300, r.Priority())
}

func TestDotEnvPropertyResolver_CustomPriority(t *testing.T) {
	r := &glue.DotEnvPropertyResolver{ResolverPriority: 500}
	require.Equal(t, 500, r.Priority())
}

func TestDotEnvPropertyResolver_QuotedValues(t *testing.T) {
	path := writeDotEnv(t, `DB_HOST="quoted-host"
DB_PASS='single-quoted'
`)
	r := &glue.DotEnvPropertyResolver{Path: path}

	value, ok := r.GetProperty("db.host")
	require.True(t, ok)
	require.Equal(t, "quoted-host", value)

	value, ok = r.GetProperty("db.pass")
	require.True(t, ok)
	require.Equal(t, "single-quoted", value)
}

func TestDotEnvPropertyResolver_CommentsAndBlanks(t *testing.T) {
	path := writeDotEnv(t, `# this is a comment
APP_PORT=3000

! another comment
APP_HOST=localhost
`)
	r := &glue.DotEnvPropertyResolver{Path: path}

	value, ok := r.GetProperty("app.port")
	require.True(t, ok)
	require.Equal(t, "3000", value)

	value, ok = r.GetProperty("app.host")
	require.True(t, ok)
	require.Equal(t, "localhost", value)
}

func TestDotEnvPropertyResolver_ExportPrefix(t *testing.T) {
	path := writeDotEnv(t, "export APP_PORT=3000\n")

	r := &glue.DotEnvPropertyResolver{Path: path}
	value, ok := r.GetProperty("app.port")
	require.True(t, ok)
	require.Equal(t, "3000", value)
}

func TestDotEnvPropertyResolver_ColonSeparator(t *testing.T) {
	path := writeDotEnv(t, "APP_PORT:3000\n")

	r := &glue.DotEnvPropertyResolver{Path: path}
	value, ok := r.GetProperty("app.port")
	require.True(t, ok)
	require.Equal(t, "3000", value)
}

func TestDotEnvPropertyResolver_MissingFile(t *testing.T) {
	r := &glue.DotEnvPropertyResolver{Path: "/nonexistent/.env"}
	_, ok := r.GetProperty("any.key")
	require.False(t, ok)
}

func TestDotEnvPropertyResolver_Keys(t *testing.T) {
	path := writeDotEnv(t, "APP_PORT=3000\nDB_HOST=localhost\n")

	r := &glue.DotEnvPropertyResolver{Path: path}
	keys := r.Keys()
	sort.Strings(keys)
	require.Equal(t, []string{"app.port", "db.host"}, keys)
}

func TestDotEnvPropertyResolver_KeysWithKeyMapper(t *testing.T) {
	path := writeDotEnv(t, "APP_PORT=3000\n")

	r := &glue.DotEnvPropertyResolver{
		Path:      path,
		KeyMapper: func(key string) string { return key },
	}
	require.Nil(t, r.Keys())
}

func TestDotEnvPropertyResolver_OverridesEnvResolver(t *testing.T) {
	os.Setenv("APP_PORT", "from-env")
	defer os.Unsetenv("APP_PORT")

	path := writeDotEnv(t, "APP_PORT=from-dotenv\n")

	type config struct {
		Port string `value:"app.port,default=8080"`
	}

	cfg := &config{}
	ctx, err := glue.New(
		&glue.DotEnvPropertyResolver{Path: path},
		&glue.EnvPropertyResolver{},
		cfg,
	)
	require.NoError(t, err)
	defer ctx.Close()

	// DotEnv (priority 300) wins over Env (priority 200)
	require.Equal(t, "from-dotenv", cfg.Port)
}
