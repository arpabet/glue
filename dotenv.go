/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

const defaultDotEnvPropertyResolverPriority = 300

// DotEnvPropertyResolver resolves properties from a .env file.
// Property keys are converted to env var names the same way as EnvPropertyResolver:
// "app.db.host" -> "APP_DB_HOST" (dots and dashes to underscores, uppercase).
//
// The .env file is parsed once on first access and cached.
// Priority defaults to 300, which is higher than EnvPropertyResolver (200),
// so .env values take precedence over OS environment variables.
type DotEnvPropertyResolver struct {

	// Path to the .env file. Defaults to ".env" if empty.
	Path string

	// ResolverPriority controls ordering among property resolvers.
	// Higher number means higher precedence. Default: 300.
	ResolverPriority int

	// KeyMapper overrides the default key-to-env-var conversion.
	// Receives the property key (e.g., "app.db.host") and returns the env var name.
	KeyMapper func(key string) string

	// MatchKey limits which property keys participate in .env lookup.
	// When set and it returns false, GetProperty returns no value without
	// consulting the .env file.
	MatchKey func(propKey, envKey string) bool

	once  sync.Once
	store map[string]string
}

func (r *DotEnvPropertyResolver) Priority() int {
	if r.ResolverPriority != 0 {
		return r.ResolverPriority
	}
	return defaultDotEnvPropertyResolverPriority
}

func (r *DotEnvPropertyResolver) GetProperty(key string) (string, bool) {
	r.loadOnce()
	envKey := r.toEnvKey(key)
	if r.MatchKey != nil && !r.MatchKey(key, envKey) {
		return "", false
	}
	value, ok := r.store[envKey]
	return value, ok
}

// Keys returns all keys from the .env file as property-style keys.
// Env var names are converted back: uppercase underscores become lowercase dots.
// When KeyMapper is set, reverse mapping is not possible and Keys returns nil.
func (r *DotEnvPropertyResolver) Keys() []string {
	r.loadOnce()
	if r.KeyMapper != nil {
		return nil
	}
	var keys []string
	for k := range r.store {
		propKey := strings.ToLower(strings.ReplaceAll(k, "_", "."))
		if r.MatchKey != nil && !r.MatchKey(propKey, k) {
			continue
		}
		keys = append(keys, propKey)
	}
	return keys
}

func (r *DotEnvPropertyResolver) loadOnce() {
	r.once.Do(func() {
		r.store = make(map[string]string)
		path := r.Path
		if path == "" {
			path = ".env"
		}
		parsed, err := parseEnvFile(path)
		if err != nil {
			return // file missing or unreadable — treat as empty
		}
		for k, v := range parsed {
			r.store[k] = v
		}
	})
}

func (r *DotEnvPropertyResolver) toEnvKey(key string) string {
	if r.KeyMapper != nil {
		return r.KeyMapper(key)
	}
	envKey := strings.ToUpper(key)
	envKey = strings.ReplaceAll(envKey, ".", "_")
	envKey = strings.ReplaceAll(envKey, "-", "_")
	return envKey
}

// parseEnvFile parses a .env file into a map.
// Supports KEY=VALUE, # comments, optional "export" prefix, and quoted values.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")

		idx := strings.IndexAny(line, "=:")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		result[key] = value
	}
	return result, scanner.Err()
}
