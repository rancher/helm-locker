package objectset

import (
	"sync"

	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/rancher/wrangler/pkg/relatedresource"
)

type ObjectSetRegister interface {
	Get(key relatedresource.Key) (*objectset.ObjectSet, bool)
	Set(key relatedresource.Key, os *objectset.ObjectSet)
}

func NewObjectSetRegister() ObjectSetRegister {
	return &objectSetRegister{
		osByKey: make(map[relatedresource.Key]*objectset.ObjectSet),
	}
}

type objectSetRegister struct {
	osByKey map[relatedresource.Key]*objectset.ObjectSet
	mapLock sync.RWMutex
}

func (r *objectSetRegister) Get(key relatedresource.Key) (*objectset.ObjectSet, bool) {
	r.mapLock.RLock()
	defer r.mapLock.RUnlock()
	os, ok := r.osByKey[key]
	return os, ok
}

func (r *objectSetRegister) Set(key relatedresource.Key, os *objectset.ObjectSet) {
	if os == nil || os.Len() == 0 {
		// set contains nothing, so it should be deleted
		r.delete(key)
		return
	}
	r.mapLock.Lock()
	defer r.mapLock.Unlock()
	r.osByKey[key] = os
}

func (r *objectSetRegister) delete(key relatedresource.Key) {
	r.mapLock.Lock()
	defer r.mapLock.Unlock()
	delete(r.osByKey, key)
}
