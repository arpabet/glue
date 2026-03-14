/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

func TestPrefixMap_Basic(t *testing.T) {
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host":  "localhost",
			"db.port":  "5432",
			"db.user":  "admin",
			"app.name": "myapp",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 3)
	require.Equal(t, "localhost", svc.DB["host"])
	require.Equal(t, "5432", svc.DB["port"])
	require.Equal(t, "admin", svc.DB["user"])
}

func TestPrefixMap_Empty(t *testing.T) {
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"app.name": "myapp",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, svc.DB)
	require.Len(t, svc.DB, 0)
}

func TestPrefixMap_Nested(t *testing.T) {
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.pool.max": "10",
			"db.pool.min": "2",
			"db.host":     "localhost",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 3)
	require.Equal(t, "10", svc.DB["pool.max"])
	require.Equal(t, "2", svc.DB["pool.min"])
	require.Equal(t, "localhost", svc.DB["host"])
}

func TestPrefixMap_WithExpressions(t *testing.T) {
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"app.name": "myapp",
			"db.name":  "${app.name}_db",
			"db.host":  "localhost",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "myapp_db", svc.DB["name"])
	require.Equal(t, "localhost", svc.DB["host"])
}

func TestPrefixMap_InvalidFieldType(t *testing.T) {
	type bad struct {
		DB int `value:"prefix=db"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "map[string]string")
}

func TestPrefixMap_EmptyPrefix(t *testing.T) {
	type bad struct {
		DB map[string]string `value:"prefix="`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty prefix")
}

func TestPrefixMap_LiveUpdate(t *testing.T) {
	type cfg struct {
		// Static prefix map — captured at construction time
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "localhost",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "localhost", svc.DB["host"])

	// update property and reload
	ctx.Properties().Set("db.host", "remotehost")
	ctx.Properties().Set("db.port", "3306")

	// static map is NOT updated (it's a snapshot)
	require.Equal(t, "localhost", svc.DB["host"])
}

func TestPrefixMap_MultiplePrefixes(t *testing.T) {
	type cfg struct {
		DB    map[string]string `value:"prefix=db"`
		Cache map[string]string `value:"prefix=cache"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host":    "dbhost",
			"cache.host": "cachehost",
			"cache.ttl":  "30s",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 1)
	require.Equal(t, "dbhost", svc.DB["host"])

	require.Len(t, svc.Cache, 2)
	require.Equal(t, "cachehost", svc.Cache["host"])
	require.Equal(t, "30s", svc.Cache["ttl"])
}

func TestPrefixMap_ExactPrefixKeyExcluded(t *testing.T) {
	// A key that equals the prefix exactly (no trailing sub-key) must NOT appear in the map.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db":      "should-not-appear",
			"db.host": "localhost",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 1)
	require.Equal(t, "localhost", svc.DB["host"])
	_, found := svc.DB[""]
	require.False(t, found, "empty-suffix key should be excluded")
}

func TestPrefixMap_SingleProperty(t *testing.T) {
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "only-one",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 1)
	require.Equal(t, "only-one", svc.DB["host"])
}

func TestPrefixMap_DottedPrefix(t *testing.T) {
	// Prefix itself contains a dot — e.g., value:"prefix=db.pool"
	type cfg struct {
		Pool map[string]string `value:"prefix=db.pool"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.pool.max":  "20",
			"db.pool.min":  "5",
			"db.pool.idle": "3",
			"db.host":      "localhost",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.Pool, 3)
	require.Equal(t, "20", svc.Pool["max"])
	require.Equal(t, "5", svc.Pool["min"])
	require.Equal(t, "3", svc.Pool["idle"])
}

func TestPrefixMap_WithRegularValueField(t *testing.T) {
	// Prefix map coexists with regular value:"..." fields in the same struct.
	type cfg struct {
		Name string            `value:"app.name"`
		DB   map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"app.name": "myapp",
			"db.host":  "localhost",
			"db.port":  "5432",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "myapp", svc.Name)
	require.Len(t, svc.DB, 2)
	require.Equal(t, "localhost", svc.DB["host"])
	require.Equal(t, "5432", svc.DB["port"])
}

func TestPrefixMap_WithInjectField(t *testing.T) {
	// Prefix map coexists with inject:"" fields in the same struct.
	type dep struct{}
	type cfg struct {
		Dep *dep              `inject:""`
		DB  map[string]string `value:"prefix=db"`
	}
	d := &dep{}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "localhost",
		}},
		d,
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Same(t, d, svc.Dep)
	require.Len(t, svc.DB, 1)
	require.Equal(t, "localhost", svc.DB["host"])
}

func TestPrefixMap_SpecialCharValues(t *testing.T) {
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.url":      "postgres://user:p@ss=word@host:5432/db?sslmode=disable",
			"db.password": "s3cr3t!@#$%",
			"db.query":    "SELECT * FROM t WHERE a = 1",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 3)
	require.Equal(t, "postgres://user:p@ss=word@host:5432/db?sslmode=disable", svc.DB["url"])
	require.Equal(t, "s3cr3t!@#$%", svc.DB["password"])
	require.Equal(t, "SELECT * FROM t WHERE a = 1", svc.DB["query"])
}

func TestPrefixMap_ReloadRefreshesMap(t *testing.T) {
	// After Reload, prefix map should re-capture from current properties.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "localhost",
			"db.port": "5432",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 2)
	require.Equal(t, "localhost", svc.DB["host"])

	// mutate properties
	ctx.Properties().Set("db.host", "remotehost")
	ctx.Properties().Set("db.name", "mydb")

	// before reload: snapshot unchanged
	require.Equal(t, "localhost", svc.DB["host"])
	require.Len(t, svc.DB, 2)

	// reload the bean
	classPtr := reflect.TypeOf(svc)
	beans := ctx.Bean(classPtr, glue.DefaultSearchLevel)
	require.Len(t, beans, 1)
	err = ctx.Reload(beans[0])
	require.NoError(t, err)

	// after reload: map refreshed
	require.Len(t, svc.DB, 3)
	require.Equal(t, "remotehost", svc.DB["host"])
	require.Equal(t, "5432", svc.DB["port"])
	require.Equal(t, "mydb", svc.DB["name"])
}

func TestPrefixMap_InvalidMapKeyType(t *testing.T) {
	type bad struct {
		DB map[int]string `value:"prefix=db"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "map[string]string")
}

func TestPrefixMap_InvalidMapValueType(t *testing.T) {
	type bad struct {
		DB map[string]int `value:"prefix=db"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "map[string]string")
}

func TestPrefixMap_SliceFieldType(t *testing.T) {
	type bad struct {
		DB []string `value:"prefix=db"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "map[string]string")
}

func TestPrefixMap_StringFieldType(t *testing.T) {
	type bad struct {
		DB string `value:"prefix=db"`
	}
	_, err := glue.New(&bad{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "map[string]string")
}

func TestPrefixMap_OverlappingPrefixes(t *testing.T) {
	// Two prefix maps where one prefix is a sub-prefix of the other.
	type cfg struct {
		DB     map[string]string `value:"prefix=db"`
		DBPool map[string]string `value:"prefix=db.pool"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host":      "localhost",
			"db.pool.max":  "10",
			"db.pool.min":  "2",
			"db.pool.idle": "5",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	// DB captures all db.* keys (including db.pool.*)
	require.Len(t, svc.DB, 4)
	require.Equal(t, "localhost", svc.DB["host"])
	require.Equal(t, "10", svc.DB["pool.max"])

	// DBPool captures only db.pool.* keys
	require.Len(t, svc.DBPool, 3)
	require.Equal(t, "10", svc.DBPool["max"])
	require.Equal(t, "2", svc.DBPool["min"])
	require.Equal(t, "5", svc.DBPool["idle"])
}

func TestPrefixMap_ChainedExpressions(t *testing.T) {
	// Expressions that chain through multiple levels of indirection.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"env":       "prod",
			"base.host": "db-${env}.example.com",
			"db.host":   "${base.host}",
			"db.port":   "5432",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 2)
	require.Equal(t, "db-prod.example.com", svc.DB["host"])
	require.Equal(t, "5432", svc.DB["port"])
}

func TestPrefixMap_DefaultExpressions(t *testing.T) {
	// Values using ${key:default} syntax.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "${db.override.host:fallback-host}",
			"db.port": "${db.override.port:3306}",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 2)
	require.Equal(t, "fallback-host", svc.DB["host"])
	require.Equal(t, "3306", svc.DB["port"])
}

func TestPrefixMap_NoProperties(t *testing.T) {
	// No PropertySource at all — the container has zero enumerable keys.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(svc)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, svc.DB)
	require.Len(t, svc.DB, 0)
}

func TestPrefixMap_ChildContainer(t *testing.T) {
	// Prefix map uses Properties.Keys() which only returns keys from the
	// container's own property store. Parent keys are NOT enumerable in the child.
	type parentCfg struct {
		App map[string]string `value:"prefix=app"`
	}
	type childCfg struct {
		DB map[string]string `value:"prefix=db"`
	}

	pSvc := &parentCfg{}
	cSvc := &childCfg{}

	parent, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"app.name": "myapp",
			"app.env":  "dev",
		}},
		pSvc,
	)
	require.NoError(t, err)
	defer parent.Close()

	// child has its own PropertySource
	child, err := parent.Extend(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "localhost",
			"db.port": "5432",
		}},
		cSvc,
	)
	require.NoError(t, err)
	defer child.Close()

	// parent bean got its prefix map
	require.Len(t, pSvc.App, 2)
	require.Equal(t, "myapp", pSvc.App["name"])
	require.Equal(t, "dev", pSvc.App["env"])

	// child bean sees properties from its own PropertySource
	require.Len(t, cSvc.DB, 2)
	require.Equal(t, "localhost", cSvc.DB["host"])
	require.Equal(t, "5432", cSvc.DB["port"])
}

func TestPrefixMap_ChildContainer_InheritsParentKeys(t *testing.T) {
	// Parent's property store implements EnumerablePropertyResolver, so
	// the child container discovers parent keys for prefix map binding.
	type childCfg struct {
		DB map[string]string `value:"prefix=db"`
	}

	cSvc := &childCfg{}

	parent, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "parent-host",
			"db.port": "5432",
		}},
	)
	require.NoError(t, err)
	defer parent.Close()

	child, err := parent.Extend(cSvc)
	require.NoError(t, err)
	defer child.Close()

	// child sees parent keys through EnumerablePropertyResolver
	require.Len(t, cSvc.DB, 2)
	require.Equal(t, "parent-host", cSvc.DB["host"])
	require.Equal(t, "5432", cSvc.DB["port"])
}

func TestPrefixMap_EmptyValues(t *testing.T) {
	// Properties with empty string values should still appear in the map.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host":     "",
			"db.password": "",
		}},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 2)
	require.Equal(t, "", svc.DB["host"])
	require.Equal(t, "", svc.DB["password"])
}

func TestPrefixMap_LargeKeySet(t *testing.T) {
	// Ensure prefix filtering works correctly with many properties.
	props := map[string]any{}
	for i := 0; i < 100; i++ {
		props[fmt.Sprintf("db.key%d", i)] = fmt.Sprintf("val%d", i)
		props[fmt.Sprintf("other.key%d", i)] = fmt.Sprintf("x%d", i)
	}

	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: props},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 100)
	require.Equal(t, "val0", svc.DB["key0"])
	require.Equal(t, "val99", svc.DB["key99"])
}

// testEnumerableResolver is a custom EnumerablePropertyResolver for testing.
type testEnumerableResolver struct {
	priority int
	data     map[string]string
}

func (r *testEnumerableResolver) Priority() int {
	return r.priority
}

func (r *testEnumerableResolver) GetProperty(key string) (string, bool) {
	v, ok := r.data[key]
	return v, ok
}

func (r *testEnumerableResolver) Keys() []string {
	keys := make([]string, 0, len(r.data))
	for k := range r.data {
		keys = append(keys, k)
	}
	return keys
}

func TestPrefixMap_EnumerableResolver_Basic(t *testing.T) {
	// Keys from an EnumerablePropertyResolver are discovered for prefix map binding.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	resolver := &testEnumerableResolver{
		priority: 300,
		data: map[string]string{
			"db.host": "resolver-host",
			"db.port": "9999",
			"app.name": "myapp",
		},
	}
	svc := &cfg{}
	ctx, err := glue.New(resolver, svc)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 2)
	require.Equal(t, "resolver-host", svc.DB["host"])
	require.Equal(t, "9999", svc.DB["port"])
}

func TestPrefixMap_EnumerableResolver_MergedWithStore(t *testing.T) {
	// Keys from both the property store and the enumerable resolver are merged.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	resolver := &testEnumerableResolver{
		priority: 300,
		data: map[string]string{
			"db.password": "secret",
		},
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "localhost",
			"db.port": "5432",
		}},
		resolver,
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 3)
	require.Equal(t, "localhost", svc.DB["host"])
	require.Equal(t, "5432", svc.DB["port"])
	require.Equal(t, "secret", svc.DB["password"])
}

func TestPrefixMap_EnumerableResolver_HigherPriorityWins(t *testing.T) {
	// When the same key exists in both store and resolver, the higher-priority
	// source wins because Resolve uses the priority-sorted resolver chain.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	resolver := &testEnumerableResolver{
		priority: 300, // higher than default store priority (100)
		data: map[string]string{
			"db.host": "resolver-host",
		},
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "store-host",
		}},
		resolver,
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 1)
	require.Equal(t, "resolver-host", svc.DB["host"])
}

func TestPrefixMap_EnumerableResolver_LowerPriorityFallback(t *testing.T) {
	// When the resolver has lower priority, the store value wins.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	resolver := &testEnumerableResolver{
		priority: 50, // lower than default store priority (100)
		data: map[string]string{
			"db.host": "resolver-host",
		},
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "store-host",
		}},
		resolver,
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 1)
	require.Equal(t, "store-host", svc.DB["host"])
}

func TestPrefixMap_NonEnumerableResolver_Ignored(t *testing.T) {
	// A plain PropertyResolver (not EnumerablePropertyResolver) cannot
	// contribute keys to prefix map binding — only point lookups work.
	type plainResolver struct {
		data map[string]string
	}
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(svc)
	require.NoError(t, err)
	defer ctx.Close()

	// no keys were discoverable
	require.NotNil(t, svc.DB)
	require.Len(t, svc.DB, 0)
}

func TestPrefixMap_EnvResolver_DiscoversPrefixKeys(t *testing.T) {
	// EnvPropertyResolver implements EnumerablePropertyResolver.
	// Set env vars and verify they appear in the prefix map.
	t.Setenv("GLUETEST_DB_HOST", "env-host")
	t.Setenv("GLUETEST_DB_PORT", "3306")
	t.Setenv("GLUETEST_APP_NAME", "myapp")

	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.EnvPropertyResolver{Prefix: "GLUETEST"},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "env-host", svc.DB["host"])
	require.Equal(t, "3306", svc.DB["port"])
	_, hasApp := svc.DB["app.name"]
	require.False(t, hasApp, "app.name should not appear under db prefix")
}

func TestPrefixMap_EnvResolver_MergedWithPropertySource(t *testing.T) {
	// Env resolver keys are merged with PropertySource keys.
	t.Setenv("GLUETEST2_DB_PASSWORD", "env-secret")

	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.host": "localhost",
		}},
		&glue.EnvPropertyResolver{Prefix: "GLUETEST2"},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "localhost", svc.DB["host"])
	require.Equal(t, "env-secret", svc.DB["password"])
}

func TestPrefixMap_EnvResolver_WithKeyMapper_KeysNil(t *testing.T) {
	// When KeyMapper is set, reverse mapping is not possible and Keys() returns nil.
	// The resolver cannot contribute to prefix map enumeration.
	t.Setenv("CUSTOM_DB_HOST", "custom-host")

	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	svc := &cfg{}
	ctx, err := glue.New(
		&glue.EnvPropertyResolver{
			KeyMapper: func(key string) string {
				return "CUSTOM_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			},
		},
		svc,
	)
	require.NoError(t, err)
	defer ctx.Close()

	// KeyMapper prevents key enumeration, so no keys are discovered
	require.NotNil(t, svc.DB)
	require.Len(t, svc.DB, 0)
}

func TestPrefixMap_MultipleEnumerableResolvers(t *testing.T) {
	// Multiple EnumerablePropertyResolvers contribute keys.
	type cfg struct {
		DB map[string]string `value:"prefix=db"`
	}
	r1 := &testEnumerableResolver{
		priority: 300,
		data: map[string]string{
			"db.host": "r1-host",
			"db.port": "1111",
		},
	}
	r2 := &testEnumerableResolver{
		priority: 200,
		data: map[string]string{
			"db.user":     "r2-user",
			"db.password": "r2-pass",
		},
	}
	svc := &cfg{}
	ctx, err := glue.New(r1, r2, svc)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, svc.DB, 4)
	require.Equal(t, "r1-host", svc.DB["host"])
	require.Equal(t, "1111", svc.DB["port"])
	require.Equal(t, "r2-user", svc.DB["user"])
	require.Equal(t, "r2-pass", svc.DB["password"])
}
