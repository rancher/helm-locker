package release

import (
	"context"
	"fmt"

	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	helmcontrollers "github.com/aiyengar2/helm-locker/pkg/generated/controllers/helm.cattle.io/v1alpha1"
	"github.com/aiyengar2/helm-locker/pkg/objectset"
	"github.com/aiyengar2/helm-locker/pkg/parser"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	rspb "helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	HelmReleaseBySecret   = "v1alpha1.cattle.io/helm-release-by-secret"
	HelmReleaseSecretType = "v1alpha1.sh/release.v1"
)

type handler struct {
	systemNamespace string

	helmReleases     helmcontrollers.HelmReleaseController
	helmReleaseCache helmcontrollers.HelmReleaseCache
	secrets          corecontrollers.SecretController
	secretCache      corecontrollers.SecretCache

	objectSetParser        parser.ObjectSetParser
	lockedObjectSetManager *objectset.LockedObjectSetManager
}

func Register(
	ctx context.Context,
	systemNamespace string,
	helmReleases helmcontrollers.HelmReleaseController,
	helmReleaseCache helmcontrollers.HelmReleaseCache,
	secrets corecontrollers.SecretController,
	secretCache corecontrollers.SecretCache,
	objectSetParser parser.ObjectSetParser,
	lockedObjectSetManager *objectset.LockedObjectSetManager,
) {

	h := &handler{
		systemNamespace:  systemNamespace,
		helmReleases:     helmReleases,
		helmReleaseCache: helmReleaseCache,
		secrets:          secrets,
		secretCache:      secretCache,
	}

	helmReleaseCache.AddIndexer(HelmReleaseBySecret, h.helmReleaseBySecret)
	relatedresource.Watch(ctx, "on-helm-secret-change", h.resolveHelmRelease, helmReleases, secrets)

	helmReleases.OnChange(ctx, "apply-lock-on-release", h.OnHelmRelease)
}

func (h *handler) helmReleaseBySecret(helmRelease *v1alpha1.HelmRelease) ([]string, error) {
	if helmRelease == nil {
		return nil, nil
	}
	secretNamespace := helmRelease.Spec.Namespace
	secretName := helmRelease.Spec.Name
	return []string{
		getKey(secretNamespace, secretName),
	}, nil
}

func (h *handler) resolveHelmRelease(secretNamespace, secretName string, obj runtime.Object) ([]relatedresource.Key, error) {
	secret, ok := obj.(*v1.Secret)
	if !ok {
		return nil, nil
	}
	if secret.Type != HelmReleaseSecretType {
		return nil, nil
	}

	helmReleases, err := h.helmReleaseCache.GetByIndex(
		HelmReleaseBySecret,
		getKey(secretNamespace, secretName),
	)
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

func getKey(namespace string, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (h *handler) OnHelmRelease(key string, helmRelease *v1alpha1.HelmRelease) (*v1alpha1.HelmRelease, error) {
	if helmRelease == nil || helmRelease.DeletionTimestamp != nil {
		return nil, nil
	}
	secretNamespace := helmRelease.Spec.Namespace
	secretName := helmRelease.Spec.Name
	secretKey := relatedresource.Key{
		Namespace: secretNamespace,
		Name:      secretName,
	}
	helmReleaseSecret, err := h.secretCache.Get(secretNamespace, secretName)
	if err != nil {
		// TODO: add status
		return helmRelease, fmt.Errorf("unable to find Helm Release Secret (%s/%s) for HelmRelease %s", secretNamespace, secretName, helmRelease.GetName())
	}
	if helmReleaseSecret.Type != HelmReleaseSecretType {
		// TODO: add status
		return helmRelease, fmt.Errorf(
			"unable to parse contents of Secret (%s/%s) that HelmRelease %s points to: not a Helm secret, found type %s instead of %s",
			secretNamespace, secretName, helmRelease.GetName(), helmReleaseSecret.Type, HelmReleaseSecretType,
		)
	}
	releaseData, ok := helmReleaseSecret.Data["release"]
	if !ok {
		// TODO: add status
		return helmRelease, fmt.Errorf(
			"unable to parse contents of Secret (%s/%s) that HelmRelease %s points to: could not find key 'release' in Secret",
			secretNamespace, secretName, helmRelease.GetName(),
		)
	}
	release, err := decodeRelease(string(releaseData))
	if err != nil {
		// TODO: add status
		return helmRelease, fmt.Errorf(
			"unable to parse contents of Secret (%s/%s) that HelmRelease %s points to: could not decode contents of 'release' key in Secret",
			secretNamespace, secretName, helmRelease.GetName(),
		)
	}
	if release.Info.Status != rspb.StatusDeployed {
		// TODO: add status
		logrus.Infof("detected HelmRelease %s is not deployed (status is %s), unlocking release", helmRelease, release.Info.Status)
		h.lockedObjectSetManager.Unlock(secretKey)
		return helmRelease, nil
	}
	logrus.Infof("detected HelmRelease %s is deployed, locking release")
	manifestOS, err := h.objectSetParser.Parse(release.Manifest, parser.ObjectSetParserOptions{})
	if err != nil {
		// TODO: add status
		return helmRelease, fmt.Errorf(
			"unable to parse objectset from manifest contained in Secret (%s/%s) that HelmRelease %s points to: %s",
			secretNamespace, secretName, helmRelease.GetName(), err,
		)
	}
	if err := h.lockedObjectSetManager.Apply(secretKey, manifestOS); err != nil {
		return helmRelease, fmt.Errorf(
			"unable to apply objectset from manifest contained in Secret (%s/%s) that HelmRelease %s points to: %s",
			secretNamespace, secretName, helmRelease.GetName(), err,
		)
	}
	return helmRelease, nil
}
