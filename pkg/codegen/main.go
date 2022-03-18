package main

import (
	"os"

	"github.com/aiyengar2/helm-locker/pkg/apis/helm.cattle.io/v1alpha1"

	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
)

func main() {
	os.Unsetenv("GOPATH")
	controllergen.Run(args.Options{
		OutputPackage: "github.com/aiyengar2/helm-locker/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"helm.cattle.io": {
				Types: []interface{}{
					v1alpha1.HelmRelease{},
				},
				GenerateTypes: true,
			},
		},
	})
}
