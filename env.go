/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"os"
	"strings"
)

const defaultEnvPropertyResolverPriority = 200

// EnvPropertyResolver resolves properties from OS environment variables.
// Property keys are converted to env var names: "app.db.host" -> "APP_DB_HOST"
// (dots and dashes to underscores, uppercase).
//
// An optional prefix filters env vars: with prefix "MYAPP", the key "db.host"
// maps to "MYAPP_DB_HOST".
//
// A custom KeyMapper function overrides the default key-to-env-var mapping.
type EnvPropertyResolver struct {

	// Prefix prepended to the env var name (e.g., "MYAPP" -> "MYAPP_DB_HOST").
	// Empty means no prefix.
	Prefix string

	// ResolverPriority controls ordering among property resolvers.
	// Higher number means higher precedence. Default: 200.
	// The built-in Properties store uses 100, so env values override file/map values by default.
	ResolverPriority int

	// KeyMapper overrides the default key-to-env-var conversion.
	// When set, Prefix is ignored.
	// Receives the property key (e.g., "app.db.host") and returns the env var name.
	KeyMapper func(key string) string

	// MatchKey limits which property keys participate in environment lookup.
	// When set and it returns false, GetProperty returns no value without
	// consulting the environment.
	MatchKey func(propKey, envKey string) bool
}

func OnlyEnvStyle(propKey, envKey string) bool {
	return propKey == envKey
}

func (r *EnvPropertyResolver) Priority() int {
	if r.ResolverPriority != 0 {
		return r.ResolverPriority
	}
	return defaultEnvPropertyResolverPriority
}

func (r *EnvPropertyResolver) GetProperty(key string) (string, bool) {
	envKey := r.toEnvKey(key)
	if r.MatchKey != nil && !r.MatchKey(key, envKey) {
		return "", false
	}
	return os.LookupEnv(r.withPrefix(envKey))
}

func (r *EnvPropertyResolver) toEnvKey(key string) string {
	if r.KeyMapper != nil {
		return r.KeyMapper(key)
	}
	envKey := strings.ToUpper(key)
	envKey = strings.ReplaceAll(envKey, ".", "_")
	envKey = strings.ReplaceAll(envKey, "-", "_")
	return envKey
}

func (r *EnvPropertyResolver) withPrefix(envKey string) string {
	if r.Prefix != "" {
		return r.Prefix + "_" + envKey
	}
	return envKey
}
