package informerfactory

import (
	"github.com/rancher/lasso/pkg/controller"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type InformerFactory struct {
	controller.SharedControllerFactory
}

func New(scf controller.SharedControllerFactory) *InformerFactory {
	return &InformerFactory{
		SharedControllerFactory: scf,
	}
}

func (f *InformerFactory) Get(gvk schema.GroupVersionKind, _ schema.GroupVersionResource) (cache.SharedIndexInformer, error) {
	controller, err := f.ForKind(gvk)
	if err != nil {
		return nil, nil
	}
	return controller.Informer(), nil
}
