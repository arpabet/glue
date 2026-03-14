/*
 * Glue Example: Dynamic Secret-Driven DB Client
 *
 * Demonstrates:
 *   - Custom EnumerablePropertyResolver backed by a mock secret store
 *   - Dynamic func() (string, error) for live secret rotation
 *   - Prefix map injection with resolver-provided keys
 *   - PostConstruct validation
 *   - Container.Reload to pick up rotated secrets
 *
 * The mock secret store simulates a cloud secret manager (AWS Secrets Manager,
 * GCP Secret Manager, HashiCorp Vault) that returns different values over time.
 *
 * Run:
 *   go run ./examples/secrets
 */
package main

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"go.arpabet.com/glue"
)

func main() {
	store := &mockSecretStore{
		secrets: map[string]string{
			"db.username": "app_user",
			"db.password": "initial-secret-v1",
			"db.host":     "prod-db.internal",
			"db.port":     "5432",
			"cache.token": "redis-token-abc",
		},
	}

	ctn, err := glue.New(
		&glue.PropertySource{Map: map[string]any{
			"db.name":     "myapp",
			"db.pool.max": "20",
		}},
		store,         // EnumerablePropertyResolver — keys discoverable for prefix maps
		&dbClient{},
		&cacheClient{},
	)
	if err != nil {
		log.Fatalf("container creation failed: %v", err)
	}
	defer ctn.Close()

	db := lookupBean[*dbClient](ctn)

	// initial state
	fmt.Println("=== Initial ===")
	fmt.Printf("DB config:  %v\n", db.Config)
	pw, _ := db.Password()
	fmt.Printf("DB password (dynamic): %s\n", pw)
	fmt.Printf("Connected: host=%s db=%s user=%s\n", db.Config["host"], db.Config["name"], db.Config["username"])

	// simulate secret rotation
	fmt.Println("\n=== Secret Rotation ===")
	store.Rotate("db.password", "rotated-secret-v2")

	// dynamic property picks up the new value immediately
	pw, _ = db.Password()
	fmt.Printf("DB password (dynamic, after rotation): %s\n", pw)

	// prefix map is a static snapshot — unchanged until Reload
	fmt.Printf("DB config (static, before reload): password=%s\n", db.Config["password"])

	// reload to refresh the prefix map
	beans := ctn.Bean(reflect.TypeOf(db), glue.DefaultSearchLevel)
	if err := ctn.Reload(beans[0]); err != nil {
		log.Fatalf("reload failed: %v", err)
	}
	fmt.Printf("DB config (static, after reload):  password=%s\n", db.Config["password"])

	// cache client also sees the secret store
	cache := lookupBean[*cacheClient](ctn)
	fmt.Printf("\nCache token: %s\n", cache.Token)

	fmt.Println("\n=== Dependency Graph ===")
	fmt.Println(ctn.Graph())
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

// ---------------------------------------------------------------------------
// Mock Secret Store — implements EnumerablePropertyResolver
// ---------------------------------------------------------------------------

type mockSecretStore struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func (s *mockSecretStore) Priority() int { return 500 } // highest priority

func (s *mockSecretStore) GetProperty(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.secrets[key]
	return v, ok
}

func (s *mockSecretStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.secrets))
	for k := range s.secrets {
		keys = append(keys, k)
	}
	return keys
}

func (s *mockSecretStore) Rotate(key, newValue string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.secrets[key]
	s.secrets[key] = newValue
	log.Printf("[secret-store] rotated %s: %s -> %s", key, old, newValue)
}

// ---------------------------------------------------------------------------
// DB Client — uses both prefix map and dynamic property
// ---------------------------------------------------------------------------

type dbClient struct {
	// Static snapshot of all db.* properties (from store + PropertySource)
	Config map[string]string `value:"prefix=db"`

	// Dynamic password — re-reads from the resolver chain on every call.
	// When the secret store rotates the password, the next call gets the new value.
	Password func() (string, error) `value:"db.password"`
}

func (d *dbClient) PostConstruct() error {
	// Validate required fields
	for _, required := range []string{"host", "port", "name"} {
		if d.Config[required] == "" {
			return fmt.Errorf("db.%s is required", required)
		}
	}
	pw, err := d.Password()
	if err != nil {
		return fmt.Errorf("cannot read db.password: %w", err)
	}
	log.Printf("[db-client] connected to %s:%s/%s (user=%s, pw=%s...)",
		d.Config["host"], d.Config["port"], d.Config["name"],
		d.Config["username"], pw[:min(5, len(pw))])
	return nil
}

func (d *dbClient) Destroy() error {
	log.Println("[db-client] connection closed")
	return nil
}

// ---------------------------------------------------------------------------
// Cache Client — reads token from secret store
// ---------------------------------------------------------------------------

type cacheClient struct {
	Token string `value:"cache.token"`
}

func (c *cacheClient) PostConstruct() error {
	log.Printf("[cache-client] initialized with token=%s...", c.Token[:min(5, len(c.Token))])
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	// Suppress log timestamps for cleaner example output
	log.SetFlags(0)
	_ = time.Now // reference time package
}
