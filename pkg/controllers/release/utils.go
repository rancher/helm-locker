package release

import (
	"fmt"

	"github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	v1 "k8s.io/api/core/v1"
)

const (
	HelmReleaseSecretType = "helm.sh/release.v1"
)

func releaseKeyToString(key relatedresource.Key) string {
	return fmt.Sprintf("%s/%s", key.Namespace, key.Name)
}

func releaseKeyFromRelease(release *v1alpha1.HelmRelease) relatedresource.Key {
	return relatedresource.Key{
		Namespace: release.Spec.Release.Namespace,
		Name:      release.Spec.Release.Name,
	}
}

func releaseKeyFromSecret(secret *v1.Secret) *relatedresource.Key {
	if !isHelmReleaseSecret(secret) {
		return nil
	}
	releaseNameFromLabel, ok := secret.GetLabels()["name"]
	if !ok {
		return nil
	}
	return &relatedresource.Key{
		Namespace: secret.GetNamespace(),
		Name:      releaseNameFromLabel,
	}
}

func isHelmReleaseSecret(secret *v1.Secret) bool {
	return secret.Type == HelmReleaseSecretType
}
