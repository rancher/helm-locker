package release

import (
	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	"github.com/pkg/errors"

	rspb "helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
)

var (
	ReleaseInfoParseErr = errors.New("unable to get release info from secret")
)

type releaseInfo struct {
	rspb.Info

	Name      string
	Namespace string
	Version   int
	Manifest  string
}

func NewReleaseInfo(helmReleaseSecret *v1.Secret) (*releaseInfo, error) {
	if !isHelmReleaseSecret(helmReleaseSecret) {
		return nil, errors.Wrapf(ReleaseInfoParseErr, "%s/%s is not a Helm Release Secret", helmReleaseSecret.Name, helmReleaseSecret.Namespace)
	}
	releaseData, ok := helmReleaseSecret.Data["release"]
	if !ok {
		return nil, errors.Wrapf(ReleaseInfoParseErr, "secret %s/%s does not have key for 'release'", helmReleaseSecret.Name, helmReleaseSecret.Namespace)
	}
	release, err := decodeRelease(string(releaseData))
	if err != nil {
		return nil, errors.Wrapf(ReleaseInfoParseErr, "cannot decode release from contents of 'release' key in Secret %s/%s", helmReleaseSecret.Name, helmReleaseSecret.Namespace)
	}
	var info rspb.Info
	if release.Info != nil {
		info = *release.Info
	}
	return &releaseInfo{
		Info: info,

		Name:      release.Name,
		Namespace: release.Namespace,
		Version:   release.Version,
		Manifest:  release.Manifest,
	}, nil
}

func (i *releaseInfo) Locked() bool {
	return i.Status == rspb.StatusDeployed
}

func (i *releaseInfo) SetStatus(helmRelease *v1alpha1.HelmRelease) *v1alpha1.HelmRelease {
	helmRelease.Status.Version = i.Version
	helmRelease.Status.Description = i.Description
	switch i.Status {
	case rspb.StatusDeployed:
		helmRelease.Status.ReleaseStatus = "Deployed"
	case rspb.StatusFailed:
		helmRelease.Status.ReleaseStatus = "Failed"
	case rspb.StatusUninstalling, rspb.StatusPendingInstall, rspb.StatusPendingUpgrade, rspb.StatusPendingRollback:
		helmRelease.Status.ReleaseStatus = "Transitioning"
	default:
		helmRelease.Status.ReleaseStatus = "ErrInvalidRelease"
	}
	helmRelease.Status.Notes = i.Notes
	return helmRelease
}
