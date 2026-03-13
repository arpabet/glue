/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import "reflect"

func (t *container) searchByNameRecursive(name string) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		if list, ok := ctx.localNames[name]; ok && len(list) > 0 {
			candidates = append(candidates, beanlist{level: level, list: list})
		}
		level++
	}
	return candidates
}

func (t *container) findObjectRecursive(requiredType reflect.Type) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		if direct, ok := ctx.core[requiredType]; ok {
			candidates = append(candidates, beanlist{level: level, list: direct})
		}
		level++
	}
	return candidates
}

func (t *container) searchAndCacheInterfaceCandidatesRecursive(ifaceType reflect.Type) []beanlist {
	var candidates []beanlist
	level := 1
	for ctx := t; ctx != nil; ctx = ctx.parent {
		// first lookup in the interface cache
		if list, ok := ctx.ifaceCache.find(ifaceType); !ok {
			list = ctx.searchInterfaceCandidates(ifaceType)
			if len(list) > 0 {
				candidates = append(candidates, beanlist{level: level, list: list})
			}
			// cache result
			// even empty list, so we would not come here again
			ctx.ifaceCache.store(ifaceType, list)
		} else if len(list) > 0 {
			candidates = append(candidates, beanlist{level: level, list: list})
		}
		level++
	}
	return candidates
}

func (t *container) searchInterfaceCandidates(ifaceType reflect.Type) []*bean {
	var candidates []*bean
	// go through all beans in core, since this interface is new for us
	for _, list := range t.core {
		if len(list) > 0 && list[0].beanDef.implements(ifaceType) {
			candidates = append(candidates, list...)
		}
	}
	return candidates
}
