package objectset

import (
	"context"
	"fmt"
	"sync"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ObjectSetLocker interface {
	Lock(key relatedresource.Key, os *objectset.ObjectSet) error
	Unlock(key relatedresource.Key)
}

func NewObjectSetLocker(enqueuer relatedresource.Enqueuer, scf controller.SharedControllerFactory) ObjectSetLocker {
	l := objectSetLocker{
		keyByResourceKeyByGVK: make(map[schema.GroupVersionKind]map[relatedresource.Key]relatedresource.Key),
	}
	l.gvkWatcher = NewGVKWatcher(l.resolver, enqueuer, scf)
	return &l
}

type objectSetLocker struct {
	gvkWatcher GVKWatcher

	keyByResourceKeyByGVK map[schema.GroupVersionKind]map[relatedresource.Key]relatedresource.Key
	mapLock               sync.RWMutex
}

func (l *objectSetLocker) Lock(key relatedresource.Key, os *objectset.ObjectSet) error {
	if err := l.canLock(key, os); err != nil {
		return err
	}

	l.mapLock.Lock()
	defer l.mapLock.Unlock()

	l.removeAllEntries(key)

	for gvk, objMap := range os.ObjectsByGVK() {
		keyByResourceKey, ok := l.keyByResourceKeyByGVK[gvk]
		if !ok {
			keyByResourceKey = make(map[relatedresource.Key]relatedresource.Key)
		}
		for objKey := range objMap {
			resourceKey := relatedresource.Key{
				Name:      objKey.Name,
				Namespace: objKey.Namespace,
			}
			keyByResourceKey[resourceKey] = key
		}
		l.keyByResourceKeyByGVK[gvk] = keyByResourceKey

		// ensure that we are watching this new GVK
		if err := l.gvkWatcher.Watch(context.TODO(), gvk); err != nil {
			return err
		}
	}

	return nil
}

func (l *objectSetLocker) Unlock(key relatedresource.Key) {
	l.mapLock.Lock()
	defer l.mapLock.Unlock()

	l.removeAllEntries(key)
}

func (l *objectSetLocker) resolver(gvk schema.GroupVersionKind, namespace, name string, _ runtime.Object) ([]relatedresource.Key, error) {
	resourceKey := keyFunc(namespace, name)

	l.mapLock.RLock()
	defer l.mapLock.RUnlock()
	keyByResourceKey, ok := l.keyByResourceKeyByGVK[gvk]
	if !ok {
		// do nothing since we're not watching this GVK anymore
		return nil, nil
	}
	key, ok := keyByResourceKey[resourceKey]
	if !ok {
		// do nothing since the resource is not tied to a set
		return nil, nil
	}
	return []relatedresource.Key{key}, nil
}

func (l *objectSetLocker) canLock(key relatedresource.Key, os *objectset.ObjectSet) error {
	l.mapLock.RLock()
	defer l.mapLock.RUnlock()

	for gvk, objMap := range os.ObjectsByGVK() {
		keyByResourceKey, ok := l.keyByResourceKeyByGVK[gvk]
		if !ok {
			continue
		}
		for objKey := range objMap {
			resourceKey := relatedresource.Key{
				Name:      objKey.Name,
				Namespace: objKey.Namespace,
			}
			currKey, ok := keyByResourceKey[resourceKey]
			if ok && currKey != key {
				// object is already associated with another set
				return fmt.Errorf("cannot lock objectset for %s: object %s is already associated with key %s", key, objKey, currKey)
			}
		}
	}
	return nil
}

func (l *objectSetLocker) removeAllEntries(key relatedresource.Key) {
	for gvk, keyByResourceKey := range l.keyByResourceKeyByGVK {
		for resourceKey, currSetKey := range keyByResourceKey {
			if key == currSetKey {
				delete(keyByResourceKey, resourceKey)
			}
		}
		if len(keyByResourceKey) == 0 {
			delete(l.keyByResourceKeyByGVK, gvk)
		} else {
			l.keyByResourceKeyByGVK[gvk] = keyByResourceKey
		}
	}
}
