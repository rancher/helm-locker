package objectset

import (
	"fmt"
	"sync"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/rancher/wrangler/pkg/relatedresource"
)

type LockedObjectSetManager struct {
	objectSetRegister   ObjectSetRegister
	objectSetController ObjectSetController
	objectSetLocker     ObjectSetLocker

	keyLockLock sync.Mutex
	keyLock     map[relatedresource.Key]*sync.Mutex
}

func NewLockedObjectSetManager(apply apply.Apply, scf controller.SharedControllerFactory) *LockedObjectSetManager {
	objectSetRegister := NewObjectSetRegister()
	objectSetController := NewObjectSetController("object-set-reconciler", objectSetRegister, apply, scf, nil)
	objectSetLocker := NewObjectSetLocker(objectSetController, scf)
	return &LockedObjectSetManager{
		objectSetRegister:   objectSetRegister,
		objectSetController: objectSetController,
		objectSetLocker:     objectSetLocker,
	}
}

func (m *LockedObjectSetManager) Apply(key relatedresource.Key, os *objectset.ObjectSet) error {
	m.initKeyLock(key)
	defer func() {
		if os == nil || os.Len() == 0 {
			m.deleteKeyLock(key)
		}
	}()

	m.keyLock[key].Lock()
	defer m.keyLock[key].Unlock()

	// remove all watchers from this key
	m.objectSetLocker.Unlock(key)

	// register the new objectset for this key
	m.objectSetRegister.Set(key, os)
	// add watchers for key
	if err := m.objectSetLocker.Lock(key, os); err != nil {
		// do not enqueue since, if we cannot lock it, the
		//  provided set is invalid
		return fmt.Errorf("could not lock object set for key %s: %s", key, err)
	}
	// enqueue once after setting watchers
	m.objectSetController.Enqueue(key.Namespace, key.Name)
	return nil
}

func (m *LockedObjectSetManager) Unlock(key relatedresource.Key) {
	m.keyLock[key].Lock()
	defer m.keyLock[key].Unlock()

	// remove all watchers from this key
	m.objectSetLocker.Unlock(key)
}

func (m *LockedObjectSetManager) initKeyLock(key relatedresource.Key) {
	m.keyLockLock.Lock()
	defer m.keyLockLock.Unlock()
	m.keyLock[key] = &sync.Mutex{}
}

func (m *LockedObjectSetManager) deleteKeyLock(key relatedresource.Key) {
	m.keyLockLock.Lock()
	defer m.keyLockLock.Unlock()
	delete(m.keyLock, key)
}
