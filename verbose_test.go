/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"go.arpabet.com/glue"
	"github.com/stretchr/testify/require"
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

