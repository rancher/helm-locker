package release

import (
	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func secretsToReleaseKey(secret *v1.Secret) ([]string, error) {
	releaseKey := releaseKeyFromSecret(secret)
	if releaseKey == nil {
		return nil, nil
	}
	return []string{releaseKeyToString(*releaseKey)}, nil
}

func helmReleaseToReleaseKey(helmRelease *v1alpha1.HelmRelease) ([]string, error) {
	releaseKey := releaseKeyFromRelease(helmRelease)
	return []string{releaseKeyToString(releaseKey)}, nil
}
