/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type profiledBean struct {
	profile string
}

func (t *profiledBean) BeanProfile() string {
	return t.profile
}

func TestIsProfileActive(t *testing.T) {
	active := map[string]struct{}{"dev": {}, "local": {}}

	require.True(t, isProfileActive(nil, ""))
	require.True(t, isProfileActive(active, ""))

	require.True(t, isProfileActive(active, "dev"))
	require.False(t, isProfileActive(active, "prod"))

	require.True(t, isProfileActive(active, "!prod"))
	require.False(t, isProfileActive(active, "!dev"))

	require.True(t, isProfileActive(active, "dev|prod"))
	require.False(t, isProfileActive(active, "prod|staging"))

	require.True(t, isProfileActive(active, "dev&local"))
	require.False(t, isProfileActive(active, "dev&prod"))

	require.True(t, isProfileActive(active, "prod|dev&local"))
	require.False(t, isProfileActive(active, "prod|dev&!local"))

	require.True(t, isProfileActive(nil, "!prod"))
	require.False(t, isProfileActive(nil, "prod"))
}

func TestIfProfile(t *testing.T) {
	props := NewProperties()
	props.Set(ActiveProfilesProperty, "dev")

	ctn, err := createContainer(nil, ContainerOptions{
		Context:    context.Background(),
		Properties: props,
	}, []interface{}{
		IfProfile("dev", &profiledBean{profile: "dev"}),
		IfProfile("prod", &profiledBean{profile: "prod"}),
	})
	require.NoError(t, err)
	defer ctn.Close()

	list := ctn.Bean(ProfileBeanClass, DefaultLevel)
	require.Len(t, list, 1)
}

func TestProfileBeanFiltering(t *testing.T) {
	props := NewProperties()
	props.Set(ActiveProfilesProperty, "dev,local")

	ctn, err := createContainer(nil, ContainerOptions{
		Context:    context.Background(),
		Properties: props,
	}, []interface{}{
		&profiledBean{profile: "dev"},
		&profiledBean{profile: "prod"},
		&profiledBean{profile: "dev&local"},
		&profiledBean{profile: "!prod"},
	})
	require.NoError(t, err)
	defer ctn.Close()

	list := ctn.Bean(ProfileBeanClass, DefaultLevel)
	require.Len(t, list, 3)
}
