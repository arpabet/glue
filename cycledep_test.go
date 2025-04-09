/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
	"testing"
)

/**
Cycle dependency test of plain beans
*/

type aPlainBean struct {
	BBean *bPlainBean `inject:""`
}

type bPlainBean struct {
	CBean *cPlainBean `inject:""`
}

type cPlainBean struct {
	ABean *aPlainBean `inject:"lazy"`
}

func TestPlainBeanCycle(t *testing.T) {

	ctx, err := glue.New(
		&aPlainBean{},
		&bPlainBean{},
		&cPlainBean{},
	)
	require.NoError(t, err)
	defer ctx.Close()

}

type selfDepBean struct {
	Self *selfDepBean `inject:""`
}

func TestSelfDepCycle(t *testing.T) {

	self := &selfDepBean{}

	ctx, err := glue.New(
		self,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.True(t, self == self.Self)

}
