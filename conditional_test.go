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

type conditionalBean struct {
	condition bool
}

func (t *conditionalBean) ShouldRegisterBean() bool {
	return t.condition
}

type profileAndConditionalBean struct {
	profile   string
	condition bool
}

func (t *profileAndConditionalBean) BeanProfile() string {
	return t.profile
}

func (t *profileAndConditionalBean) ShouldRegisterBean() bool {
	return t.condition
}

func TestConditionalBeanIncluded(t *testing.T) {
	ctx, err := glue.New(
		&conditionalBean{condition: true},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ConditionalBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 1)
}

func TestConditionalBeanExcluded(t *testing.T) {
	ctx, err := glue.New(
		&conditionalBean{condition: false},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ConditionalBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 0)
}

func TestConditionalBeanMixed(t *testing.T) {
	ctx, err := glue.New(
		&conditionalBean{condition: true},
		&conditionalBean{condition: false},
		&conditionalBean{condition: true},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ConditionalBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 2)
}

func TestProfileAndConditionalBothPass(t *testing.T) {
	ctx, err := glue.NewWithProfiles([]string{"dev"},
		&profileAndConditionalBean{profile: "dev", condition: true},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ConditionalBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 1)
}

func TestProfilePassesConditionFails(t *testing.T) {
	ctx, err := glue.NewWithProfiles([]string{"dev"},
		&profileAndConditionalBean{profile: "dev", condition: false},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ConditionalBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 0)
}

func TestProfileFailsConditionSkipped(t *testing.T) {
	ctx, err := glue.NewWithProfiles([]string{"prod"},
		&profileAndConditionalBean{profile: "dev", condition: true},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ConditionalBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 0)
}
