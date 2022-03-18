package v1alpha1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rspb "helm.sh/helm/v3/pkg/release"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HelmReleaseSpec   `json:"spec"`
	Status            HelmReleaseStatus `json:"status"`
}

type HelmReleaseSpec struct {
	Release ReleaseKey `json:"release,omitempty"`
}

type ReleaseKey struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type HelmReleaseStatus struct {
	Version     int         `json:"version,omitempty"`
	Description string      `json:"description,omitempty"`
	Status      rspb.Status `json:"status,omitempty"`
	Notes       string      `json:"notes,omitempty"`

	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}
