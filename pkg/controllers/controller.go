package controllers

import (
	"context"
	"time"

	"github.com/aiyengar2/helm-locker/pkg/controllers/release"
	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/generated/controllers/helm.cattle.io"
	helmcontrollers "github.com/aiyengar2/helm-locker/pkg/generated/controllers/helm.cattle.io/v1alpha1"
	"github.com/aiyengar2/helm-locker/pkg/objectset"
	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generated/controllers/apps"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

type appContext struct {
	helmcontrollers.Interface

	K8s  kubernetes.Interface
	Core corecontrollers.Interface

	Apply             apply.Apply
	ObjectSetRegister objectset.LockableObjectSetRegister

	ClientConfig            clientcmd.ClientConfig
	Discovery               *discovery.DiscoveryClient
	SharedControllerFactory controller.SharedControllerFactory
	starters                []start.Starter
}

func (a *appContext) start(ctx context.Context) error {
	return start.All(ctx, 50, a.starters...)
}

func Register(ctx context.Context, systemNamespace string, cfg clientcmd.ClientConfig) error {
	appCtx, err := newContext(ctx, cfg)
	if err != nil {
		return err
	}

	if err := addData(systemNamespace, appCtx); err != nil {
		return err
	}

	// TODO: Register all controllers
	release.Register(ctx,
		systemNamespace,
		appCtx.HelmRelease(),
		appCtx.HelmRelease().Cache(),
		appCtx.Core.Secret(),
		appCtx.Core.Secret().Cache(),
		appCtx.ObjectSetRegister,
	)

	leader.RunOrDie(ctx, systemNamespace, "helm-locker-lock", appCtx.K8s, func(ctx context.Context) {
		if err := appCtx.start(ctx); err != nil {
			logrus.Fatal(err)
		}
		logrus.Info("All controllers have been started")
	})

	return nil
}

func controllerFactory(rest *rest.Config) (controller.SharedControllerFactory, error) {
	rateLimit := workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 60*time.Second)
	workqueue.DefaultControllerRateLimiter()
	clientFactory, err := client.NewSharedClientFactory(rest, nil)
	if err != nil {
		return nil, err
	}

	cacheFactory := cache.NewSharedCachedFactory(clientFactory, nil)
	return controller.NewSharedControllerFactory(cacheFactory, &controller.SharedControllerFactoryOptions{
		DefaultRateLimiter: rateLimit,
		DefaultWorkers:     50,
	}), nil
}

func newContext(ctx context.Context, cfg clientcmd.ClientConfig) (*appContext, error) {
	client, err := cfg.ClientConfig()
	if err != nil {
		return nil, err
	}
	client.RateLimiter = ratelimit.None

	k8s, err := kubernetes.NewForConfig(client)
	if err != nil {
		return nil, err
	}

	discovery, err := discovery.NewDiscoveryClientForConfig(client)
	if err != nil {
		return nil, err
	}

	scf, err := controllerFactory(client)
	if err != nil {
		return nil, err
	}

	core, err := core.NewFactoryFromConfigWithOptions(client, &core.FactoryOptions{
		SharedControllerFactory: scf,
	})
	if err != nil {
		return nil, err
	}
	corev := core.Core().V1()

	helm, err := v1alpha1.NewFactoryFromConfigWithOptions(client, &apps.FactoryOptions{
		SharedControllerFactory: scf,
	})
	if err != nil {
		return nil, err
	}
	helmv := helm.Helm().V1alpha1()

	apply := apply.New(discovery, apply.NewClientFactory(client))

	objectSet, objectSetRegister := objectset.NewLockableObjectSetRegister("object-set-register", apply, scf, discovery, nil)

	return &appContext{
		Interface: helmv,

		K8s:  k8s,
		Core: corev,

		Apply:             apply,
		ObjectSetRegister: objectSetRegister,

		ClientConfig:            cfg,
		SharedControllerFactory: scf,
		Discovery:               discovery,
		starters: []start.Starter{
			objectSet,
			core,
			helm,
		},
	}, nil
}