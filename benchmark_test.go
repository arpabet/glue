/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"fmt"
	"reflect"
	"testing"

	"go.arpabet.com/glue"
)

// benchBean is a simple bean used for benchmarking.
type benchBean struct {
	Value int
}

// benchService is an interface used for interface-based lookup benchmarks.
type benchService interface {
	ID() int
}

var benchServiceClass = reflect.TypeOf((*benchService)(nil)).Elem()

// benchServiceImpl implements benchService and NamedBean.
type benchServiceImpl struct {
	id int
}

func (b *benchServiceImpl) ID() int {
	return b.id
}

func (b *benchServiceImpl) BeanName() string {
	return fmt.Sprintf("benchService-%d", b.id)
}

// benchServiceHolder collects all benchServiceImpl beans via slice injection,
// which triggers their registration in the container registry.
type benchServiceHolder struct {
	Services []*benchServiceImpl `inject:""`
}

// generatePointerBeans creates n unique pointer beans for container registration.
func generatePointerBeans(n int) []interface{} {
	beans := make([]interface{}, n)
	for i := 0; i < n; i++ {
		beans[i] = &benchBean{Value: i}
	}
	return beans
}

// generateServiceBeans creates n unique named service beans implementing benchService,
// plus a holder to trigger registry registration.
func generateServiceBeans(n int) []interface{} {
	beans := make([]interface{}, n+1)
	for i := 0; i < n; i++ {
		beans[i] = &benchServiceImpl{id: i}
	}
	beans[n] = &benchServiceHolder{}
	return beans
}

// disableVerbose turns off verbose logging and returns a restore function.
func disableVerbose() func() {
	prev := glue.Verbose(nil)
	return func() { glue.Verbose(prev) }
}

// --- Startup Benchmarks ---

func benchmarkStartupPointer(b *testing.B, count int) {
	restore := disableVerbose()
	defer restore()
	beans := generatePointerBeans(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, err := glue.New(beans...)
		if err != nil {
			b.Fatal(err)
		}
		ctx.Close()
	}
}

func benchmarkStartupInterface(b *testing.B, count int) {
	restore := disableVerbose()
	defer restore()
	beans := generateServiceBeans(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, err := glue.New(beans...)
		if err != nil {
			b.Fatal(err)
		}
		ctx.Close()
	}
}

func BenchmarkStartupPointer_100(b *testing.B)  { benchmarkStartupPointer(b, 100) }
func BenchmarkStartupPointer_1000(b *testing.B) { benchmarkStartupPointer(b, 1000) }
func BenchmarkStartupPointer_5000(b *testing.B) { benchmarkStartupPointer(b, 5000) }

func BenchmarkStartupInterface_100(b *testing.B)  { benchmarkStartupInterface(b, 100) }
func BenchmarkStartupInterface_1000(b *testing.B) { benchmarkStartupInterface(b, 1000) }
func BenchmarkStartupInterface_5000(b *testing.B) { benchmarkStartupInterface(b, 5000) }

// --- Lookup by Type Benchmarks ---

func benchmarkLookupByType(b *testing.B, count int) {
	restore := disableVerbose()
	defer restore()
	beans := generatePointerBeans(count)
	ctx, err := glue.New(beans...)
	if err != nil {
		b.Fatal(err)
	}
	defer ctx.Close()

	lookupType := reflect.TypeOf((*benchBean)(nil)) // *benchBean
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ctx.Bean(lookupType, glue.DefaultSearchLevel)
		if len(result) == 0 {
			b.Fatal("expected beans from lookup")
		}
	}
}

func BenchmarkLookupByType_100(b *testing.B)  { benchmarkLookupByType(b, 100) }
func BenchmarkLookupByType_1000(b *testing.B) { benchmarkLookupByType(b, 1000) }
func BenchmarkLookupByType_5000(b *testing.B) { benchmarkLookupByType(b, 5000) }

// --- Lookup by Interface Benchmarks ---

func benchmarkLookupByInterface(b *testing.B, count int) {
	restore := disableVerbose()
	defer restore()
	beans := generateServiceBeans(count)
	ctx, err := glue.New(beans...)
	if err != nil {
		b.Fatal(err)
	}
	defer ctx.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ctx.Bean(benchServiceClass, glue.DefaultSearchLevel)
		if len(result) == 0 {
			b.Fatal("expected beans from lookup")
		}
	}
}

func BenchmarkLookupByInterface_100(b *testing.B)  { benchmarkLookupByInterface(b, 100) }
func BenchmarkLookupByInterface_1000(b *testing.B) { benchmarkLookupByInterface(b, 1000) }
func BenchmarkLookupByInterface_5000(b *testing.B) { benchmarkLookupByInterface(b, 5000) }

// --- Lookup by Name Benchmarks ---

func benchmarkLookupByName(b *testing.B, count int) {
	restore := disableVerbose()
	defer restore()
	beans := generateServiceBeans(count)
	ctx, err := glue.New(beans...)
	if err != nil {
		b.Fatal(err)
	}
	defer ctx.Close()

	// Lookup the middle bean by name to avoid edge effects
	targetName := fmt.Sprintf("benchService-%d", count/2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ctx.Lookup(targetName, glue.DefaultSearchLevel)
		if len(result) == 0 {
			b.Fatal("expected bean from name lookup")
		}
	}
}

func BenchmarkLookupByName_100(b *testing.B)  { benchmarkLookupByName(b, 100) }
func BenchmarkLookupByName_1000(b *testing.B) { benchmarkLookupByName(b, 1000) }
func BenchmarkLookupByName_5000(b *testing.B) { benchmarkLookupByName(b, 5000) }
