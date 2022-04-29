# Developing Helm Locker

## Repository Structure

```bash
## This directory contains Helm charts that can be used to deploy Helm Locker in a Kubernetes cluster in the cattle-helm-system namespace
##
## By default, you should always install the Helm Locker CRD chart before installing the main Helm Locker chart.
charts/
  
  ## The CRD chart that installs the HelmRelease CRD. This must be installed before installing all other charts.
  helm-locker-crd/

  ## The main chart that deploys Helm Locker in the cluster.
  ##
  ## Depends on 'helm-locker-crd' being deployed onto the cluster first.
  helm-locker/
  
  ## A dummy chart that can be deployed as a Helm release in the cluster under the release name 'helm-locker-example' and the namespace 'cattle-helm-system'
  ##
  ## By default, it deploys with a HelmRelease CR that targets itself.
  ##
  ## Depends on 'helm-locker-crd' and 'helm-locker' being deployed onto the cluster first.
  helm-locker-example/

## This directory will contain additional docs to assist users in getting started with using Helm Locker
docs/

## This directory contains the image that is used to build rancher/helm-locker, which is hosted on hub.docker.com
package/
  Dockerfile

## The main source directory for the code. See below for more details.
pkg/

## The Dockerfile used to run CI and other scripts executed by make in a Docker container (powered by https://github.com/rancher/dapper)
Dockerfile.dapper

## The file that contains the underlying actions that 'go generate' needs to execute on a call to it. Includes the logic for generating controllers and updating the CRD packaged into the CRD chart
generate.go

## The main entrypoint into HelmLocker
main.go
```

## Making changes to the codebase (`pkg`)

Most of the code for Helm Locker is contained in the `pkg` directory, which has the following structure:

```bash
## This directory contains the definition of a HelmRelease CR under release.go; if you need to add new fields to HelmRelease CRs, this is where you would make the change
apis/

## These directories manage all the logic around 'go generate', including the creation of the 'generated/' directory that contains all the underlying controllers that are auto-generated based on the API definition of the HelmRelease CR defined under 'apis/'
codegen/
crd/
version/
generated/

## These directories are the core controller directories that manage how the operator watches HelmReleases and executes operations on the underlying in-memory ObjectSet LockableRegister (Lock, Unlock, Set, Delete)
controllers/
  ## This directory is where logic is defined for watching Helm Release Secrets targeted by HelmReleases and automatically keeping resources locked or unlocked
  release/
  ## This is where the underlying context used by all controllers of this operator are registered, all using the same underlying SharedControllerFactory
  controller.go
## A utility package to help wrap getting Helm releases via Helm library calls
releases/

## These directories implement an object that satisfies the LockableRegister interface; it is used as an underlying set of libraries that Helm Locker calls upon to achieve locking or unlocking HelmReleases (tracked as ObjectSets, or a []runtime.Object) and dynamically starting controllers based on GVKs observed in tracked object sets
gvk/
informerfactory/
objectset/
```

## Once you have made a change

If you modified `pkg/apis` or `generate.go`, make sure you run `go generate`.

Also, make sure you run `go mod tidy`.