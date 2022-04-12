package release

import (
	"context"
	"fmt"

	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	helmcontrollers "github.com/aiyengar2/helm-locker/pkg/generated/controllers/helm.cattle.io/v1alpha1"
	"github.com/aiyengar2/helm-locker/pkg/objectset"
	"github.com/aiyengar2/helm-locker/pkg/objectset/parser"
	"github.com/aiyengar2/helm-locker/pkg/releases"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/storage/driver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	HelmReleaseByReleaseKey = "helm.cattle.io/helm-release-by-release-key"
)

type handler struct {
	systemNamespace string

	helmReleases     helmcontrollers.HelmReleaseController
	helmReleaseCache helmcontrollers.HelmReleaseCache
	secrets          corecontrollers.SecretController
	secretCache      corecontrollers.SecretCache

	releases releases.HelmReleaseGetter

	lockableObjectSetRegister objectset.LockableObjectSetRegister
}

func Register(
	ctx context.Context,
	systemNamespace string,
	helmReleases helmcontrollers.HelmReleaseController,
	helmReleaseCache helmcontrollers.HelmReleaseCache,
	secrets corecontrollers.SecretController,
	secretCache corecontrollers.SecretCache,
	k8s kubernetes.Interface,
	lockableObjectSetRegister objectset.LockableObjectSetRegister,
) {

	h := &handler{
		systemNamespace: systemNamespace,

		helmReleases:     helmReleases,
		helmReleaseCache: helmReleaseCache,
		secrets:          secrets,
		secretCache:      secretCache,

		releases: releases.NewHelmReleaseGetter(k8s),

		lockableObjectSetRegister: lockableObjectSetRegister,
	}

	helmReleaseCache.AddIndexer(HelmReleaseByReleaseKey, helmReleaseToReleaseKey)

	relatedresource.Watch(ctx, "on-helm-secret-change", h.resolveHelmRelease, helmReleases, secrets)

	helmReleases.OnChange(ctx, "apply-lock-on-release", h.OnHelmRelease)
	helmReleases.OnRemove(ctx, "on-remove", h.OnHelmReleaseRemove)
}

func helmReleaseToReleaseKey(helmRelease *v1alpha1.HelmRelease) ([]string, error) {
	releaseKey := releaseKeyFromRelease(helmRelease)
	return []string{releaseKeyToString(releaseKey)}, nil
}

func (h *handler) resolveHelmRelease(secretNamespace, secretName string, obj runtime.Object) ([]relatedresource.Key, error) {
	secret, ok := obj.(*v1.Secret)
	if !ok {
		return nil, nil
	}
	releaseKey := releaseKeyFromSecret(secret)
	if releaseKey == nil {
		// No release found matching this secret
		return nil, nil
	}
	helmReleases, err := h.helmReleaseCache.GetByIndex(HelmReleaseByReleaseKey, releaseKeyToString(*releaseKey))
	if err != nil {
		return nil, err
	}

	keys := make([]relatedresource.Key, len(helmReleases))
	for i, helmRelease := range helmReleases {
		keys[i] = relatedresource.Key{
			Name:      helmRelease.Name,
			Namespace: helmRelease.Namespace,
		}
	}

	return keys, nil
}

func (h *handler) OnHelmReleaseRemove(key string, helmRelease *v1alpha1.HelmRelease) (*v1alpha1.HelmRelease, error) {
	if helmRelease == nil {
		return nil, nil
	}
	if helmRelease.Namespace != h.systemNamespace {
		// do nothing if it's not in the namespace this controller was registered with
		return nil, nil
	}
	if helmRelease.Status.State == v1alpha1.SecretNotFoundState || helmRelease.Status.State == v1alpha1.UninstalledState {
		// HelmRelease was not tracking any underlying objectSet
		return nil, nil
	}
	// HelmRelease CRs are only pointers to Helm releases... if the HelmRelease CR is removed, we should do nothing, but should warn the user
	// that we are leaving behind resources in the cluster
	logrus.Warnf("HelmRelease %s/%s was removed, resources tied to Helm release may need to be manually deleted", helmRelease.Namespace, helmRelease.Name)
	logrus.Warnf("To delete the contents of a Helm release automatically, delete the Helm release secret before deleting the HelmRelease.")
	releaseKey := releaseKeyFromRelease(helmRelease)
	h.lockableObjectSetRegister.Delete(releaseKey, false) // remove the objectset, but don't purge the underlying resources
	return nil, nil
}

func (h *handler) OnHelmRelease(key string, helmRelease *v1alpha1.HelmRelease) (*v1alpha1.HelmRelease, error) {
	if helmRelease == nil || helmRelease.DeletionTimestamp != nil {
		return nil, nil
	}
	if helmRelease.Namespace != h.systemNamespace {
		// do nothing if it's not in the namespace this controller was registered with
		return nil, nil
	}
	releaseKey := releaseKeyFromRelease(helmRelease)
	latestRelease, err := h.releases.Last(releaseKey.Namespace, releaseKey.Name)
	if err != nil {
		if err == driver.ErrReleaseNotFound {
			logrus.Warnf("waiting for release %s/%s to be found to reconcile HelmRelease %s, deleting any orphaned resources", releaseKey.Namespace, releaseKey.Name, helmRelease.GetName())
			h.lockableObjectSetRegister.Delete(releaseKey, true) // remove the objectset and purge any untracked resources
			helmRelease.Status.Version = 0
			helmRelease.Status.Description = "Could not find Helm Release Secret"
			helmRelease.Status.State = v1alpha1.SecretNotFoundState
			helmRelease.Status.Notes = ""
			return h.helmReleases.UpdateStatus(helmRelease)
		}
		return helmRelease, fmt.Errorf("unable to find latest Helm Release Secret tied to Helm Release %s: %s", helmRelease.GetName(), err)
	}
	logrus.Infof("loading latest release version %d of HelmRelease %s", latestRelease.Version, helmRelease.GetName())
	releaseInfo := NewReleaseInfo(latestRelease)
	helmRelease, err = h.helmReleases.UpdateStatus(releaseInfo.GetUpdatedStatus(helmRelease))
	if err != nil {
		return helmRelease, fmt.Errorf("unable to update status of HelmRelease %s: %s", helmRelease.GetName(), err)
	}
	if !releaseInfo.Locked() {
		// TODO: add status
		logrus.Infof("detected HelmRelease %s is not deployed or transitioning (state is %s), unlocking release", helmRelease.GetName(), releaseInfo.State)
		h.lockableObjectSetRegister.Unlock(releaseKey)
		return helmRelease, nil
	}
	manifestOS, err := parser.Parse(releaseInfo.Manifest)
	if err != nil {
		// TODO: add status
		return helmRelease, fmt.Errorf("unable to parse objectset from manifest for HelmRelease %s: %s", helmRelease.GetName(), err)
	}
	logrus.Infof("detected HelmRelease %s is deployed, locking release %s with %d objects", helmRelease.GetName(), releaseKey, len(manifestOS.All()))
	locked := true
	h.lockableObjectSetRegister.Set(releaseKey, manifestOS, &locked)
	return helmRelease, nil
}
