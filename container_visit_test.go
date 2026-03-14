/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

type zeroVisitA struct{}
type zeroVisitB struct{}

func TestVisitStateDistinguishesZeroSizedPointerTypes(t *testing.T) {
	var shared byte

	a := (*zeroVisitA)(unsafe.Pointer(&shared))
	b := (*zeroVisitB)(unsafe.Pointer(&shared))

	visited := newVisitState()

	require.False(t, visited.markVisited(a))
	require.False(t, visited.markVisited(b))
	require.True(t, visited.markVisited(a))
	require.True(t, visited.markVisited(b))
}
