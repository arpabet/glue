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

type profiledBean struct {
	profile string
}

func (t *profiledBean) BeanProfile() string {
	return t.profile
}

func TestIfProfile(t *testing.T) {
	ctx, err := glue.NewWithProfiles([]string{"dev"},
		glue.IfProfile("dev", &profiledBean{profile: "dev"}),
		glue.IfProfile("prod", &profiledBean{profile: "prod"}),
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ProfileBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 1)
}

func TestProfileBeanFiltering(t *testing.T) {
	ctx, err := glue.NewWithProfiles([]string{"dev", "local"},
		&profiledBean{profile: "dev"},
		&profiledBean{profile: "prod"},
		&profiledBean{profile: "dev&local"},
		&profiledBean{profile: "!prod"},
	)
	require.NoError(t, err)
	defer ctx.Close()

	list := ctx.Bean(glue.ProfileBeanClass, glue.DefaultSearchLevel)
	require.Len(t, list, 3)
}
