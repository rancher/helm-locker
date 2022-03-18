package objectset

import (
	"context"
	"time"

	"github.com/aiyengar2/helm-locker/pkg/informerfactory"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/util/workqueue"
)

type ControllerOptions struct {
	RateLimiter workqueue.RateLimiter
}

func NewObjectSetController(name string, apply apply.Apply, scf controller.SharedControllerFactory, opts *controller.Options) (start.Starter, ObjectSetRegister) {
	// handler watches for keyStates
	h := &keyStateHandler{
		Apply: apply.WithCacheTypeFactory(informerfactory.New(scf)),
	}
	// Define a new cache
	cache := NewLockableObjectSetCache(scf)
	startCache := func(ctx context.Context) error {
		go cache.Run(ctx.Done())
		return nil
	}
	// Define a new controller that responds to events from the cache
	objectSetController := controller.New(name, cache, startCache, h, applyDefaultOptions(opts))
	return wrapStarter(objectSetController), cache
}

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
