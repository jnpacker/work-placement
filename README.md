# work-placement
This controller uses the [ManifetWorkSpec](https://github.com/open-cluster-management-io/work) and [PlacementDecision](https://github.com/open-cluster-management-io/placement) CR's to create ManifestWork resources in specific clusters based on the `status.decisions`

# Solves
This solves creating a single resource, that applies work to multiple clusters.

# Security
The controller watches for PlaceManifestWork custom resources.  When found, it reads the PlacementRef. If the placementRef has a valid PlacementDecision, a ManifestWork will be created in each cluster listed in the PlacementDecision.

This relies on the ManagedClusterSet to restrict the PlacementDecision status to clusters in the ManagedClusterSet.  It also requires that granting user role permission to PlacementDecision should be limited to read-only.

The PlaceManfiestWork.PlacementRef is namespace scoped, so you can not use a PlacementDecision that is not in the given namespace. This also controls access, a user can only reference Placement's that are in a given namespace where they have permission to create the PlaceManifestWork. Since for Placement to create a PlacementDecision, it must be bound to the ManagedClusterSet (either in its definition or via a namespace binding) the user will not be able to target clusters, that their Placement resource does not have permission to reach.

# How it works
The CRD is simple and draws from the [ManifestWorkSpec](https://github.com/open-cluster-management-io/api/blob/f4d9773affaf693a4a94a7e4caca134c40dde3bd/work/v1/types.go#L30). It's Spec contains a reference to the ManifetWorkSpec, and a object reference to put the Placement resource for cluster targeting.

It updates conditions on the status of whether it has found the PlacementDecision to use for distribution of the ManifestWorkSpec.

The controller will reconcile the PlaceManifestWork against all the created ManifestWork each time it reconciles. If no change occurs to the PlaceManifestWork resource, the reconcile happens every 120s by default.  When reconciling, first the PlacementDecisions status is reconciled. The ManifestWorkSpec is updated or added to all clusters in the PlacementDecision (will loop through multi-page decisions). Once all decided clusters have had their ManifestWork updated or created, it will delete any ManifestWork that are no longer part of the PlacementDecision status.

# Laptop usage
## Build it locally
```shell
make build
```

## Apply CRD
```shell
make install
```

## Run it locally in your dev environment
```shell
make run
```

## Remove CRD
```shell
make uninstall
```

# Kube controller deployment
## Build image
[Contributing...](./CONTRIBUTING.md)
```shell
export REPO=registry.com/repo-name/ # It is important to include the trailing slash
make docker-build
make docker-push
```

## Deploy the controller
```shell
make deploy
```

## Undeploy the controller
```shell
make undeploy
```

# Sample PlaceManifestWork
This sample assumes you have [Open-Cluster-Management.io setup](https://open-cluster-management.io/getting-started/)

* You will need at least 1 `ManagedCluster` to be able to leverage the PlaceManifestWork
* Create the sample `all-clusters` clusterSet
  ```shell
  # Creates a clusterSet named all-clusters and binds that to the default namespace
  kubectl -n default apply -f samples/clusterset.yaml

  # Creates a placement rule, in the default namespace targeting the all-clusters clusterSet
  kubectl -n default apply -f samples/placement.yaml

  # Creates the PlaceManifest work in the default namespace
  kubectl -n default apply -f samples/place-manifestwork.yaml
  ```



# Enhnacements
* See the [Issues tab](https://github.com/jnpacker/work-placement/issues/)

