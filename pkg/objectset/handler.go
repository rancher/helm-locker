package objectset

import (
	"fmt"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// objectSetStateHandlerFunc returns a HandlerFunc built on wrangler.apply that watches for changes to
// objectSetState resources and performs an apply if the objectSetState is locked using the provided wrangler.apply
func objectSetStateHandlerFunc(apply apply.Apply) controller.HandlerFunc {
	return func(_ string, obj runtime.Object) error {
		objectSetState, ok := obj.(*objectSetState)
		if !ok {
			return fmt.Errorf("expected object of type objectSetState, found %t", obj)
		}
		if objectSetState == nil {
			// nothing to do
			return nil
		}

		logrus.Debugf("on change called: key %s/%s, locked %s, num objects %d", objectSetState.Namespace, objectSetState.Name, objectSetState.Locked, len(objectSetState.ObjectSet.All()))
		if !objectSetState.Locked {
			// nothing to do
			return nil
		}

		// Run the apply
		if err := apply.WithSetID(fmt.Sprintf("%s/%s", objectSetState.Namespace, objectSetState.Name)).Apply(objectSetState.ObjectSet); err != nil {
			return fmt.Errorf("failed to apply objectset %s/%s: %s", objectSetState.Namespace, objectSetState.Name, err)
		}

		// log some useful information
		if objectSetState.ObjectSet == nil || objectSetState.ObjectSet.Len() == 0 {
			logrus.Infof("deleted objectset %s/%s", objectSetState.Namespace, objectSetState.Name)
		} else {
			logrus.Infof("applied objectset %s/%s", objectSetState.Namespace, objectSetState.Name)
		}
		return nil
	}
}
