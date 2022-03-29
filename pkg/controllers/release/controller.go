package release

import (
	"context"
	"fmt"
	"strconv"

	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	helmcontrollers "github.com/aiyengar2/helm-locker/pkg/generated/controllers/helm.cattle.io/v1alpha1"
	"github.com/aiyengar2/helm-locker/pkg/objectset"
	"github.com/aiyengar2/helm-locker/pkg/objectset/parser"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	HelmReleaseByReleaseKey = "v1alpha1.cattle.io/helm-release-by-release-key"
	HelmSecretByReleaseKey  = "v1alpha1.cattle.io/helm-secret-by-release-key"
)

type handler struct {
	systemNamespace string

	helmReleases     helmcontrollers.HelmReleaseController
	helmReleaseCache helmcontrollers.HelmReleaseCache
	secrets          corecontrollers.SecretController
	secretCache      corecontrollers.SecretCache

	lockableObjectSetRegister objectset.LockableObjectSetRegister
}

func Register(
	ctx context.Context,
	systemNamespace string,
	helmReleases helmcontrollers.HelmReleaseController,
	helmReleaseCache helmcontrollers.HelmReleaseCache,
	secrets corecontrollers.SecretController,
	secretCache corecontrollers.SecretCache,
	lockableObjectSetRegister objectset.LockableObjectSetRegister,
) {

	h := &handler{
		systemNamespace: systemNamespace,

		helmReleases:     helmReleases,
		helmReleaseCache: helmReleaseCache,
		secrets:          secrets,
		secretCache:      secretCache,

		lockableObjectSetRegister: lockableObjectSetRegister,
	}

	secretCache.AddIndexer(HelmSecretByReleaseKey, secretsToReleaseKey)
	helmReleaseCache.AddIndexer(HelmReleaseByReleaseKey, helmReleaseToReleaseKey)

	relatedresource.Watch(ctx, "on-helm-secret-change", h.resolveHelmRelease, helmReleases, secrets)

	helmReleases.OnChange(ctx, "apply-lock-on-release", h.OnHelmRelease)
	helmReleases.OnRemove(ctx, "on-remove", h.OnHelmReleaseRemove)
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
	releaseKey := releaseKeyFromRelease(helmRelease)
	h.lockableObjectSetRegister.Delete(releaseKey)
	return nil, nil
}

func (h *handler) OnHelmRelease(key string, helmRelease *v1alpha1.HelmRelease) (*v1alpha1.HelmRelease, error) {
	if helmRelease == nil || helmRelease.DeletionTimestamp != nil {
		return nil, nil
	}
	releaseKey := releaseKeyFromRelease(helmRelease)
	helmReleaseSecrets, err := h.secretCache.GetByIndex(HelmSecretByReleaseKey, releaseKeyToString(releaseKey))
	if err != nil {
		return helmRelease, fmt.Errorf("unable to find Helm Release Secret tied to Helm Release %s: %s", helmRelease.GetName(), err)
	}
	if len(helmReleaseSecrets) == 0 {
		return helmRelease, fmt.Errorf("could not find any Helm Release Secrets tied to HelmRelease %s", helmRelease.GetName())
	}
	var helmReleaseSecret *v1.Secret
	var latestVersion, currVersion int
	for _, secret := range helmReleaseSecrets {
		version, ok := secret.Labels["version"]
		if !ok {
			// ignore if the version is not set, which is unexpected
			logrus.Debugf("could not identify release version tied to Helm release secret %s/%s: version label does not exist", secret.GetNamespace(), secret.GetName())
			continue
		}
		currVersion, err = strconv.Atoi(version)
		if err != nil {
			logrus.Debugf("could not identify release version tied to Helm release secret %s/%s: %s", secret.GetNamespace(), secret.GetName(), err)
		}
		if currVersion > latestVersion {
			latestVersion = currVersion
			helmReleaseSecret = secret
		}
	}
	logrus.Infof("loading latest release version %d of HelmRelease %s", currVersion, helmRelease.GetName())
	releaseInfo, err := NewReleaseInfo(helmReleaseSecret)
	if err != nil {
		return helmRelease, err
	}
	helmRelease, err = h.helmReleases.UpdateStatus(releaseInfo.SetStatus(helmRelease))
	if err != nil {
		return helmRelease, fmt.Errorf("unable to update status of HelmRelease %s: %s", helmRelease.GetName(), err)
	}
	if !releaseInfo.Locked() {
		// TODO: add status
		logrus.Infof("detected HelmRelease %s is not deployed (status is %s), unlocking release", helmRelease.GetName(), releaseInfo.Status)
		h.lockableObjectSetRegister.Unlock(releaseKey)
		return helmRelease, nil
	}
	manifestOS, err := parser.Parse(releaseInfo.Manifest)
	if err != nil {
		// TODO: add status
		return helmRelease, fmt.Errorf("unable to parse objectset from manifest for HelmRelease %s: %s", helmRelease.GetName(), err)
	}
	logrus.Infof("detected HelmRelease %s is deployed, locking release %s with %d objects", helmRelease.GetName(), releaseKey, len(manifestOS.All()))
	h.lockableObjectSetRegister.Lock(releaseKey, manifestOS)
	return helmRelease, nil
}
