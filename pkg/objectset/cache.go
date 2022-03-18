package objectset

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type LockableObjectSetCache interface {
	cache.ListerWatcher
	watch.Interface
	cache.SharedIndexInformer
	ObjectSetRegister
}

type ObjectSetRegister interface {
	Lock(key relatedresource.Key, os *objectset.ObjectSet)
	Unlock(key relatedresource.Key)
	Delete(key relatedresource.Key)
}

func NewLockableObjectSetCache(scf controller.SharedControllerFactory) LockableObjectSetCache {
	c := lockableObjectSetCache{}
	// initialize maps
	c.stateByKey = make(map[relatedresource.Key]KeyState)
	c.keyByResourceKeyByGVK = make(map[schema.GroupVersionKind]map[relatedresource.Key]relatedresource.Key)
	// initialize watch queue
	c.stateChanges = make(chan watch.Event, 50)
	// initialize watcher that populates watch queue
	c.gvkWatcher = NewGVKWatcher(c.Resolve, &c, scf)
	// initialize informer
	c.SharedIndexInformer = cache.NewSharedIndexInformer(&c, &KeyState{}, 10*time.Hour, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	return &c
}

type lockableObjectSetCache struct {
	cache.SharedIndexInformer

	stateChanges chan watch.Event
	gvkWatcher   GVKWatcher
	started      bool
	startLock    sync.RWMutex

	stateByKey   map[relatedresource.Key]KeyState
	stateMapLock sync.RWMutex

	keyByResourceKeyByGVK map[schema.GroupVersionKind]map[relatedresource.Key]relatedresource.Key
	keyMapLock            sync.RWMutex
}

func (c *lockableObjectSetCache) init() {
	c.startLock.Lock()
	defer c.startLock.Unlock()
	// do not start twice
	if !c.started {
		c.started = true
	}
}

func (c *lockableObjectSetCache) Run(stopCh <-chan struct{}) {
	c.init()
	err := c.gvkWatcher.Start(context.TODO(), 50)
	if err != nil {
		logrus.Errorf("unable to watch gvks: %s", err)
	}

	c.SharedIndexInformer.Run(stopCh)
}

func (c *lockableObjectSetCache) Stop() {}

func (c *lockableObjectSetCache) ResultChan() <-chan watch.Event {
	return c.stateChanges
}

func (c *lockableObjectSetCache) List(options metav1.ListOptions) (runtime.Object, error) {
	c.stateMapLock.RLock()
	defer c.stateMapLock.RUnlock()
	keyStateList := &KeyStateList{}
	for _, keyState := range c.stateByKey {
		keyStateList.Items = append(keyStateList.Items, keyState)
	}
	keyStateList.ResourceVersion = options.ResourceVersion
	return keyStateList, nil
}

func (c *lockableObjectSetCache) Watch(options metav1.ListOptions) (watch.Interface, error) {
	c.startLock.RLock()
	defer c.startLock.RUnlock()
	if !c.started {
		return nil, fmt.Errorf("cache is not started yet")
	}
	return c, nil
}

func (c *lockableObjectSetCache) Lock(key relatedresource.Key, os *objectset.ObjectSet) {
	logrus.Infof("locking %s", key)

	locked := true
	c.setState(key, os, &locked)
	c.lock(key, os)
}

func (c *lockableObjectSetCache) Unlock(key relatedresource.Key) {
	logrus.Infof("unlocking %s", key)

	var locked bool
	c.setState(key, nil, &locked)
	c.unlock(key)
}

func (c *lockableObjectSetCache) Delete(key relatedresource.Key) {
	logrus.Infof("deleting %s", key)

	c.deleteState(key)
	c.unlock(key)
}

func (c *lockableObjectSetCache) Enqueue(namespace, name string) {
	key := relatedresource.Key{
		Namespace: namespace,
		Name:      name,
	}
	logrus.Infof("enqueuing %s", key)

	c.setState(key, nil, nil)
}

func (c *lockableObjectSetCache) Resolve(gvk schema.GroupVersionKind, namespace, name string, _ runtime.Object) ([]relatedresource.Key, error) {
	resourceKey := relatedresource.Key{
		Namespace: namespace,
		Name:      name,
	}

	c.keyMapLock.RLock()
	defer c.keyMapLock.RUnlock()
	keyByResourceKey, ok := c.keyByResourceKeyByGVK[gvk]
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

func (c *lockableObjectSetCache) getState(key relatedresource.Key) (KeyState, bool) {
	c.stateMapLock.RLock()
	defer c.stateMapLock.RUnlock()
	state, ok := c.stateByKey[key]
	return state, ok
}

func (c *lockableObjectSetCache) setState(key relatedresource.Key, os *objectset.ObjectSet, locked *bool) {
	// get old state and use as the base
	oldKeyState, modifying := c.getState(key)

	var objectMeta metav1.ObjectMeta
	oldKeyState.ObjectMeta.DeepCopyInto(&objectMeta)
	objectMeta.Name = key.Name
	objectMeta.Namespace = key.Namespace
	if modifying {
		objectMeta.Generation = oldKeyState.Generation + 1
	} else {
		// UID is tied to the address of the object that first loaded it
		// If the controller restarts, UIDs will always be recreated since
		// this controller is purely in memory
		objectMeta.UID = types.UID(fmt.Sprintf("initial-object-set-%p", os))
		objectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	}
	objectMeta.ResourceVersion = fmt.Sprintf("%d", objectMeta.Generation)

	s := KeyState{ObjectMeta: objectMeta}
	if os == nil {
		s.ObjectSet = oldKeyState.ObjectSet
	} else {
		s.ObjectSet = os
	}
	if locked != nil {
		s.Locked = *locked
	} else {
		s.Locked = oldKeyState.Locked
	}
	c.stateMapLock.Lock()
	defer c.stateMapLock.Unlock()
	if modifying {
		c.stateChanges <- watch.Event{Type: watch.Modified, Object: &s}
	} else {
		c.stateChanges <- watch.Event{Type: watch.Added, Object: &s}
	}
	c.stateByKey[key] = s
	logrus.Debugf("set state for %s/%s: locked %t, os %p, objectMeta: %v", s.Namespace, s.Name, s.Locked, s.ObjectSet, s.ObjectMeta)
}

func (c *lockableObjectSetCache) deleteState(key relatedresource.Key) {
	s, exists := c.getState(key)
	if !exists {
		// nothing to add, event was already processed
		return
	}
	deletionTime := metav1.NewTime(time.Now())
	s.DeletionTimestamp = &deletionTime
	c.stateMapLock.Lock()
	defer c.stateMapLock.Unlock()
	c.stateChanges <- watch.Event{Type: watch.Deleted, Object: &s}
	delete(c.stateByKey, key)
}

func (c *lockableObjectSetCache) lock(key relatedresource.Key, os *objectset.ObjectSet) error {
	if err := c.canLock(key, os); err != nil {
		return err
	}

	c.keyMapLock.Lock()
	defer c.keyMapLock.Unlock()

	c.removeAllEntries(key)

	objectsByGVK := os.ObjectsByGVK()

	for gvk, objMap := range objectsByGVK {
		keyByResourceKey, ok := c.keyByResourceKeyByGVK[gvk]
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
		c.keyByResourceKeyByGVK[gvk] = keyByResourceKey

		// ensure that we are watching this new GVK
		if err := c.gvkWatcher.Watch(gvk); err != nil {
			return err
		}
	}

	return nil
}

func (c *lockableObjectSetCache) unlock(key relatedresource.Key) {
	c.keyMapLock.Lock()
	defer c.keyMapLock.Unlock()

	c.removeAllEntries(key)
}

func (c *lockableObjectSetCache) canLock(key relatedresource.Key, os *objectset.ObjectSet) error {
	c.keyMapLock.RLock()
	defer c.keyMapLock.RUnlock()

	objectsByGVK := os.ObjectsByGVK()
	for gvk, objMap := range objectsByGVK {
		keyByResourceKey, ok := c.keyByResourceKeyByGVK[gvk]
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

func (c *lockableObjectSetCache) removeAllEntries(key relatedresource.Key) {
	for gvk, keyByResourceKey := range c.keyByResourceKeyByGVK {
		for resourceKey, currSetKey := range keyByResourceKey {
			if key == currSetKey {
				delete(keyByResourceKey, resourceKey)
			}
		}
		if len(keyByResourceKey) == 0 {
			delete(c.keyByResourceKeyByGVK, gvk)
		} else {
			c.keyByResourceKeyByGVK[gvk] = keyByResourceKey
		}
	}
}
