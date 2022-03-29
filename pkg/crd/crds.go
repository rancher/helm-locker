package crd

import (
	"context"

	v1alpha1 "github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// List returns the set of CRDs that need to be generated
func List() []crd.CRD {
	return []crd.CRD{
		newCRD(&v1alpha1.HelmRelease{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("Release Status", ".status.releaseStatus")
		}),
	}
}

// Create creates the necessary CRDs on starting this program onto the target cluster
func Create(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, List()...).BatchWait()
}

// newCRD returns the CustomResourceDefinition of an object that is customized
// according to the provided customize function
func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "helm.cattle.io",
			Version: "v1alpha1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}
