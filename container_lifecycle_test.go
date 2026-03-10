/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

// --- test beans ---

// contextAwareBean implements only the context-aware interfaces
type contextAwareBean struct {
	constructCtx context.Context
	destroyCtx   context.Context
	constructed  bool
	destroyed    bool
}

func (t *contextAwareBean) PostConstruct(ctx context.Context) error {
	t.constructCtx = ctx
	t.constructed = true
	return nil
}

func (t *contextAwareBean) Destroy(ctx context.Context) error {
	t.destroyCtx = ctx
	t.destroyed = true
	return nil
}

// legacyBean implements only the non-context interfaces
type legacyBean struct {
	constructed bool
	destroyed   bool
}

func (t *legacyBean) PostConstruct() error {
	t.constructed = true
	return nil
}

func (t *legacyBean) Destroy() error {
	t.destroyed = true
	return nil
}

// dualBean implements both context-aware and legacy interfaces
type dualBean struct {
	legacyCalled      bool
	contextCalled     bool
	legacyDestroyed   bool
	contextDestroyed  bool
}

func (t *dualBean) PostConstruct(ctx context.Context) error {
	t.contextCalled = true
	return nil
}

// This shadows the interface but Go resolves to the correct one via type assertion
func (t *dualBean) postConstructLegacy() error {
	t.legacyCalled = true
	return nil
}

func (t *dualBean) Destroy(ctx context.Context) error {
	t.contextDestroyed = true
	return nil
}

// --- tests ---

func TestContextInitializingBean(t *testing.T) {
	b := &contextAwareBean{}
	ctx := context.WithValue(context.Background(), contextKey("test"), "init-value")

	ctn, err := glue.NewWithContext(ctx, b)
	require.NoError(t, err)
	defer ctn.Close()

	require.True(t, b.constructed)
	require.Equal(t, "init-value", b.constructCtx.Value(contextKey("test")))
}

func TestContextDisposableBean(t *testing.T) {
	b := &contextAwareBean{}

	ctn, err := glue.New(b)
	require.NoError(t, err)

	require.True(t, b.constructed)
	require.False(t, b.destroyed)

	destroyCtx := context.WithValue(context.Background(), contextKey("test"), "destroy-value")
	err = ctn.CloseWithContext(destroyCtx)
	require.NoError(t, err)

	require.True(t, b.destroyed)
	require.Equal(t, "destroy-value", b.destroyCtx.Value(contextKey("test")))
}

func TestContextDisposableBean_CloseWithoutContext(t *testing.T) {
	b := &contextAwareBean{}

	ctn, err := glue.New(b)
	require.NoError(t, err)

	err = ctn.Close()
	require.NoError(t, err)
	require.True(t, b.destroyed, "ContextDisposableBean must be destroyed via Close()")
}

func TestLegacyBean_StillWorks(t *testing.T) {
	b := &legacyBean{}

	ctn, err := glue.New(b)
	require.NoError(t, err)

	require.True(t, b.constructed)
	require.False(t, b.destroyed)

	err = ctn.Close()
	require.NoError(t, err)

	require.True(t, b.destroyed)
}

func TestContextAwareTakesPrecedence(t *testing.T) {
	b := &dualBean{}

	ctn, err := glue.New(b)
	require.NoError(t, err)
	defer ctn.Close()

	require.True(t, b.contextCalled, "ContextInitializingBean should take precedence")
	require.False(t, b.legacyCalled, "legacy PostConstruct should not be called")
}

func TestPostConstruct_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	b := &cancelCheckBean{}
	_, err := glue.NewWithContext(ctx, b)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "context canceled"))
}

type cancelCheckBean struct{}

func (t *cancelCheckBean) PostConstruct(ctx context.Context) error {
	return ctx.Err()
}

func TestPostConstruct_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	b := &slowBean{}
	_, err := glue.NewWithContext(ctx, b)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "deadline exceeded"))
}

type slowBean struct{}

func (t *slowBean) PostConstruct(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return nil
	}
}

func TestCloseWithContext_PropagatedToChildren(t *testing.T) {
	child := &contextAwareBean{}

	ctn, err := glue.New(
		glue.Child("sub", child),
	)
	require.NoError(t, err)

	// trigger lazy child creation
	children := ctn.Children()
	require.Equal(t, 1, len(children))
	_, err = children[0].Object()
	require.NoError(t, err)

	require.True(t, child.constructed)

	destroyCtx := context.WithValue(context.Background(), contextKey("test"), "child-destroy")
	err = ctn.CloseWithContext(destroyCtx)
	require.NoError(t, err)

	require.True(t, child.destroyed)
	require.Equal(t, "child-destroy", child.destroyCtx.Value(contextKey("test")))
}

func TestNewWithContext_Background(t *testing.T) {
	b := &contextAwareBean{}
	ctn, err := glue.New(b)
	require.NoError(t, err)
	defer ctn.Close()

	require.True(t, b.constructed)
	require.NotNil(t, b.constructCtx, "New() should pass context.Background()")
}

func TestPostConstructError_ContextAware(t *testing.T) {
	b := &errorContextBean{err: fmt.Errorf("init failed")}
	ctn, err := glue.New(b)
	require.Error(t, err)
	require.Nil(t, ctn)
	require.Contains(t, err.Error(), "init failed")
}

type errorContextBean struct {
	err error
}

func (t *errorContextBean) PostConstruct(ctx context.Context) error {
	return t.err
}

func TestDestroyError_ContextAware(t *testing.T) {
	b := &errorDestroyContextBean{}
	ctn, err := glue.New(b)
	require.NoError(t, err)

	err = ctn.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "destroy failed")
}

type errorDestroyContextBean struct{}

func (t *errorDestroyContextBean) PostConstruct(ctx context.Context) error { return nil }
func (t *errorDestroyContextBean) Destroy(ctx context.Context) error {
	return fmt.Errorf("destroy failed")
}

// contextKey avoids collisions with other packages
type contextKey string
