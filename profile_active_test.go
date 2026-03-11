/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsProfileActive(t *testing.T) {
	active := map[string]struct{}{"dev": {}, "local": {}}

	require.False(t, isProfileActive(nil, ""))
	require.False(t, isProfileActive(active, ""))

	require.True(t, isProfileActive(nil, "*"))
	require.True(t, isProfileActive(active, "*"))

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
