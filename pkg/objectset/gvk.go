package objectset

import (
	"context"
	"fmt"
	"sync"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GVKWatcher interface {
	Watch(ctx context.Context, gvk schema.GroupVersionKind) error
}

func NewGVKWatcher(gvkResolver GVKResolver, enqueuer relatedresource.Enqueuer, scf controller.SharedControllerFactory) GVKWatcher {
	return &gvkWatcher{
		GVKResolver:             gvkResolver,
		Enqueuer:                enqueuer,
		SharedControllerFactory: scf,
	}
}

type GVKResolver func(gvk schema.GroupVersionKind, namespace, name string, _ runtime.Object) ([]relatedresource.Key, error)

func (r GVKResolver) ForGVK(gvk schema.GroupVersionKind) relatedresource.Resolver {
	return func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		return r(gvk, namespace, name, obj)
	}
}

type gvkWatcher struct {
	GVKResolver             GVKResolver
	Enqueuer                relatedresource.Enqueuer
	SharedControllerFactory controller.SharedControllerFactory

	watchingGVK map[schema.GroupVersionKind]bool
	lock        sync.RWMutex
}

func (w *gvkWatcher) watching(gvk schema.GroupVersionKind) bool {
	w.lock.RLock()
	defer w.lock.RUnlock()
	return w.watchingGVK[gvk]
}

func (w *gvkWatcher) Watch(ctx context.Context, gvk schema.GroupVersionKind) error {
	if w.watching(gvk) {
		return nil
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	gvkController, err := w.SharedControllerFactory.ForKind(gvk)
	if err != nil {
		return err
	}
	relatedresource.Watch(ctx, fmt.Sprintf("watch-%s", gvk.GroupKind()), w.GVKResolver.ForGVK(gvk), w.Enqueuer, sharedControllerWrapper{gvkController})
	w.watchingGVK[gvk] = true
	return nil
}

type sharedControllerWrapper struct {
	controller.SharedController
}

func (s sharedControllerWrapper) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	s.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}
