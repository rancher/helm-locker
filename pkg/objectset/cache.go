package objectset

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aiyengar2/helm-locker/pkg/gvk"
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

// LockableObjectSetRegisters can keep track of sets of ObjectSets that need to be locked or unlocked
type LockableObjectSetRegister interface {
	relatedresource.Enqueuer

	// Lock allows you to lock an objectset associated with a specific key
	Lock(key relatedresource.Key, os *objectset.ObjectSet)

	// Unlock allows you to unlock an objectset associated with a specific key
	Unlock(key relatedresource.Key)

	// Delete allows you to delete an objectset associated with a specific key
	Delete(key relatedresource.Key)
}

// newLockableObjectSetRegisterAndCache returns a pair:
// 1) a LockableObjectSetRegister that implements the interface described above
// 2) a cache.SharedIndexInformer that listens to events on objectSetStates that are created from interacting with the provided register
//
// Note: This function is intentionally internal since the cache.SharedIndexInformer responds to an internal runtime.Object type (objectSetState)
func newLockableObjectSetRegisterAndCache(scf controller.SharedControllerFactory) (LockableObjectSetRegister, cache.SharedIndexInformer) {
	c := lockableObjectSetRegisterAndCache{
		stateByKey:            make(map[relatedresource.Key]objectSetState),
		keyByResourceKeyByGVK: make(map[schema.GroupVersionKind]map[relatedresource.Key]relatedresource.Key),

		stateChanges: make(chan watch.Event, 50),
	}
	// initialize watcher that populates watch queue
	c.gvkWatcher = gvk.NewGVKWatcher(scf, c.Resolve, &c)
	// initialize informer
	c.SharedIndexInformer = cache.NewSharedIndexInformer(&c, &objectSetState{}, 10*time.Hour, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	return &c, &c
}

// lockableObjectSetRegisterAndCache is a cache.SharedIndexInformer that operates on objectSetStates
// and implements the LockableObjectSetRegister interface via the informer
//
// internal note: also implements cache.ListerWatcher on objectSetStates
// internal note: also implements watch.Interface on objectSetStates
type lockableObjectSetRegisterAndCache struct {
	cache.SharedIndexInformer

	// stateChanges is the internal channel tracking events that happen to ObjectSetStates
	stateChanges chan watch.Event
	// gvkWatcher watches all GVKs tied to resources tracked by any ObjectSet tracked by this register
	// It will automatically trigger an Enqueue on seeing changes, which will trigger an event that
	// the underlying cache.SharedIndexInformer will process
	gvkWatcher gvk.GVKWatcher
	// started represents whether the cache has been started yet
	started bool
	// startLock is a lock that prevents a Watch from occurring before the Informer has been started
	startLock sync.RWMutex

	// stateByKey is a map that keeps track of the desired state of the ObjectSetRegister
	stateByKey map[relatedresource.Key]objectSetState
	// stateMapLock is a lock on the stateByKey map
	stateMapLock sync.RWMutex

	// keyByResourceKeyByGVK is a map that keeps track of which resources are tied to a particular ObjectSet
	// This is used to make resolving the objectset on seeing changes to underlying resources more efficient
	keyByResourceKeyByGVK map[schema.GroupVersionKind]map[relatedresource.Key]relatedresource.Key
	// keyMapLock is a lock on the keyByResourceKeyByGVK map
	keyMapLock sync.RWMutex
}

// init initializes the register and the cache
func (c *lockableObjectSetRegisterAndCache) init() {
	c.startLock.Lock()
	defer c.startLock.Unlock()
	// do not start twice
	if !c.started {
		c.started = true
	}
}

// Run starts the objectSetState informer and starts watching GVKs tracked by ObjectSets
func (c *lockableObjectSetRegisterAndCache) Run(stopCh <-chan struct{}) {
	c.init()
	err := c.gvkWatcher.Start(context.TODO(), 50)
	if err != nil {
		logrus.Errorf("unable to watch gvks: %s", err)
	}

	c.SharedIndexInformer.Run(stopCh)
}

// Stop is a noop
// Allows implementing watch.Interface on objectSetStates
func (c *lockableObjectSetRegisterAndCache) Stop() {}

// ResultChan returns the channel that watch.Events on objectSetStates are registered on
// Allows implementing watch.Interface on objectSetStates
func (c *lockableObjectSetRegisterAndCache) ResultChan() <-chan watch.Event {
	return c.stateChanges
}

// List returns an objectSetStateList
// Allows implementing cache.ListerWatcher on objectSetStates
func (c *lockableObjectSetRegisterAndCache) List(options metav1.ListOptions) (runtime.Object, error) {
	c.stateMapLock.RLock()
	defer c.stateMapLock.RUnlock()
	objectSetStateList := &objectSetStateList{}
	for _, objectSetState := range c.stateByKey {
		objectSetStateList.Items = append(objectSetStateList.Items, objectSetState)
	}
	objectSetStateList.ResourceVersion = options.ResourceVersion
	return objectSetStateList, nil
}

// List returns an watch.Interface if the cache has been started that watches for events on objectSetStates
// Allows implementing cache.ListerWatcher on objectSetStates
func (c *lockableObjectSetRegisterAndCache) Watch(options metav1.ListOptions) (watch.Interface, error) {
	c.startLock.RLock()
	defer c.startLock.RUnlock()
	if !c.started {
		return nil, fmt.Errorf("cache is not started yet")
	}
	return c, nil
}

// Lock allows you to lock an objectset associated with a specific key
func (c *lockableObjectSetRegisterAndCache) Lock(key relatedresource.Key, os *objectset.ObjectSet) {
	logrus.Infof("locking %s", key)

	locked := true
	c.setState(key, os, &locked)
	c.lock(key, os)
}

// Unlock allows you to unlock an objectset associated with a specific key
func (c *lockableObjectSetRegisterAndCache) Unlock(key relatedresource.Key) {
	logrus.Infof("unlocking %s", key)

	var locked bool
	c.setState(key, nil, &locked)
	c.unlock(key)
}

// Delete allows you to delete an objectset associated with a specific key
func (c *lockableObjectSetRegisterAndCache) Delete(key relatedresource.Key) {
	logrus.Infof("deleting %s", key)

	c.deleteState(key)
	c.unlock(key)
}

// Enqueue allows you to enqueue an objectset associated with a specific key
func (c *lockableObjectSetRegisterAndCache) Enqueue(namespace, name string) {
	key := keyFunc(namespace, name)
	logrus.Infof("enqueuing %s", key)

	c.setState(key, nil, nil)
}

// Resolve allows you to resolve an object seen in the cluster to an ObjectSet tracked in this LockableObjectSetRegister
// Objects will only be resolved if the LockableObjectSetRegister has locked this ObjectSet
func (c *lockableObjectSetRegisterAndCache) Resolve(gvk schema.GroupVersionKind, namespace, name string, _ runtime.Object) ([]relatedresource.Key, error) {
	resourceKey := keyFunc(namespace, name)

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

// getState returns the underlying objectSetState for a given key
func (c *lockableObjectSetRegisterAndCache) getState(key relatedresource.Key) (objectSetState, bool) {
	c.stateMapLock.RLock()
	defer c.stateMapLock.RUnlock()
	state, ok := c.stateByKey[key]
	return state, ok
}

// setState allows a user to set the objectSetState for a given key
func (c *lockableObjectSetRegisterAndCache) setState(key relatedresource.Key, os *objectset.ObjectSet, locked *bool) {
	// get old state and use as the base
	oldObjectSetState, modifying := c.getState(key)

	var objectMeta metav1.ObjectMeta
	oldObjectSetState.ObjectMeta.DeepCopyInto(&objectMeta)
	objectMeta.Name = key.Name
	objectMeta.Namespace = key.Namespace
	if modifying {
		objectMeta.Generation = oldObjectSetState.Generation + 1
	} else {
		// UID is tied to the address of the object that first loaded it
		// If the controller restarts, UIDs will always be recreated since
		// this controller is purely in memory
		objectMeta.UID = types.UID(fmt.Sprintf("initial-object-set-%p", os))
		objectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	}
	objectMeta.ResourceVersion = fmt.Sprintf("%d", objectMeta.Generation)

	s := objectSetState{ObjectMeta: objectMeta}
	if os == nil {
		s.ObjectSet = oldObjectSetState.ObjectSet
	} else {
		s.ObjectSet = os
	}
	if locked != nil {
		s.Locked = *locked
	} else {
		s.Locked = oldObjectSetState.Locked
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

// deleteState deletes anything on the register for a given key
func (c *lockableObjectSetRegisterAndCache) deleteState(key relatedresource.Key) {
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

// lock adds entries to the register to ensure that resources tracked by this ObjectSet are resolved to this ObjectSet
func (c *lockableObjectSetRegisterAndCache) lock(key relatedresource.Key, os *objectset.ObjectSet) error {
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
			resourceKey := keyFunc(objKey.Namespace, objKey.Name)
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

// unlock removes all entries to the register tied to a particular ObjectSet by key
func (c *lockableObjectSetRegisterAndCache) unlock(key relatedresource.Key) {
	c.keyMapLock.Lock()
	defer c.keyMapLock.Unlock()

	c.removeAllEntries(key)
}

// canLock returns whether trynig to lock the provided ObjectSet will result in an error
// One of the few reasons why this is possible is if two registered ObjectSets are attempting to track the same resource
func (c *lockableObjectSetRegisterAndCache) canLock(key relatedresource.Key, os *objectset.ObjectSet) error {
	c.keyMapLock.RLock()
	defer c.keyMapLock.RUnlock()

	objectsByGVK := os.ObjectsByGVK()
	for gvk, objMap := range objectsByGVK {
		keyByResourceKey, ok := c.keyByResourceKeyByGVK[gvk]
		if !ok {
			continue
		}
		for objKey := range objMap {
			resourceKey := keyFunc(objKey.Namespace, objKey.Name)
			currKey, ok := keyByResourceKey[resourceKey]
			if ok && currKey != key {
				// object is already associated with another set
				return fmt.Errorf("cannot lock objectset for %s: object %s is already associated with key %s", key, objKey, currKey)
			}
		}
	}
	return nil
}

// removeAllEntries removes all entries to the register tied to a particular ObjectSet by key
// Note: This is a thread-unsafe version of delete
func (c *lockableObjectSetRegisterAndCache) removeAllEntries(key relatedresource.Key) {
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
