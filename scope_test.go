/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue_test

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.arpabet.com/glue"
)

// --- Prototype scope tests ---

type protoWorker struct {
	ID int32
}

var protoWorkerClass = reflect.TypeOf((*protoWorker)(nil))

var protoWorkerIDSeq int32

type protoWorkerFactory struct {
	glue.FactoryBean
}

func (t *protoWorkerFactory) Object() (any, error) {
	return &protoWorker{ID: atomic.AddInt32(&protoWorkerIDSeq, 1)}, nil
}

func (t *protoWorkerFactory) ObjectType() reflect.Type {
	return protoWorkerClass
}

func (t *protoWorkerFactory) ObjectName() string {
	return ""
}

func (t *protoWorkerFactory) Singleton() bool {
	return false
}

type protoConsumer struct {
	NewWorker func() (*protoWorker, error) `inject:"scope=prototype"`
}

func TestPrototypeScope(t *testing.T) {
	atomic.StoreInt32(&protoWorkerIDSeq, 0)

	consumer := &protoConsumer{}
	ctx, err := glue.New(
		&protoWorkerFactory{},
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, consumer.NewWorker)

	w1, err := consumer.NewWorker()
	require.NoError(t, err)
	require.NotNil(t, w1)

	w2, err := consumer.NewWorker()
	require.NoError(t, err)
	require.NotNil(t, w2)

	// Each call should produce a new instance
	require.NotSame(t, w1, w2)
	require.NotEqual(t, w1.ID, w2.ID)
}

type protoConsumerWithCtx struct {
	NewWorker func(context.Context) (*protoWorker, error) `inject:"scope=prototype"`
}

func TestPrototypeScopeWithContext(t *testing.T) {
	atomic.StoreInt32(&protoWorkerIDSeq, 0)

	consumer := &protoConsumerWithCtx{}
	ctx, err := glue.New(
		&protoWorkerFactory{},
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, consumer.NewWorker)

	w1, err := consumer.NewWorker(context.Background())
	require.NoError(t, err)
	require.NotNil(t, w1)

	w2, err := consumer.NewWorker(context.Background())
	require.NoError(t, err)
	require.NotNil(t, w2)

	require.NotSame(t, w1, w2)
}

// --- Request scope tests ---

type requestSession struct {
	UserID string
}

var requestSessionClass = reflect.TypeOf((*requestSession)(nil))

var requestSessionSeq int32

type requestSessionFactory struct {
	glue.FactoryBean
}

func (t *requestSessionFactory) Object() (any, error) {
	id := atomic.AddInt32(&requestSessionSeq, 1)
	return &requestSession{UserID: "user-" + string(rune('0'+id))}, nil
}

func (t *requestSessionFactory) ObjectType() reflect.Type {
	return requestSessionClass
}

func (t *requestSessionFactory) ObjectName() string {
	return ""
}

func (t *requestSessionFactory) Singleton() bool {
	return false
}

type requestConsumer struct {
	GetSession func(context.Context) (*requestSession, error) `inject:"scope=request"`
}

func TestRequestScope(t *testing.T) {
	atomic.StoreInt32(&requestSessionSeq, 0)

	consumer := &requestConsumer{}
	ctx, err := glue.New(
		&requestSessionFactory{},
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, consumer.GetSession)

	// Create a request scope
	scope1 := glue.NewRequestScope()
	reqCtx1 := glue.WithRequestScope(context.Background(), scope1)

	s1a, err := consumer.GetSession(reqCtx1)
	require.NoError(t, err)
	require.NotNil(t, s1a)

	// Same request scope should return same instance
	s1b, err := consumer.GetSession(reqCtx1)
	require.NoError(t, err)
	require.Same(t, s1a, s1b)

	// Different request scope should return different instance
	scope2 := glue.NewRequestScope()
	reqCtx2 := glue.WithRequestScope(context.Background(), scope2)

	s2, err := consumer.GetSession(reqCtx2)
	require.NoError(t, err)
	require.NotNil(t, s2)
	require.NotSame(t, s1a, s2)
}

func TestRequestScopeNoContext(t *testing.T) {
	atomic.StoreInt32(&requestSessionSeq, 0)

	consumer := &requestConsumer{}
	ctx, err := glue.New(
		&requestSessionFactory{},
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	// Calling without a RequestScope in context should return error
	_, err = consumer.GetSession(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no RequestScope found")
}

// --- Validation tests ---

type badScopeNotFunc struct {
	Worker *protoWorker `inject:"scope=prototype"`
}

func TestScopeValidation_NotFunc(t *testing.T) {
	_, err := glue.New(
		&protoWorkerFactory{},
		&badScopeNotFunc{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a function type")
}

type badScopeUnknown struct {
	Worker func() (*protoWorker, error) `inject:"scope=custom"`
}

func TestScopeValidation_UnknownScope(t *testing.T) {
	_, err := glue.New(
		&protoWorkerFactory{},
		&badScopeUnknown{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown scope")
}

type badRequestNoCtx struct {
	Worker func() (*protoWorker, error) `inject:"scope=request"`
}

func TestScopeValidation_RequestRequiresContext(t *testing.T) {
	_, err := glue.New(
		&protoWorkerFactory{},
		&badRequestNoCtx{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must have exactly 1 parameter")
}

type badScopeWrongReturn struct {
	Worker func() *protoWorker `inject:"scope=prototype"`
}

func TestScopeValidation_MustReturnError(t *testing.T) {
	_, err := glue.New(
		&protoWorkerFactory{},
		&badScopeWrongReturn{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must return (T, error)")
}

// --- Interface-based scoped injection ---

var WorkerInterface = reflect.TypeOf((*Worker)(nil)).Elem()

type Worker interface {
	DoWork() string
}

type workerImpl struct {
	id int32
}

func (w *workerImpl) DoWork() string {
	return "working"
}

var workerImplClass = reflect.TypeOf((*workerImpl)(nil))

type workerFactory struct {
	glue.FactoryBean
}

var workerSeq int32

func (t *workerFactory) Object() (any, error) {
	return &workerImpl{id: atomic.AddInt32(&workerSeq, 1)}, nil
}

func (t *workerFactory) ObjectType() reflect.Type {
	return workerImplClass
}

func (t *workerFactory) ObjectName() string {
	return ""
}

func (t *workerFactory) Singleton() bool {
	return false
}

type ifaceProtoConsumer struct {
	NewWorker func() (Worker, error) `inject:"scope=prototype"`
}

func TestPrototypeScopeInterface(t *testing.T) {
	atomic.StoreInt32(&workerSeq, 0)

	consumer := &ifaceProtoConsumer{}
	ctx, err := glue.New(
		&workerFactory{},
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, consumer.NewWorker)

	w1, err := consumer.NewWorker()
	require.NoError(t, err)
	require.Equal(t, "working", w1.DoWork())

	w2, err := consumer.NewWorker()
	require.NoError(t, err)
	require.NotSame(t, w1.(*workerImpl), w2.(*workerImpl))
}

// --- Non-factory scoped bean with field injection and ContextInitializingBean ---

type sharedConfig struct {
	Value string
}

type scopedHandler struct {
	glue.ContextInitializingBean
	Config          *sharedConfig `inject:""`
	InitializedWith string
}

func (t *scopedHandler) PostConstruct(ctx context.Context) error {
	// Verify that injected field is set and context is propagated
	if t.Config == nil {
		return fmt.Errorf("config was not injected")
	}
	t.InitializedWith = t.Config.Value
	return nil
}

type handlerConsumer struct {
	NewHandler func(context.Context) (*scopedHandler, error) `inject:"scope=prototype"`
}

func TestPrototypeScopeNonFactory_FieldInjectionAndContextInit(t *testing.T) {
	cfg := &sharedConfig{Value: "test-config"}
	consumer := &handlerConsumer{}

	ctx, err := glue.New(
		cfg,
		&scopedHandler{},
		consumer,
	)
	require.NoError(t, err)
	defer ctx.Close()

	require.NotNil(t, consumer.NewHandler)

	h1, err := consumer.NewHandler(context.Background())
	require.NoError(t, err)
	require.NotNil(t, h1)

	// Field injection should have set Config
	require.NotNil(t, h1.Config)
	require.Same(t, cfg, h1.Config)

	// ContextInitializingBean.PostConstruct(ctx) should have run
	require.Equal(t, "test-config", h1.InitializedWith)

	// Each call produces a new instance
	h2, err := consumer.NewHandler(context.Background())
	require.NoError(t, err)
	require.NotSame(t, h1, h2)
	require.Same(t, cfg, h2.Config)
	require.Equal(t, "test-config", h2.InitializedWith)
}

// --- Request scope with classical bean implementing ScopedBean (no FactoryBean) ---

type requestLogger struct {
	glue.ScopedBean
	glue.InitializingBean
	Config      *sharedConfig `inject:""`
	initialized bool
	message     string
}

func (t *requestLogger) BeanScope() glue.BeanScope {
	return glue.ScopeRequest
}

func (t *requestLogger) PostConstruct() error {
	if t.Config == nil {
		return fmt.Errorf("config was not injected into requestLogger")
	}
	t.initialized = true
	t.message = "logger for " + t.Config.Value
	return nil
}

type requestLoggerConsumer struct {
	GetLogger func(context.Context) (*requestLogger, error) `inject:"scope=request"`
}

func TestRequestScopeClassicalBean(t *testing.T) {
	cfg := &sharedConfig{Value: "prod"}
	consumer := &requestLoggerConsumer{}

	ctn, err := glue.New(
		cfg,
		&requestLogger{},
		consumer,
	)
	require.NoError(t, err)
	defer ctn.Close()

	require.NotNil(t, consumer.GetLogger)

	// First request
	scope1 := glue.NewRequestScope()
	reqCtx1 := glue.WithRequestScope(context.Background(), scope1)

	l1a, err := consumer.GetLogger(reqCtx1)
	require.NoError(t, err)
	require.NotNil(t, l1a)

	// Field injection and PostConstruct should have run
	require.NotNil(t, l1a.Config)
	require.Same(t, cfg, l1a.Config)
	require.True(t, l1a.initialized)
	require.Equal(t, "logger for prod", l1a.message)

	// Same request scope returns the same cached instance
	l1b, err := consumer.GetLogger(reqCtx1)
	require.NoError(t, err)
	require.Same(t, l1a, l1b)
	require.Equal(t, l1b.message, l1a.message)

	// Second request gets a fresh instance
	scope2 := glue.NewRequestScope()
	reqCtx2 := glue.WithRequestScope(context.Background(), scope2)

	l2, err := consumer.GetLogger(reqCtx2)
	require.NoError(t, err)
	require.NotNil(t, l2)
	require.NotSame(t, l1a, l2)

	// The new instance also got proper injection and initialization
	require.Same(t, cfg, l2.Config)
	require.True(t, l2.initialized)
	require.Equal(t, "logger for prod", l2.message)
}

// --- ContextFactoryBean with request scope ---

type ctxRequestSession struct {
	TraceID string
}

var ctxRequestSessionClass = reflect.TypeOf((*ctxRequestSession)(nil))

type ctxSessionFactory struct {
	glue.ContextFactoryBean
}

type traceKey struct{}

func (t *ctxSessionFactory) Object(ctx context.Context) (any, error) {
	traceID, _ := ctx.Value(traceKey{}).(string)
	return &ctxRequestSession{TraceID: traceID}, nil
}

func (t *ctxSessionFactory) ObjectType() reflect.Type {
	return ctxRequestSessionClass
}

func (t *ctxSessionFactory) ObjectName() string {
	return ""
}

func (t *ctxSessionFactory) Singleton() bool {
	return false
}

type ctxSessionConsumer struct {
	GetSession func(context.Context) (*ctxRequestSession, error) `inject:"scope=request"`
}

func TestRequestScopeWithContextFactoryBean(t *testing.T) {
	consumer := &ctxSessionConsumer{}
	ctn, err := glue.New(
		&ctxSessionFactory{},
		consumer,
	)
	require.NoError(t, err)
	defer ctn.Close()

	require.NotNil(t, consumer.GetSession)

	// Create a request context with trace ID and request scope
	scope1 := glue.NewRequestScope()
	reqCtx := context.WithValue(context.Background(), traceKey{}, "trace-abc")
	reqCtx = glue.WithRequestScope(reqCtx, scope1)

	s1, err := consumer.GetSession(reqCtx)
	require.NoError(t, err)
	require.NotNil(t, s1)
	// ContextFactoryBean should have received the context with the trace ID
	require.Equal(t, "trace-abc", s1.TraceID)

	// Same request scope returns same instance
	s1b, err := consumer.GetSession(reqCtx)
	require.NoError(t, err)
	require.Same(t, s1, s1b)

	// Different request scope with different trace
	scope2 := glue.NewRequestScope()
	reqCtx2 := context.WithValue(context.Background(), traceKey{}, "trace-xyz")
	reqCtx2 = glue.WithRequestScope(reqCtx2, scope2)

	s2, err := consumer.GetSession(reqCtx2)
	require.NoError(t, err)
	require.NotSame(t, s1, s2)
	require.Equal(t, "trace-xyz", s2.TraceID)
}
