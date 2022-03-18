package v1alpha1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Name      string `json:"release"`
	Namespace string `json:"namespace"`
}

type HelmReleaseStatus struct {
	ReleaseStatus bool `json:"releaseStatus"`

	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}
