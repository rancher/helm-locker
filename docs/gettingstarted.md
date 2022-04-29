# Getting Started

## Simple Installation

### In Rancher (via Apps & Marketplace)

1. Navigate to `Apps & Marketplace -> Repositories` in your target downstream cluster and create a Repository that points to a `Git repository containing Helm chart or cluster template definitions` where the `Git Repo URL` is `https://github.com/rancher/helm-locker` and the `Git Branch` is `main`
2. Navigate to `Apps & Marketplace -> Charts`; you should see two charts under the new Repository you created: `Helm Locker` and `Helm Locker Example Chart`. 
3. Install `Helm Locker` first (which will automatically install `helm-locker-crd`)
4. Install `Helm Locker Example Chart`

### In a normal Kubernetes cluster (via running Helm 3 locally)

1. Install `helm-locker-crd` onto your cluster via Helm to install the HelmRelease CRD

```
helm install -n cattle-helm-system helm-locker-crd charts/helm-locker-crd
```

2. Install `helm-locker` onto your cluster via Helm to install the Helm Locker Operator

```
helm install -n cattle-helm-system helm-locker charts/helm-locker
```

3. Install `helm-locker-example` to check out a simple Helm chart containing a ConfigMap and a HelmRelease CR that targets the release itself and keeps it locked into place

```bash
helm install -n cattle-helm-system helm-locker-example charts/helm-locker-example
```

### Checking if the HelmRelease works

1. Ensure that the logs of `helm-locker` in the `cattle-helm-system` namespace show that the controller was able to acquire a lock and has started in that namespace
2. Try to delete or modify the ConfigMaps deployed by the `helm-locker-example` chart (`cattle-helm-system/my-config-map` and `cattle-helm-system/my-config-map-2`); any changes should automatically be overwritten and a log will show up in the Helm Locker logs that showed which ConfigMap it detected a change in
3. Run `kubectl describe helmreleases -n cattle-helm-system helm-locker-example`; you should be able to see events that have been triggered on changes.
4. Upgrade the `helm-locker-example` values to change the contents of the ConfigMap; you should see the modifications show up in the ConfigMap deployed in the cluster as well as events that have been triggered on Helm Locker noticing that change (i.e. you should see a `Transitioning` event that is emitted).