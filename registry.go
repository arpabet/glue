/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue

import (
	"reflect"
	"sync"
)

/**
	Holds runtime information about all beans visible from current context including all parents.
 */

type registry struct {
	sync.RWMutex
	beansByName map[string][]*bean
	beansByType map[reflect.Type][]*bean
}

func (t *registry) findByType(ifaceType reflect.Type) ([]*bean, bool) {
	t.RLock()
	defer t.RUnlock()
	list, ok := t.beansByType[ifaceType]
	return list, ok
}

func (t *registry) findByName(name string) ([]*bean, bool) {
	t.RLock()
	defer t.RUnlock()
	list, ok := t.beansByName[name]
	return list, ok
}

func (t *registry) addBeanList(ifaceType reflect.Type, list []*bean) {
	t.Lock()
	defer t.Unlock()
	for _, b := range list {
		t.beansByType[ifaceType] = append(t.beansByType[ifaceType], b)
		t.beansByName[b.name] = append(t.beansByName[b.name], b)
	}
}

func (t *registry) addBean(ifaceType reflect.Type, b *bean) {
	t.Lock()
	defer t.Unlock()
	t.beansByType[ifaceType] = append(t.beansByType[ifaceType], b)
	t.beansByName[b.name] = append(t.beansByName[b.name], b)
}

