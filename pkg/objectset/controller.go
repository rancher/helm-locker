package objectset

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aiyengar2/helm-locker/pkg/informerfactory"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

type ObjectSetController interface {
	relatedresource.Enqueuer

	Start(ctx context.Context, workers int) error
}

type ObjectSetControllerOptions struct {
	RateLimiter workqueue.RateLimiter
}

func NewObjectSetController(name string, objectSetRegister ObjectSetRegister, apply apply.Apply, scf controller.SharedControllerFactory, opts *ObjectSetControllerOptions) ObjectSetController {
	var newOpts ObjectSetControllerOptions
	if opts != nil {
		newOpts = *opts
	}
	if newOpts.RateLimiter == nil {
		newOpts.RateLimiter = workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemFastSlowRateLimiter(time.Millisecond, 2*time.Minute, 30),
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 30*time.Second),
		)
	}
	return &objectSetController{
		name:              name,
		objectSetRegister: objectSetRegister,
		apply: apply.WithCacheTypeFactory(
			informerfactory.New(scf),
		),
		rateLimiter: newOpts.RateLimiter,
	}
}

type startKey struct {
	key   relatedresource.Key
	after time.Duration
}

type objectSetController struct {
	name string

	objectSetRegister ObjectSetRegister
	apply             apply.Apply

	workqueue   workqueue.RateLimitingInterface
	rateLimiter workqueue.RateLimiter

	started   bool
	startKeys []startKey
	startLock sync.RWMutex
}

func (c *objectSetController) Enqueue(namespace, name string) {
	key := keyFunc(namespace, name)

	c.startLock.Lock()
	defer c.startLock.Unlock()

	if c.workqueue == nil {
		c.startKeys = append(c.startKeys, startKey{key: key})
	} else {
		c.workqueue.AddRateLimited(key)
	}
}

func (c *objectSetController) Start(ctx context.Context, workers int) error {
	c.startLock.Lock()
	defer c.startLock.Unlock()

	if c.started {
		return nil
	}

	go c.run(workers, ctx.Done())
	c.started = true
	return nil
}

func (c *objectSetController) run(workers int, stopCh <-chan struct{}) {
	c.startLock.Lock()
	// we have to defer queue creation until we have a stopCh available because a workqueue
	// will create a goroutine under the hood.  It we instantiate a workqueue we must have
	// a mechanism to Shutdown it down.  Without the stopCh we don't know when to shutdown
	// the queue and release the goroutine
	c.workqueue = workqueue.NewNamedRateLimitingQueue(c.rateLimiter, c.name)
	for _, start := range c.startKeys {
		if start.after == 0 {
			c.workqueue.Add(start.key)
		} else {
			c.workqueue.AddAfter(start.key, start.after)
		}
	}
	c.startKeys = nil
	c.startLock.Unlock()

	defer utilruntime.HandleCrash()
	defer func() {
		c.workqueue.ShutDown()
	}()

	// Start the informer factories to begin populating the informer caches
	logrus.Infof("starting %s", c.name)

	// Launch workers to process reconciles
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	c.startLock.Lock()
	defer c.startLock.Unlock()
	c.started = false
	logrus.Info("shutting down %s workers", c.name)
}

func (c *objectSetController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *objectSetController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	if err := c.processSingleItem(obj); err != nil {
		if !strings.Contains(err.Error(), "please apply your changes to the latest version and try again") {
			logrus.Errorf("%v", err)
		}
		return true
	}

	return true
}

func (c *objectSetController) processSingleItem(obj interface{}) error {
	var (
		key relatedresource.Key
		ok  bool
	)

	defer c.workqueue.Done(obj)

	if key, ok = obj.(relatedresource.Key); !ok {
		c.workqueue.Forget(obj)
		logrus.Errorf("expected string in workqueue but got %#v", obj)
		return nil
	}
	if err := c.syncHandler(key); err != nil {
		c.workqueue.AddRateLimited(key)
		return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
	}

	c.workqueue.Forget(obj)
	return nil
}

func (c *objectSetController) syncHandler(key relatedresource.Key) error {
	os, exists := c.objectSetRegister.Get(key)
	if !exists {
		return nil
	}

	return c.applyOS(key, os)
}

func (c *objectSetController) applyOS(key relatedresource.Key, os *objectset.ObjectSet) error {
	if err := c.apply.WithSetID(fmt.Sprintf("%s/%s", key.Namespace, key.Name)).Apply(os); err != nil {
		logrus.Infof("failed to apply set %s: %s", key, err)
	}
	if os == nil || os.Len() == 0 {
		logrus.Infof("deleted set %s", key)
	} else {
		logrus.Infof("applied set %s", key)
	}
	return nil
}
