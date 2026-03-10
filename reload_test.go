/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

var reloadableBeanClass = reflect.TypeOf((*reloadableBean)(nil))

type reloadableBean struct {
	constructed int
	destroyed   int
}

func (t *reloadableBean) PostConstruct() error {
	t.constructed++
	return nil
}

func (t *reloadableBean) Destroy() error {
	t.destroyed++
	return nil
}

type topBean struct {
	ReloadableBean *reloadableBean `inject:""`
}

func TestBeanReload(t *testing.T) {

	reBean := &reloadableBean{}
	tBean := &topBean{}

	// initialization order
	ctn, err := glue.New(
		reBean,
		tBean,
	)
	require.NoError(t, err)

	require.Equal(t, 1, reBean.constructed)
	require.Equal(t, 0, reBean.destroyed)
	require.True(t, tBean.ReloadableBean == reBean)

	list := ctn.Bean(reloadableBeanClass, glue.DefaultLevel)
	require.Equal(t, 1, len(list))
	require.Equal(t, reBean, list[0].Object())

	err = ctn.Reload(list[0])
	require.NoError(t, err)

	require.Equal(t, 2, reBean.constructed)
	require.Equal(t, 1, reBean.destroyed)

	ctn.Close()

	require.Equal(t, 2, reBean.constructed)
	require.Equal(t, 2, reBean.destroyed)
	require.True(t, tBean.ReloadableBean == reBean)

}

// --- reload with property re-resolution ---

var configBeanClass = reflect.TypeOf((*configBean)(nil))

type configBean struct {
	URL     string        `value:"db.url,default=localhost"`
	Dynamic func() string `value:"db.url,default=localhost"`

	constructed int
	lastURL     string
}

func (t *configBean) PostConstruct() error {
	t.constructed++
	t.lastURL = t.URL
	return nil
}

func TestReload_PropertyReResolution(t *testing.T) {
	b := &configBean{}
	ctn, err := glue.New(
		&glue.PropertySource{Map: map[string]interface{}{"db.url": "pg://old"}},
		b,
	)
	require.NoError(t, err)
	defer ctn.Close()

	require.Equal(t, 1, b.constructed)
	require.Equal(t, "pg://old", b.URL)
	require.Equal(t, "pg://old", b.lastURL)
	require.Equal(t, "pg://old", b.Dynamic())

	// change property
	ctn.Properties().Set("db.url", "pg://new")

	// dynamic func already sees the new value
	require.Equal(t, "pg://new", b.Dynamic())
	// static field still has old value
	require.Equal(t, "pg://old", b.URL)

	// reload the bean
	list := ctn.Bean(configBeanClass, glue.DefaultLevel)
	require.Equal(t, 1, len(list))
	err = ctn.Reload(list[0])
	require.NoError(t, err)

	require.Equal(t, 2, b.constructed)
	require.Equal(t, "pg://new", b.URL, "static value: field should be re-resolved on reload")
	require.Equal(t, "pg://new", b.lastURL, "PostConstruct should see updated URL")
}

func TestReloadWithContext_PropertyReResolution(t *testing.T) {
	b := &contextAwareConfigBean{}
	ctn, err := glue.New(
		&glue.PropertySource{Map: map[string]interface{}{"app.name": "v1"}},
		b,
	)
	require.NoError(t, err)
	defer ctn.Close()

	require.Equal(t, "v1", b.Name)

	ctn.Properties().Set("app.name", "v2")

	ctx := context.WithValue(context.Background(), contextKey("reload"), "yes")
	list := ctn.Bean(reflect.TypeOf((*contextAwareConfigBean)(nil)), glue.DefaultLevel)
	err = ctn.ReloadWithContext(ctx, list[0])
	require.NoError(t, err)

	require.Equal(t, "v2", b.Name, "static value: should be re-resolved")
	require.Equal(t, "yes", b.lastCtx.Value(contextKey("reload")), "context should propagate")
}

type contextAwareConfigBean struct {
	Name    string `value:"app.name"`
	lastCtx context.Context
}

func (t *contextAwareConfigBean) PostConstruct(ctx context.Context) error {
	t.lastCtx = ctx
	return nil
}

func TestReload_DefaultUsedWhenPropertyRemoved(t *testing.T) {
	b := &configBean{}
	ctn, err := glue.New(
		&glue.PropertySource{Map: map[string]interface{}{"db.url": "pg://set"}},
		b,
	)
	require.NoError(t, err)
	defer ctn.Close()

	require.Equal(t, "pg://set", b.URL)

	// remove property — reload should fall back to default
	ctn.Properties().Remove("db.url")

	list := ctn.Bean(configBeanClass, glue.DefaultLevel)
	err = ctn.Reload(list[0])
	require.NoError(t, err)

	require.Equal(t, "localhost", b.URL, "should fall back to default value after property removed")
}

func TestReload_InjectFieldsUntouched(t *testing.T) {
	dep := &reloadableBean{}
	holder := &injectHolder{}
	ctn, err := glue.New(dep, holder)
	require.NoError(t, err)
	defer ctn.Close()

	require.True(t, holder.Dep == dep, "should be same pointer")

	list := ctn.Bean(reflect.TypeOf((*injectHolder)(nil)), glue.DefaultLevel)
	err = ctn.Reload(list[0])
	require.NoError(t, err)

	require.True(t, holder.Dep == dep, "inject field should remain same pointer after reload")
}

type injectHolder struct {
	Dep *reloadableBean `inject:""`
}

func (t *injectHolder) PostConstruct() error { return nil }
