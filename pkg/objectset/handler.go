package objectset

import (
	"fmt"

	"github.com/aiyengar2/helm-locker/pkg/gvk"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	SecretGVK = v1.SchemeGroupVersion.WithKind("Secret")
)

type handler struct {
	apply     apply.Apply
	gvkLister gvk.GVKLister
}

// configureApply configures the apply object for the provided setID and objectSetState
func (h *handler) configureApply(setID string, oss *objectSetState) apply.Apply {
	apply := h.apply.
		WithSetID("object-set-applier").
		WithOwnerKey(setID, internalGroupVersion.WithKind("objectSetState"))

	if oss != nil && oss.ObjectSet != nil {
		apply = apply.WithGVK(oss.ObjectSet.GVKs()...)
	} else {
		// if we cannot infer the GVK from the provided object set, include all GVKs in the cache types
		gvks, err := h.gvkLister.List()
		if err != nil {
			logrus.Errorf("unable to list GVKs to apply deletes on objects, objectset %s may require manual cleanup: %s", setID, err)
		} else {
			apply = apply.WithGVK(gvks...)
		}
	}

	return apply
}

// OnChange reconciles the resources tracked by an objectSetState
func (h *handler) OnChange(setID string, obj runtime.Object) error {
	logrus.Infof("on change: %s", setID)

	if obj == nil {
		// nothing to do
		return nil
	}
	oss, ok := obj.(*objectSetState)
	if !ok {
		return fmt.Errorf("expected object of type objectSetState, found %t", obj)
	}

	if oss.DeletionTimestamp != nil {
		return nil
	}
	if !oss.Locked {
		// nothing to do
		return nil
	}

	// Run the apply
	if err := h.configureApply(setID, oss).Apply(oss.ObjectSet); err != nil {
		return fmt.Errorf("failed to apply objectset for %s: %s", setID, err)
	}

	logrus.Infof("applied objectset %s", setID)
	return nil
}

// OnRemove cleans up the resources tracked by an objectSetState
func (h *handler) OnRemove(setID string, obj *objectSetState) {
	logrus.Infof("on delete: %s", setID)

	if obj == nil {
		return
	}

	if err := h.configureApply(setID, obj).ApplyObjects(); err != nil {
		logrus.Errorf("failed to clean up objectset %s: %s", setID, err)
	}

	logrus.Infof("applied objectset %s", setID)
}
