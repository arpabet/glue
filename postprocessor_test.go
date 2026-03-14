/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

// --- interfaces and beans used by post-processor tests ---

type ppHandler interface {
	Pattern() string
}

type ppHandlerA struct{}

func (h *ppHandlerA) Pattern() string { return "/a" }

type ppHandlerB struct{}

func (h *ppHandlerB) Pattern() string { return "/b" }

// --- registrar post-processor: collects all handlers ---

type ppRegistrar struct {
	Routes map[string]ppHandler
}

func (r *ppRegistrar) PostProcessBean(bean any, name string) error {
	if h, ok := bean.(ppHandler); ok {
		r.Routes[h.Pattern()] = h
	}
	return nil
}

func TestPostProcessor_Basic(t *testing.T) {
	registrar := &ppRegistrar{Routes: make(map[string]ppHandler)}

	ctx, err := glue.New(
		&ppHandlerA{},
		&ppHandlerB{},
		registrar,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, registrar.Routes, 2)
	require.NotNil(t, registrar.Routes["/a"])
	require.NotNil(t, registrar.Routes["/b"])
}

func TestPostProcessor_NoProcessors(t *testing.T) {
	ctx, err := glue.New(&ppHandlerA{})
	require.NoError(t, err)
	defer ctx.Close()
}

// --- ordered post-processors ---

type ppLogger struct {
	log   *[]string
	order int
	label string
}

func (p *ppLogger) PostProcessBean(bean any, name string) error {
	*p.log = append(*p.log, fmt.Sprintf("%s:%s", p.label, name))
	return nil
}

func (p *ppLogger) BeanOrder() int { return p.order }

func TestPostProcessor_Ordered(t *testing.T) {
	var log []string
	p1 := &ppLogger{log: &log, order: 2, label: "second"}
	p2 := &ppLogger{log: &log, order: 1, label: "first"}

	ctx, err := glue.New(&ppHandlerA{}, p1, p2)
	require.NoError(t, err)
	defer ctx.Close()

	// "first" (order=1) should process all beans before "second" (order=2) starts.
	// core contains user beans + internal framework beans (Properties, etc.).
	// Find the boundary between first and second processor runs.
	require.True(t, len(log) >= 2, "expected at least 2 log entries")
	firstDone := false
	for _, entry := range log {
		label := extractLabel(entry)
		if label == "second" {
			firstDone = true
		}
		if firstDone {
			require.Equal(t, "second", label, "once 'second' starts, 'first' should not appear again")
		}
	}
	require.True(t, firstDone, "expected 'second' processor to run")
}

func extractLabel(entry string) string {
	for i, c := range entry {
		if c == ':' {
			return entry[:i]
		}
	}
	return entry
}

// --- error propagation ---

type ppErrorProcessor struct{}

func (p *ppErrorProcessor) PostProcessBean(bean any, name string) error {
	return fmt.Errorf("processing failed for %s", name)
}

func TestPostProcessor_Error(t *testing.T) {
	_, err := glue.New(&ppHandlerA{}, &ppErrorProcessor{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "processing failed")
}

// --- processor does not process other processors ---

type ppSelfCheckProcessor struct {
	Seen []string
}

func (p *ppSelfCheckProcessor) PostProcessBean(bean any, name string) error {
	p.Seen = append(p.Seen, name)
	return nil
}

func TestPostProcessor_SkipsSelf(t *testing.T) {
	processor := &ppSelfCheckProcessor{}

	ctx, err := glue.New(&ppHandlerA{}, processor)
	require.NoError(t, err)
	defer ctx.Close()

	// should see the handler (and internal beans), but not the processor itself
	for _, name := range processor.Seen {
		require.NotContains(t, name, "ppSelfCheckProcessor")
	}
	// must have seen at least the handler
	found := false
	for _, name := range processor.Seen {
		if strings.Contains(name, "ppHandlerA") {
			found = true
		}
	}
	require.True(t, found, "processor should have seen ppHandlerA")
}

// --- processor skips all processor beans (not just self) ---

type ppCounterProcessor struct {
	Count int
}

func (p *ppCounterProcessor) PostProcessBean(bean any, name string) error {
	p.Count++
	return nil
}

func TestPostProcessor_SkipsOtherProcessors(t *testing.T) {
	counter := &ppCounterProcessor{}
	other := &ppSelfCheckProcessor{}

	ctx, err := glue.New(&ppHandlerA{}, &ppHandlerB{}, counter, other)
	require.NoError(t, err)
	defer ctx.Close()

	// counter should see handlers + internal beans, but NOT the 2 processors
	// With 2 handlers and internal beans, count > 2 is fine as long as
	// processors are excluded. Verify by checking other processor's Seen list.
	require.GreaterOrEqual(t, counter.Count, 2, "should see at least 2 handlers")
	for _, name := range other.Seen {
		require.NotContains(t, name, "ppCounterProcessor")
		require.NotContains(t, name, "ppSelfCheckProcessor")
	}
}

// --- processor with injected dependencies ---

type ppService struct {
	Name string
}

func (s *ppService) BeanName() string { return s.Name }

type ppProcessorWithDeps struct {
	Svc *ppService `inject`
	log []string
}

func (p *ppProcessorWithDeps) PostProcessBean(bean any, name string) error {
	p.log = append(p.log, fmt.Sprintf("processed:%s:by:%s", name, p.Svc.Name))
	return nil
}

func TestPostProcessor_WithInjectedDependency(t *testing.T) {
	svc := &ppService{Name: "mySvc"}
	processor := &ppProcessorWithDeps{}

	ctx, err := glue.New(svc, &ppHandlerA{}, processor)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, processor.Svc)
	require.Equal(t, "mySvc", processor.Svc.Name)
	// processor should have processed the handler (not svc, not self)
	// Note: svc is also a bean, so processor may see it too
	require.GreaterOrEqual(t, len(processor.log), 1)
}

// --- interaction with decorators: processor sees decorated beans ---

type ppDecService interface {
	Value() string
}

type ppDecServiceImpl struct{}

func (s *ppDecServiceImpl) Value() string { return "original" }

type ppDecWrapper struct {
	delegate ppDecService
}

func (w *ppDecWrapper) Value() string { return "decorated:" + w.delegate.Value() }

type ppDecDecorator struct{}

func (d *ppDecDecorator) DecorateType() reflect.Type {
	return reflect.TypeOf((*ppDecService)(nil)).Elem()
}

func (d *ppDecDecorator) Decorate(original any) (any, error) {
	return &ppDecWrapper{delegate: original.(ppDecService)}, nil
}

type ppInspector struct {
	values []string
}

func (p *ppInspector) PostProcessBean(bean any, name string) error {
	if s, ok := bean.(ppDecService); ok {
		p.values = append(p.values, s.Value())
	}
	return nil
}

func TestPostProcessor_SeesDecoratedBeans(t *testing.T) {
	inspector := &ppInspector{}

	ctx, err := glue.New(
		&ppDecServiceImpl{},
		&ppDecDecorator{},
		inspector,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.Len(t, inspector.values, 1)
	require.Equal(t, "decorated:original", inspector.values[0])
}
