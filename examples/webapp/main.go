/*
 * Glue Example: Web Application
 *
 * Demonstrates the full Glue feature set in one realistic HTTP server:
 *   - Profile-based bean selection (dev vs prod database)
 *   - Dynamic func() string for a secret that can change at runtime
 *   - Request-scoped bean for per-request transaction
 *   - Decorator for service-layer logging
 *   - Property expressions with env var fallback
 *   - Collection injection for HTTP handlers
 *   - BeanPostProcessor for auto-registering HTTP handlers
 *   - Prefix map injection for grouped config
 *   - Container.Graph() for debugging
 *   - Graceful shutdown via context cancellation
 *
 * Run:
 *   go run ./examples/webapp
 *   GLUE_PROFILES=prod go run ./examples/webapp
 *   APP_SERVER_PORT=9090 go run ./examples/webapp
 */
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"go.arpabet.com/glue"
)

func main() {
	profiles := strings.Split(os.Getenv("GLUE_PROFILES"), ",")
	if len(profiles) == 1 && profiles[0] == "" {
		profiles = []string{"dev"}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctn, err := glue.NewWithOptions(
		[]glue.ContainerOption{
			glue.WithContext(ctx),
			glue.WithProfiles(profiles...),
			glue.WithLogger(&simpleLogger{}),
		},

		// --- config ---
		&glue.PropertySource{Map: map[string]any{
			"app.name":            "glue-webapp",
			"server.port":         "${APP_SERVER_PORT:8080}",
			"server.read.timeout": "5s",
			"db.driver":           "postgres",
			"db.host":             "${APP_DB_HOST:localhost}",
			"db.port":             "5432",
			"db.name":             "${app.name}_db",
			"db.pool.max":         "20",
			"db.pool.min":         "5",
			"db.pool.idle":        "10s",
		}},
		&glue.EnvPropertyResolver{Prefix: "APP"},

		// --- beans ---
		&serverConfig{},
		&handlerRegistrar{},
		&serviceLoggingDecorator{},
		&userServiceImpl{},
		&txContext{}, // template for request-scoped instances

		// profile-selected database
		glue.IfProfile("dev", &devDatabase{}),
		glue.IfProfile("prod", &prodDatabase{}),

		// handlers
		&healthHandler{},
		&userHandler{},
		&graphHandler{},
	)
	if err != nil {
		log.Fatalf("container creation failed: %v", err)
	}
	defer ctn.Close()

	// print dependency graph
	fmt.Println("--- Dependency Graph ---")
	fmt.Println(ctn.Graph())
	fmt.Println("------------------------")

	// start HTTP server
	cfg := lookupBean[*serverConfig](ctn)
	mux := lookupBean[*handlerRegistrar](ctn)

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%s", cfg.Port),
		Handler:     mux.mux,
		ReadTimeout: cfg.ReadTimeout,
	}

	go func() {
		log.Printf("[%s] listening on :%s (profiles: %v)", cfg.AppName, cfg.Port, profiles)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func lookupBean[T any](ctn glue.Container) T {
	var zero T
	typ := reflect.TypeOf(&zero).Elem()
	beans := ctn.Bean(typ, glue.DefaultSearchLevel)
	if len(beans) == 0 {
		log.Fatalf("bean not found: %v", typ)
	}
	return beans[0].Object().(T)
}

type simpleLogger struct {
}

// Enabled - return true if log is enabled
func (t *simpleLogger) Enabled() bool {
	return true
}

// Printf calls l.Output to print to the logger.
// Arguments are handled in the manner of [fmt.Printf].
func (t *simpleLogger) Printf(format string, v ...any) {
	log.Printf(format, v...)
}

// Println calls l.Output to print to the logger.
// Arguments are handled in the manner of [fmt.Println].
func (t *simpleLogger) Println(v ...any) {
	log.Println(v...)
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

type serverConfig struct {
	AppName     string            `value:"app.name"`
	Port        string            `value:"server.port"`
	ReadTimeout time.Duration     `value:"server.read.timeout"`
	DB          map[string]string `value:"prefix=db"`
}

func (c *serverConfig) PostConstruct() error {
	log.Printf("config loaded: port=%s db=%v", c.Port, c.DB)
	return nil
}

// ---------------------------------------------------------------------------
// Database — profile-selected
// ---------------------------------------------------------------------------

// Database is the interface both dev and prod implement.
type Database interface {
	Query(ctx context.Context, sql string) (string, error)
}

// devDatabase is a stub used during development.
type devDatabase struct {
	Host string `value:"db.host"`
	Name string `value:"db.name"`
}

func (d *devDatabase) BeanName() string    { return "database" }
func (d *devDatabase) BeanProfile() string { return "dev" }
func (d *devDatabase) PostConstruct() error {
	log.Printf("[dev-db] connected to %s/%s (in-memory stub)", d.Host, d.Name)
	return nil
}
func (d *devDatabase) Destroy() error {
	log.Println("[dev-db] closed")
	return nil
}
func (d *devDatabase) Query(_ context.Context, sql string) (string, error) {
	return fmt.Sprintf("dev-result(%s)", sql), nil
}

// prodDatabase connects to a real database (simulated here).
type prodDatabase struct {
	Host string `value:"db.host"`
	Port string `value:"db.port"`
	Name string `value:"db.name"`

	// Dynamic secret — re-read on every call so rotated credentials work.
	Password func() (string, error) `value:"db.password,default=changeme"`
}

func (d *prodDatabase) BeanName() string    { return "database" }
func (d *prodDatabase) BeanProfile() string { return "prod" }
func (d *prodDatabase) PostConstruct() error {
	pw, _ := d.Password()
	log.Printf("[prod-db] connected to %s:%s/%s (pw=%s...)", d.Host, d.Port, d.Name, pw[:min(3, len(pw))])
	return nil
}
func (d *prodDatabase) Destroy() error {
	log.Println("[prod-db] connection pool closed")
	return nil
}
func (d *prodDatabase) Query(_ context.Context, sql string) (string, error) {
	return fmt.Sprintf("prod-result(%s)", sql), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// UserService — decorated with logging
// ---------------------------------------------------------------------------

type UserService interface {
	GetUser(ctx context.Context, id string) (string, error)
}

var userServiceType = reflect.TypeOf((*UserService)(nil)).Elem()

type userServiceImpl struct {
	DB Database `inject:""`
}

func (s *userServiceImpl) GetUser(ctx context.Context, id string) (string, error) {
	return s.DB.Query(ctx, fmt.Sprintf("SELECT * FROM users WHERE id='%s'", id))
}

// serviceLoggingDecorator wraps every UserService with request logging.
type serviceLoggingDecorator struct{}

func (d *serviceLoggingDecorator) BeanOrder() int             { return 1 }
func (d *serviceLoggingDecorator) DecorateType() reflect.Type { return userServiceType }
func (d *serviceLoggingDecorator) Decorate(original any) (any, error) {
	return &loggingUserService{delegate: original.(UserService)}, nil
}

type loggingUserService struct {
	delegate UserService
}

func (s *loggingUserService) GetUser(ctx context.Context, id string) (string, error) {
	start := time.Now()
	result, err := s.delegate.GetUser(ctx, id)
	log.Printf("[UserService.GetUser] id=%s dur=%s err=%v", id, time.Since(start), err)
	return result, err
}

// ---------------------------------------------------------------------------
// HTTP Handlers — auto-registered via BeanPostProcessor
// ---------------------------------------------------------------------------

// Handler is implemented by beans that serve HTTP endpoints.
type Handler interface {
	Pattern() string
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// handlerRegistrar is a BeanPostProcessor that auto-registers Handler beans.
// The mux is initialized at construction time so it is available during PostProcessBean
// (which runs before PostConstruct).
type handlerRegistrar struct {
	mux *http.ServeMux
	cnt int
}

func (r *handlerRegistrar) PostProcessBean(bean any, name string) error {
	if r.mux == nil {
		r.mux = http.NewServeMux()
	}
	if h, ok := bean.(Handler); ok {
		log.Printf("registered handler: %s", h.Pattern())
		r.mux.Handle(h.Pattern(), h)
		r.cnt++
	}
	return nil
}

func (r *handlerRegistrar) PostConstruct() error {
	log.Printf("handler registrar ready: %d handlers registered", r.cnt)
	return nil
}

// healthHandler responds to /healthz.
type healthHandler struct{}

func (h *healthHandler) Pattern() string { return "/healthz" }
func (h *healthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"status":"ok"}`)
}

// userHandler uses request scope for a per-request transaction context.
type userHandler struct {
	UserSvc  UserService                               `inject:""`
	NewTxCtx func(context.Context) (*txContext, error) `inject:"scope=request"`
}

func (h *userHandler) Pattern() string { return "/users/" }
func (h *userHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scope := glue.NewRequestScope()
	ctx := glue.WithRequestScope(r.Context(), scope)
	defer scope.Close()

	tx, err := h.NewTxCtx(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/users/")
	result, err := h.UserSvc.GetUser(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "tx=%s result=%s\n", tx.ID, result)
}

// graphHandler dumps the dependency graph as DOT.
type graphHandler struct {
	Ctn glue.Container `inject:""`
}

func (h *graphHandler) Pattern() string { return "/debug/graph" }
func (h *graphHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, h.Ctn.Graph())
}

// ---------------------------------------------------------------------------
// Request-scoped bean
// ---------------------------------------------------------------------------

// txContext represents a per-request transaction.
type txContext struct {
	ID string
}

func (t *txContext) PostConstruct() error {
	t.ID = fmt.Sprintf("tx-%d", time.Now().UnixNano())
	log.Printf("[txContext] created %s", t.ID)
	return nil
}

func (t *txContext) Destroy() error {
	log.Printf("[txContext] destroyed %s", t.ID)
	return nil
}
