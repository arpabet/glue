/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
	"reflect"
	"testing"
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
	ctx, err := glue.New(
		reBean,
		tBean,
	)
	require.NoError(t, err)

	require.Equal(t, 1, reBean.constructed)
	require.Equal(t, 0, reBean.destroyed)
	require.True(t, tBean.ReloadableBean == reBean)

	list := ctx.Bean(reloadableBeanClass, glue.DefaultLevel)
	require.Equal(t, 1, len(list))
	require.Equal(t, reBean, list[0].Object())

	err = list[0].Reload()
	require.NoError(t, err)

	require.Equal(t, 2, reBean.constructed)
	require.Equal(t, 1, reBean.destroyed)

	ctx.Close()

	require.Equal(t, 2, reBean.constructed)
	require.Equal(t, 2, reBean.destroyed)
	require.True(t, tBean.ReloadableBean == reBean)

}
