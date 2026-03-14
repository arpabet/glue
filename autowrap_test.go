/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

type supplyDB struct {
	DSN string
}

type supplyConsumer struct {
	DB *supplyDB `inject:""`
}

func TestStructValue_AutoWrap(t *testing.T) {
	// struct value (not pointer) — container wraps it to *supplyDB automatically
	db := supplyDB{DSN: "postgres://localhost/test"}
	consumer := &supplyConsumer{}

	ctx, err := glue.New(db, consumer)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "postgres://localhost/test", consumer.DB.DSN)
}

func TestStructValue_LookupByType(t *testing.T) {
	db := supplyDB{DSN: "mysql://localhost"}

	ctx, err := glue.New(db)
	require.NoError(t, err)
	defer ctx.Close()

	beans := ctx.Bean(reflect.TypeOf((*supplyDB)(nil)), glue.SearchCurrent)
	require.Len(t, beans, 1)
	require.Equal(t, "mysql://localhost", beans[0].Object().(*supplyDB).DSN)
}

func TestStructValue_MixedWithPointers(t *testing.T) {
	db := supplyDB{DSN: "sqlite"}
	consumer := &supplyConsumer{}

	ctx, err := glue.New(db, consumer)
	require.NoError(t, err)
	defer ctx.Close()

	require.Equal(t, "sqlite", consumer.DB.DSN)
}

func TestStructValue_PointerStillWorks(t *testing.T) {
	db := &supplyDB{DSN: "pg"}
	consumer := &supplyConsumer{}

	ctx, err := glue.New(db, consumer)
	require.NoError(t, err)
	defer ctx.Close()

	require.Same(t, db, consumer.DB)
}
