package restclientgetter

import (
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type restClientGetter struct {
	restConfig      *rest.Config
	cachedDiscovery discovery.CachedDiscoveryInterface
	restMapper      meta.RESTMapper
}

func New(restConfig *rest.Config, discovery discovery.DiscoveryInterface) action.RESTClientGetter {
	cachedDiscovery := memory.NewMemCacheClient(discovery)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)
	g := &restClientGetter{
		restConfig:      restConfig,
		cachedDiscovery: cachedDiscovery,
		restMapper:      restMapper,
	}
	return g
}

func (g *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.restConfig, nil
}

func (g *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return g.cachedDiscovery, nil
}

func (g *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return g.restMapper, nil
}
