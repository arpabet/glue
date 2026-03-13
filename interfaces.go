/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"reflect"
	"sync"
)

/**
Mutable cache for interface-to-implementation lookups.
Populated lazily on first lookup per interface type and reused for subsequent calls.
*/

type interfaceCache struct {
	sync.RWMutex
	candidates map[reflect.Type][]*bean
}

func ctorInterfaceCache() interfaceCache {
	return interfaceCache{
		candidates: make(map[reflect.Type][]*bean),
	}
}

func (t *interfaceCache) find(ifaceType reflect.Type) ([]*bean, bool) {
	t.RLock()
	defer t.RUnlock()
	list, ok := t.candidates[ifaceType]
	return list, ok
}

func (t *interfaceCache) store(ifaceType reflect.Type, list []*bean) {
	t.Lock()
	defer t.Unlock()
	if len(list) == 0 {
		_, ok := t.candidates[ifaceType]
		if !ok {
			// use placeholder for the interface type even if not found, for repeated cache lookups
			t.candidates[ifaceType] = []*bean{}
		}
	} else {
		for _, b := range list {
			t.candidates[ifaceType] = append(t.candidates[ifaceType], b)
		}
	}
}
