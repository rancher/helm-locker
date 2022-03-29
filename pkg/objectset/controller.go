package objectset

import (
	"context"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/util/workqueue"
)

// NewLockableObjectSetRegister returns a starter that starts an ObjectSetController listening to events on ObjectSetStates
// and a LockableObjectSetRegister that allows you to register new states for ObjectSets in memory
func NewLockableObjectSetRegister(name string, apply apply.Apply, scf controller.SharedControllerFactory, opts *controller.Options) (start.Starter, LockableObjectSetRegister) {
	// Define a new cache
	register, cache := newLockableObjectSetRegisterAndCache(scf)
	startCache := func(ctx context.Context) error {
		go cache.Run(ctx.Done())
		return nil
	}
	// Define a new controller that responds to events from the cache
	objectSetController := controller.New(name, cache, startCache, objectSetStateHandlerFunc(apply), applyDefaultOptions(opts))
	return wrapStarter(objectSetController), register
}

// applyDefaultOptions applies default controller options if none are provided
func applyDefaultOptions(opts *controller.Options) *controller.Options {
	var newOpts controller.Options
	if opts != nil {
		newOpts = *opts
	}
	if newOpts.RateLimiter == nil {
		newOpts.RateLimiter = workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemFastSlowRateLimiter(time.Millisecond, 2*time.Minute, 30),
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 30*time.Second),
		)
	}
	return &newOpts
}
