/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue_test

import (
	"github.com/stretchr/testify/require"
	"github.com/schwid/glue"
	"log"
	"testing"
)

/**
Cycle dependency test of plain beans
*/

type aPlainBean struct {
	BBean *bPlainBean `inject`
}

type bPlainBean struct {
	CBean *cPlainBean `inject`
}

type cPlainBean struct {
	ABean *aPlainBean `inject:"lazy"`
}

func TestPlainBeanCycle(t *testing.T) {

	ctx, err := glue.New(
		glue.Verbose{ Log: log.Default() },
		&aPlainBean{},
		&bPlainBean{},
		&cPlainBean{},
	)
	require.NoError(t, err)
	defer ctx.Close()

}

type selfDepBean struct {
	Self *selfDepBean `inject`
}

func TestSelfDepCycle(t *testing.T) {

	self := &selfDepBean{}

	ctx, err := glue.New(
		glue.Verbose{ Log: log.Default() },
		self,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.True(t, self == self.Self)

}