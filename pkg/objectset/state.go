package objectset

import (
	"github.com/rancher/wrangler/pkg/objectset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// objectSetState represents the state of an object set. This resource is only intended for
// internal use by this controller
type objectSetState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ObjectSet is a pointer to the underlying ObjectSet whose state is being tracked
	ObjectSet *objectset.ObjectSet `json:"objectSet,omitempty"`

	// Locked represents whether the ObjectSet should be locked in the cluster or not
	Locked bool `json:"locked"`
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *objectSetState) DeepCopyInto(out *objectSetState) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.ObjectSet = in.ObjectSet
	out.Locked = in.Locked
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new objectSetState.
func (in *objectSetState) DeepCopy() *objectSetState {
	if in == nil {
		return nil
	}
	out := new(objectSetState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is a deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *objectSetState) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// objectSetStateList represents a list of objectSetStates
type objectSetStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items are the objectSetStates tracked by this list
	Items []objectSetState `json:"items"`
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *objectSetStateList) DeepCopyInto(out *objectSetStateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]objectSetState, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new objectSetStateList.
func (in *objectSetStateList) DeepCopy() *objectSetStateList {
	if in == nil {
		return nil
	}
	out := new(objectSetStateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is a deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *objectSetStateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
