/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
	"log"
	"testing"
)

func init() {
	glue.Verbose(log.Default())
}

func TestVerbose(t *testing.T) {
	prev := glue.Verbose(log.Default())
	require.NotNil(t, prev)
}
