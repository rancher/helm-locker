package v1alpha1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Helm Release Statuses

	SecretNotFoundState = "SecretNotFound"
	UnknownState        = "Unknown"
	DeployedState       = "Deployed"
	UninstalledState    = "Uninstalled"
	ErrorState          = "Error"
	FailedState         = "Failed"
	TransitioningState  = "Transitioning"
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
	State       string `json:"state,omitempty"`
	Version     int    `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Notes       string `json:"notes,omitempty"`

	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}
