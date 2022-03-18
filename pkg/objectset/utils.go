package objectset

import (
	"github.com/rancher/wrangler/pkg/relatedresource"
)

func keyFunc(namespace, name string) relatedresource.Key {
	return relatedresource.Key{
		Namespace: namespace,
		Name:      name,
	}
}
