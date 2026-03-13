package glue

import (
	"net/http"

	"github.com/pkg/errors"
)

/*
Resource types and helpers. Resources are immutable after container init.
*/
type resourceCache struct {
	sources map[string]*resourceSource
}

func ctorResourceCache() resourceCache {
	return resourceCache{
		sources: make(map[string]*resourceSource),
	}
}

type resourceSource struct {
	names     []string
	resources map[string]Resource
}

// immutable object
type resource struct {
	name   string
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
		t.resources[name] = resource{name: name, source: source.AssetFiles}
	}
	return t
}

func (t *resourceSource) merge(other *ResourceSource) error {
	for _, name := range other.AssetNames {
		if _, ok := t.resources[name]; ok {
			return errors.Errorf("resource '%s' already exist in container for resource source '%s'", name, other.Name)
		}
		t.resources[name] = resource{name: name, source: other.AssetFiles}
	}
	return nil
}

func (t *resourceCache) addResourceSource(other *ResourceSource) error {
	if rc, ok := t.sources[other.Name]; ok {
		return rc.merge(other)
	} else {
		t.sources[other.Name] = newResourceSource(other)
		return nil
	}
}

func (t *resourceCache) findResource(source, name string) (Resource, bool) {
	if src, ok := t.sources[source]; ok {
		resource, ok := src.resources[name]
		return resource, ok
	}
	return nil, false
}
