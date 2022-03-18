package objectset

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GVKResolver func(gvk schema.GroupVersionKind, namespace, name string, _ runtime.Object) ([]relatedresource.Key, error)

func (r GVKResolver) ForGVK(gvk schema.GroupVersionKind) relatedresource.Resolver {
	return func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		logrus.Infof("called resolver for gvk %s", gvk)
		return r(gvk, namespace, name, obj)
	}
}

type GVKWatcher interface {
	Start(ctx context.Context, workers int) error
	Watch(gvk schema.GroupVersionKind) error
}

func NewGVKWatcher(gvkResolver GVKResolver, enqueuer relatedresource.Enqueuer, scf controller.SharedControllerFactory) GVKWatcher {
	return &gvkWatcher{
		GVKResolver:             gvkResolver,
		Enqueuer:                enqueuer,
		SharedControllerFactory: scf,

		gvkRegistered: make(map[schema.GroupVersionKind]bool),
		gvkStarted:    make(map[schema.GroupVersionKind]bool),
	}
}

type gvkWatcher struct {
	GVKResolver             GVKResolver
	Enqueuer                relatedresource.Enqueuer
	SharedControllerFactory controller.SharedControllerFactory

	gvkRegistered map[schema.GroupVersionKind]bool
	gvkStarted    map[schema.GroupVersionKind]bool

	started           bool
	controllerCtx     context.Context
	controllerWorkers int

	lock sync.RWMutex
}

func (w *gvkWatcher) Watch(gvk schema.GroupVersionKind) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.gvkRegistered[gvk] = true
	return w.startGVK(gvk)
}

func (w *gvkWatcher) Start(ctx context.Context, workers int) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.started = true
	w.controllerCtx = ctx
	w.controllerWorkers = workers
	var multierr error
	for gvk := range w.gvkRegistered {
		if err := w.startGVK(gvk); err != nil {
			multierr = multierror.Append(multierr, err)
		}
	}
	return multierr
}

func (w *gvkWatcher) startGVK(gvk schema.GroupVersionKind) error {
	if !w.started {
		return nil
	}
	if _, ok := w.gvkStarted[gvk]; ok {
		// gvk was already started
		return nil
	}
	gvkController, err := w.SharedControllerFactory.ForKind(gvk)
	if err != nil {
		return err
	}

	// NOTE: The order here is important of calling the watch before starting the controller.
	//
	// By default, the controller returned by a shared controller factory is a deferred controller
	// that won't populate the actual underlying controller until at least one function is called on
	// the controller (e.g. Enqueue, EnqueueAfter, EnqueueKey, Informer, or RegisterHandler)
	//
	// Therefore, running Start on an empty controller will result in the controller never registering
	// the relatederesource.Watch we provide here, since the underlying informer is nil.
	relatedresource.Watch(
		w.controllerCtx,
		fmt.Sprintf("%s Watcher", gvk),
		w.GVKResolver.ForGVK(gvk),
		w.Enqueuer,
		sharedControllerWrapper{gvkController},
	)

	if err := gvkController.Start(w.controllerCtx, w.controllerWorkers); err != nil {
		return err
	}
	w.gvkStarted[gvk] = true
	return nil
}

type sharedControllerWrapper struct {
	controller.SharedController
}

func (s sharedControllerWrapper) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	logrus.Infof("Starting %s", name)
	s.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}
