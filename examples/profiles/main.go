/*
 * Glue Example: Multi-Profile App Bootstrap
 *
 * Demonstrates:
 *   - Multiple active profiles (dev, staging, prod)
 *   - Profile expressions: "dev", "prod", "dev|staging", "!prod"
 *   - ConditionalBean for feature flags
 *   - glue.IfProfile for grouping beans
 *   - Per-environment property source
 *   - Parent-child container hierarchy
 *   - EnvPropertyResolver with profile-dependent prefix
 *   - OrderedBean for initialization ordering
 *
 * Run:
 *   go run ./examples/profiles                    # defaults to dev
 *   go run ./examples/profiles staging
 *   go run ./examples/profiles prod
 *   go run ./examples/profiles dev local           # multiple profiles
 */
package main

import (
	"fmt"
	"log"
	"os"
	"reflect"

	"go.arpabet.com/glue"
)

func main() {
	log.SetFlags(0)

	profiles := os.Args[1:]
	if len(profiles) == 0 {
		profiles = []string{"dev"}
	}

	fmt.Printf("Active profiles: %v\n\n", profiles)

	// --- base properties (shared across all profiles) ---
	baseProps := &glue.PropertySource{Map: map[string]any{
		"app.name":    "glue-profiles-demo",
		"app.version": "1.0.0",
		"log.level":   "info",
	}}

	// --- profile-specific property overrides ---
	devProps := &glue.PropertySource{Map: map[string]any{
		"log.level":  "debug",
		"db.host":    "localhost",
		"db.port":    "5432",
		"cache.host": "localhost",
		"cache.port": "6379",
	}}
	stagingProps := &glue.PropertySource{Map: map[string]any{
		"log.level":  "info",
		"db.host":    "staging-db.internal",
		"db.port":    "5432",
		"cache.host": "staging-cache.internal",
		"cache.port": "6379",
	}}
	prodProps := &glue.PropertySource{Map: map[string]any{
		"log.level":  "warn",
		"db.host":    "prod-db.internal",
		"db.port":    "5432",
		"cache.host": "prod-cache.internal",
		"cache.port": "6379",
	}}

	ctn, err := glue.NewWithOptions(
		[]glue.ContainerOption{
			glue.WithProfiles(profiles...),
		},
		baseProps,

		// profile-specific properties
		glue.IfProfile("dev", devProps),
		glue.IfProfile("staging", stagingProps),
		glue.IfProfile("prod", prodProps),

		// shared beans
		&appInfo{},
		&logger{},

		// profile-selected database
		glue.IfProfile("dev", &devDB{}),
		glue.IfProfile("staging", &stagingDB{}),
		glue.IfProfile("prod", &prodDB{}),

		// beans active in dev OR staging
		glue.IfProfile("dev|staging", &debugEndpoint{}),

		// beans active only when prod is NOT active
		&mockMetrics{},

		// conditional bean: enabled by explicit feature flag
		&experimentalFeature{enabled: contains(profiles, "experimental")},

		// child container for plugin subsystem
		glue.Child("plugins",
			&pluginA{},
			&pluginB{},
			&pluginManager{},
		),
	)
	if err != nil {
		log.Fatalf("container creation failed: %v", err)
	}
	defer ctn.Close()

	// print what we got
	fmt.Println("=== Registered Beans ===")
	for _, typ := range ctn.Core() {
		beans := ctn.Bean(typ, glue.SearchCurrent)
		for _, b := range beans {
			fmt.Printf("  %s\n", b)
		}
	}

	// activate child container
	fmt.Println("\n=== Child Container: plugins ===")
	children := ctn.Children()
	for _, child := range children {
		childCtn, err := child.Object()
		if err != nil {
			log.Fatalf("child container %s failed: %v", child.ChildName(), err)
		}
		for _, typ := range childCtn.Core() {
			beans := childCtn.Bean(typ, glue.SearchCurrent)
			for _, b := range beans {
				fmt.Printf("  %s\n", b)
			}
		}
	}

	fmt.Println("\n=== Dependency Graph ===")
	fmt.Println(ctn.Graph())
}

// ---------------------------------------------------------------------------
// Shared beans
// ---------------------------------------------------------------------------

type appInfo struct {
	Name    string `value:"app.name"`
	Version string `value:"app.version"`
}

func (a *appInfo) BeanOrder() int { return 0 }
func (a *appInfo) PostConstruct() error {
	fmt.Printf("[app] %s v%s\n", a.Name, a.Version)
	return nil
}

type logger struct {
	Level string `value:"log.level"`
}

func (l *logger) BeanOrder() int { return 1 }
func (l *logger) PostConstruct() error {
	fmt.Printf("[logger] level=%s\n", l.Level)
	return nil
}

// ---------------------------------------------------------------------------
// Database — profile-selected implementations
// ---------------------------------------------------------------------------

type DB interface {
	DSN() string
}

var dbType = reflect.TypeOf((*DB)(nil)).Elem()

type devDB struct {
	Host string `value:"db.host"`
	Port string `value:"db.port"`
}

func (d *devDB) BeanName() string   { return "database" }
func (d *devDB) DSN() string        { return fmt.Sprintf("dev://%s:%s", d.Host, d.Port) }
func (d *devDB) PostConstruct() error {
	fmt.Printf("[db] dev mode: %s\n", d.DSN())
	return nil
}

type stagingDB struct {
	Host string `value:"db.host"`
	Port string `value:"db.port"`
}

func (d *stagingDB) BeanName() string   { return "database" }
func (d *stagingDB) DSN() string        { return fmt.Sprintf("staging://%s:%s", d.Host, d.Port) }
func (d *stagingDB) PostConstruct() error {
	fmt.Printf("[db] staging mode: %s\n", d.DSN())
	return nil
}

type prodDB struct {
	Host string `value:"db.host"`
	Port string `value:"db.port"`
}

func (d *prodDB) BeanName() string   { return "database" }
func (d *prodDB) DSN() string        { return fmt.Sprintf("prod://%s:%s", d.Host, d.Port) }
func (d *prodDB) PostConstruct() error {
	fmt.Printf("[db] prod mode: %s\n", d.DSN())
	return nil
}

// ---------------------------------------------------------------------------
// Profile expression: "dev|staging" — active when either profile is active
// ---------------------------------------------------------------------------

type debugEndpoint struct{}

func (d *debugEndpoint) PostConstruct() error {
	fmt.Println("[debug] debug endpoint enabled (dev or staging)")
	return nil
}

// ---------------------------------------------------------------------------
// ConditionalBean: "!prod" profile — active when prod is NOT active
// ---------------------------------------------------------------------------

type mockMetrics struct{}

func (m *mockMetrics) BeanProfile() string { return "!prod" }
func (m *mockMetrics) PostConstruct() error {
	fmt.Println("[metrics] using mock metrics (non-prod)")
	return nil
}

// ---------------------------------------------------------------------------
// ConditionalBean: feature flag
// ---------------------------------------------------------------------------

type experimentalFeature struct {
	enabled bool
}

func (f *experimentalFeature) ShouldRegisterBean() bool { return f.enabled }
func (f *experimentalFeature) PostConstruct() error {
	fmt.Println("[experimental] feature enabled!")
	return nil
}

// ---------------------------------------------------------------------------
// Child container beans
// ---------------------------------------------------------------------------

type Plugin interface {
	Name() string
}

type pluginA struct{}

func (p *pluginA) Name() string { return "plugin-a" }
func (p *pluginA) PostConstruct() error {
	fmt.Println("[plugin-a] loaded")
	return nil
}

type pluginB struct{}

func (p *pluginB) Name() string { return "plugin-b" }
func (p *pluginB) BeanOrder() int { return 2 }
func (p *pluginB) PostConstruct() error {
	fmt.Println("[plugin-b] loaded")
	return nil
}

type pluginManager struct {
	Plugins []Plugin `inject:""`
}

func (m *pluginManager) PostConstruct() error {
	names := make([]string, len(m.Plugins))
	for i, p := range m.Plugins {
		names[i] = p.Name()
	}
	fmt.Printf("[plugin-manager] managing %d plugins: %v\n", len(m.Plugins), names)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
