package objectset

import (
	"fmt"

	"github.com/rancher/wrangler/pkg/apply"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

type keyStateHandler struct {
	Apply apply.Apply
}

func (h *keyStateHandler) OnChange(_ string, obj runtime.Object) error {
	keyState, ok := obj.(*KeyState)
	if !ok {
		return fmt.Errorf("expected object of type KeyState, found %t", obj)
	}
	if keyState == nil {
		return nil
	}
	logrus.Debugf("on change called: key %s/%s, locked %s, num objects %d", keyState.Namespace, keyState.Name, keyState.Locked, len(keyState.ObjectSet.All()))
	if !keyState.Locked {
		// nothing to do
		return nil
	}
	if err := h.Apply.WithSetID(fmt.Sprintf("%s/%s", keyState.Namespace, keyState.Name)).Apply(keyState.ObjectSet); err != nil {
		return fmt.Errorf("failed to apply set %s/%s: %s", keyState.Namespace, keyState.Name, err)
	}
	if keyState.ObjectSet == nil || keyState.ObjectSet.Len() == 0 {
		logrus.Infof("deleted set %s/%s", keyState.Namespace, keyState.Name)
	} else {
		logrus.Infof("applied set %s/%s", keyState.Namespace, keyState.Name)
	}
	return nil
}
