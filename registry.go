/**
  Copyright (c) 2022 Zander Schwid & Co. LLC. All rights reserved.
*/

package glue

import (
	"github.com/pkg/errors"
	"net/http"
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
	resourceSources map[string]*resourceSource
}

type resourceSource struct {
	names []string
	resources map[string]Resource
}

// immutable object
type resource struct {
	name string
	source http.FileSystem
}

// immutable object
func (t resource) Open() (http.File, error) {
	return t.source.Open(t.name)
}

func newResourceSource(source *ResourceSource) *resourceSource {
	t := &resourceSource{
		resources: make(map[string]Resource),
	}
	for _, name := range source.AssetNames {
		t.resources[name] = resource{ name: name, source: source.AssetFiles }
	}
	return t
}

func (t *resourceSource) merge(other *ResourceSource) error {
	for _, name := range other.AssetNames {
		if _, ok := t.resources[name]; ok {
			return errors.Errorf("resource '%s' already exist in context for resource source '%s'", name, other.Name)
		}
		t.resources[name] = resource{ name: name, source: other.AssetFiles }
	}
	return nil
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

func (t *registry) findResource(source, name string) (Resource, bool) {
	t.RLock()
	defer t.RUnlock()
	if source, ok := t.resourceSources[source]; ok {
		resource, ok := source.resources[name]
		return resource, ok
	}
	return nil, false
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

func (t *registry) addResourceSource(other *ResourceSource) error {
	t.Lock()
	defer t.Unlock()
	if rc, ok := t.resourceSources[other.Name]; ok {
		return rc.merge(other)
	} else {
		t.resourceSources[other.Name] = newResourceSource(other)
		return nil
	}
}
