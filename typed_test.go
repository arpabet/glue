//go:build go1.18

/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

type service interface {
	Do() string
}

type serviceImpl struct{}

func (serviceImpl) Do() string { return "ok" }

type prototypePayload struct {
	Value  int32
	closed int32
}

func (p *prototypePayload) Close() error {
	atomic.StoreInt32(&p.closed, 1)
	return nil
}

func TestBeanHelpers(t *testing.T) {
	ctx, err := glue.New(&serviceImpl{})
	require.NoError(t, err)
	defer ctx.Close()

	value, err := glue.GetBean[service](ctx)
	require.NoError(t, err)
	require.Equal(t, "ok", value.Do())

	must := glue.MustGetBean[service](ctx)
	require.Equal(t, "ok", must.Do())

	all := glue.GetBeans[service](ctx)
	require.Equal(t, 1, len(all))
}

func TestPropertyHelpers(t *testing.T) {
	props := glue.NewProperties()
	props.Set("app.port", "8080")
	ctx, err := glue.NewWithOptions([]glue.ContainerOption{glue.WithProperties(props)})
	require.NoError(t, err)
	defer ctx.Close()

	port, err := glue.GetProperty[int](ctx, "app.port")
	require.NoError(t, err)
	require.Equal(t, 8080, port)

	missing := glue.GetPropertyOr[int](ctx, "app.missing", 42)
	require.Equal(t, 42, missing)
}

func TestFactories(t *testing.T) {
	type singletonPayload struct {
		Value int32
	}
	type requestPayload struct {
		Value int32
	}

	var singletonSeq int32
	var prototypeSeq int32
	var requestSeq int32

	type holder struct {
		Singleton    *singletonPayload                              `inject:""`
		NewPrototype func() (*prototypePayload, error)              `inject:"scope=prototype"`
		GetRequest   func(context.Context) (*requestPayload, error) `inject:"scope=request"`
	}

	factory := glue.SingletonFactory(func() (*singletonPayload, error) {
		return &singletonPayload{Value: atomic.AddInt32(&singletonSeq, 1)}, nil
	})
	prototypeFactory := glue.PrototypeFactory(func() (*prototypePayload, error) {
		return &prototypePayload{Value: atomic.AddInt32(&prototypeSeq, 1)}, nil
	})
	requestFactory := glue.RequestFactory(func() (*requestPayload, error) {
		return &requestPayload{Value: atomic.AddInt32(&requestSeq, 1)}, nil
	})

	h := &holder{}

	ctx, err := glue.New(factory, prototypeFactory, requestFactory, h)
	require.NoError(t, err)
	defer ctx.Close()

	result, err := glue.GetBean[*singletonPayload](ctx)
	require.NoError(t, err)
	require.Same(t, h.Singleton, result)
	require.Equal(t, int32(1), result.Value)
	require.Equal(t, int32(1), atomic.LoadInt32(&singletonSeq))

	p1, err := h.NewPrototype()
	require.NoError(t, err)
	p2, err := h.NewPrototype()
	require.NoError(t, err)
	require.NotSame(t, p1, p2)
	require.NotEqual(t, p1.Value, p2.Value)
	require.Equal(t, int32(0), atomic.LoadInt32(&p1.closed))
	require.NoError(t, p1.Close())
	require.Equal(t, int32(1), atomic.LoadInt32(&p1.closed))
	require.Equal(t, int32(0), atomic.LoadInt32(&p2.closed))

	scope1 := glue.NewRequestScope()
	defer scope1.Close()
	reqCtx1 := glue.WithRequestScope(context.Background(), scope1)
	r1a, err := h.GetRequest(reqCtx1)
	require.NoError(t, err)
	r1b, err := h.GetRequest(reqCtx1)
	require.NoError(t, err)
	require.Same(t, r1a, r1b)

	scope2 := glue.NewRequestScope()
	defer scope2.Close()
	reqCtx2 := glue.WithRequestScope(context.Background(), scope2)
	r2, err := h.GetRequest(reqCtx2)
	require.NoError(t, err)
	require.NotSame(t, r1a, r2)
}

func TestContextFactories(t *testing.T) {
	type ctxPayload struct {
		TraceID string
	}

	type traceKey struct{}

	singletonFactory := glue.SingletonContextFactory(func(ctx context.Context) (*ctxPayload, error) {
		traceID, _ := ctx.Value(traceKey{}).(string)
		return &ctxPayload{TraceID: traceID}, nil
	})

	holder := &struct {
		Payload *ctxPayload `inject:""`
	}{}

	buildCtx := context.WithValue(context.Background(), traceKey{}, "trace-123")
	ctn, err := glue.NewWithContext(buildCtx, singletonFactory, holder)
	require.NoError(t, err)
	defer ctn.Close()

	require.NotNil(t, holder.Payload)
	require.Equal(t, "trace-123", holder.Payload.TraceID)

	result, err := glue.GetBean[*ctxPayload](ctn)
	require.NoError(t, err)
	require.Same(t, holder.Payload, result)
}

func TestContextFactoryRequestScope(t *testing.T) {
	type ctxSession struct {
		TraceID string
		Seq     int32
	}

	type traceKey struct{}

	var seq int32
	requestFactory := glue.RequestContextFactory(func(ctx context.Context) (*ctxSession, error) {
		traceID, _ := ctx.Value(traceKey{}).(string)
		return &ctxSession{TraceID: traceID, Seq: atomic.AddInt32(&seq, 1)}, nil
	})

	holder := &struct {
		GetSession func(context.Context) (*ctxSession, error) `inject:"scope=request"`
	}{}

	ctn, err := glue.New(requestFactory, holder)
	require.NoError(t, err)
	defer ctn.Close()

	scope := glue.NewRequestScope()
	defer scope.Close()
	reqCtx := glue.WithRequestScope(context.WithValue(context.Background(), traceKey{}, "trace-abc"), scope)

	s1, err := holder.GetSession(reqCtx)
	require.NoError(t, err)
	require.Equal(t, "trace-abc", s1.TraceID)

	// Same scope returns same instance
	s2, err := holder.GetSession(reqCtx)
	require.NoError(t, err)
	require.Same(t, s1, s2)

	// Different scope returns different instance
	scope2 := glue.NewRequestScope()
	defer scope2.Close()
	reqCtx2 := glue.WithRequestScope(context.WithValue(context.Background(), traceKey{}, "trace-def"), scope2)
	s3, err := holder.GetSession(reqCtx2)
	require.NoError(t, err)
	require.NotSame(t, s1, s3)
	require.Equal(t, "trace-def", s3.TraceID)
}
