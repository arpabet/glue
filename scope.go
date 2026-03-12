package glue

import (
	"context"
	"reflect"
	"sync"
)

type requestScope struct {
	mu          sync.Mutex
	instances   map[reflect.Type]any
	disposables []any
	closeOnce   sync.Once
}

// NewRequestScope creates a new empty request scope.
func NewRequestScope() RequestScope {
	return &requestScope{
		instances: make(map[reflect.Type]any),
	}
}

type requestScopeKey struct{}

// WithRequestScope returns a new context carrying the given RequestScope.
func WithRequestScope(ctx context.Context, scope RequestScope) context.Context {
	return context.WithValue(ctx, requestScopeKey{}, scope)
}

// RequestScopeFromContext extracts the RequestScope from the context, if present.
func RequestScopeFromContext(ctx context.Context) (RequestScope, bool) {
	scope, ok := ctx.Value(requestScopeKey{}).(RequestScope)
	return scope, ok
}

// getOrCreate returns the cached instance for the given type, or calls create to make one.
func (rs *requestScope) getOrCreate(typ reflect.Type, create func() (any, error)) (any, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if inst, ok := rs.instances[typ]; ok {
		return inst, nil
	}
	inst, err := create()
	if err != nil {
		return nil, err
	}
	rs.instances[typ] = inst
	rs.addDisposable(inst)
	return inst, nil
}

func (rs *requestScope) addDisposable(obj any) {
	if _, ok := obj.(ContextDisposableBean); ok {
		rs.disposables = append(rs.disposables, obj)
	} else if _, ok := obj.(DisposableBean); ok {
		rs.disposables = append(rs.disposables, obj)
	}
}

func (rs *requestScope) Close() error {
	return rs.CloseWithContext(context.Background())
}

func (rs *requestScope) CloseWithContext(ctx context.Context) (err error) {
	var listErr []error

	rs.closeOnce.Do(func() {
		rs.mu.Lock()
		disposables := append([]any(nil), rs.disposables...)
		rs.mu.Unlock()

		for i := len(disposables) - 1; i >= 0; i-- {
			if dis, ok := disposables[i].(ContextDisposableBean); ok {
				if e := dis.Destroy(ctx); e != nil {
					listErr = append(listErr, e)
				}
			} else if dis, ok := disposables[i].(DisposableBean); ok {
				if e := dis.Destroy(); e != nil {
					listErr = append(listErr, e)
				}
			}
		}
	})

	return multipleErr(listErr)
}
