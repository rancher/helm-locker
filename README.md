helm-locker
========

Helm Locker is a Kubernetes operator that prevents resource drift on (i.e. "locks") Kubernetes objects that are tracked by Helm 3 releases.

Once installed, a user can create a `HelmRelease` CR in the `Helm Release Registration Namespace` (default: `cattle-helm-system`) by providing:
1. The name of a Helm 3 release
2. The namespace that contains the Helm Release Secret (supplied as `--namespace` on the `helm install` command that created the release)

Once created, the Helm Locker controllers will watch all resources tracked by the Helm Release Secret and automatically revert any changes to the persisted resources that were not made through Helm (e.g. changes that were directly applied via `kubectl` or other controllers).

## Who needs Helm Locker?

Anyone who would like to declaratively manage resources deployed by existing Helm chart releases.

## How is this different from projects like `fluxcd/helm-controller`?

Projects like [`fluxcd/helm-controller`](https://github.com/fluxcd/helm-controller) allow users to declaratively manage **Helm chart releases**, whereas this project only allows you to manage the **resources** deployed by those Helm chart releases; as a result, the scope of this project is much more narrow than what is offered by `fluxcd/helm-controller` and should be integrable with any solution that produces Helm releases.

If you are looking for a larger, more opinionated solution that also has features around **how** Helm charts should be deployed onto a cluster (e.g. from a `GitRepository` or `Bucket` or `HelmRepository`), this is not the project for you.

However, if you are looking for something light-weight that simply guarentees that **Helm is the only way to modify resources tracked by Helm releases**, this is a good solution to use.

## How does Helm Locker know whether a release was changed by Helm or by another source?

In order to prevent multiple Helm instances from performing the same upgrade at the same time, Helm 3 will always first update the `info.status` field on a Helm Release Secret from `deployed` to another state (e.g. `pending-upgrade`, `pending-install`, `uninstalling`, etc.) before performing the operation; once the operation is complete, the Helm Release Secret is expected to be reverted back to `deployed`.

Therefore, if Helm Locker observes a Helm Release Secret tied to a `HelmRelease` has been updated, it will check to see what the current status of the release is; if the release is anything but `deployed`, Helm Locker will not perform any operations on the resources tracked by this release, which will allow upgrades to occur as expected. 

However, once a release is `deployed`, if what is tracked in the Helm secret is different than what is currently installed onto the cluster, Helm Locker will revert all resources back to what was tracked by the Helm release (in case a change was made to the resource tracked by the Helm Release while the release was being modified).




------

While upstream Helm's **"one-way"** model of performing installs / upgrade / rollbacks on-demand is great for manual deployments and Continous Deployment solutions, Helm's current model currently lacks the ability to efficiently synchronize deployed resources on seeing post-action **resource drift** or **shifting cluster definitons** (e.g. Kubernetes upgrades, modified `lookup` resources, etc.) that alter what Helm intends to deploy on the cluster.

Therefore, in order to support this **two-way** model of reconciling deployed Helm charts, the Helm Apply re-implements the Helm install/upgrade process via a set of Kubernetes controllers that watch for changes to:
- The cluster state: underlying `kubeVersion`, resources targeted by a Helm `lookup`, and other cluster state qualities that affect the desired state of resources in the cluster
- The desired state: a `HelmChart` resource, which encodes the files that compose a Helm chart (`crds/`, `templates/`, `Chart.yaml`, etc.) and is the **source of truth** for what needs to be deployed onto the cluster
- The current state: the Helm resources deployed to allow the cluster to meet the desired state; if there are any changes to these resources, it is expected that those changes are either overridden (default) or rejected (requires a `ValidatingWebhookConfiguration`)


## How is this different from `k3s-io/helm-controller`?

TBD... maybe they might be the same?

## WIP

Helm Chart is the source of truth

Registering controllers for every CRD in the cluster, and a watch on CRD resources to auto-register new typed controllers on seeing new CRDs. Should implement locking to access the informers -> InformerFactory
-> register dynamic controller, create informer factory based on it, and supply that to apply. It takes care of watching new GVKs for us anyways

Register a controller that re-enqueues all Helm charts on changes to the Kubernetes version

On changes to the Helm chart provided, run a helm install but pass in a special read-only Kubernetes client.
- For any `get` calls, figure out if it is a lookup function call or a call to get actual resources in the cluster. If it is a lookup function call, register a controller that watches for a change in that resource
- For any `create` calls, ignore?
- For any `delete` calls, ignore?
- At the end, grab the release object and pass it along to a call to `Apply`

`Apply` takes in the release object and:
- Performs a dry run on the wrangler apply object. If the dry-run results does not result in modified resources, do nothing.
  - Apply should only run hooks IF the manifest has changes. If not, we would keep triggering pre-installs and post-installs again and again for no reason.
- Creates resources for the pre-install hooks
- Applies the object set for the Helm chart
- Creates resoures for the post-install hooks

On cleanup, pre-delete + apply empty object set + post-delete

### WIP 2

The Helm Release Secret is the source of truth. 

Whenever we see a new release secret or a release secret changes:
- we look at release.Info.Status to see if the release is deployed; if it is not deployed, we updateLock the FixedObjectSet
- If the release is deployed, we updateLock the FixedObjectSet, update the FixedObjectSet, and then unlock the updateLock.
- If the release is deployed, we'll also peer inside the grab lookups and kubeVersion requests. If we see something changes amongst those triggers, we'll trigger a Helm upgrade.

FixedObjectSet will keep track of a set of objects that need to be fixed in the cluster. This will be done via wrangler apply + relatedresource.Lock

If a FixedObjectSet is Locked, it will stop trying to reconcile everything

If a FixedObjectSet is Unlocked, it will continously reconcile everything.

## Building

`make`


## Running

`./bin/helm-locker`

## License
Copyright (c) 2020 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
