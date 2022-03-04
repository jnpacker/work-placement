# work-placement
This controller uses the [ManifetWorkSpec](https://github.com/open-cluster-management-io/work) and [PlacementDecision](https://github.com/open-cluster-management-io/placement) CR's to create ManifestWork resources in specific clusters based on the `status.decisions`

# Solves
Use a single resource for creating ManifestWork, across multiple clusters.

# Security
The controller watches for PlaceManifestWork custom resources.  When found, it reads the `Spec.PlacementRef`. If the placementRef has a valid `PlacementDecision` resource available, a ManifestWork will be created in each cluster listed in the `PlacementDecision.Status.Decisions`.

This relies on `ManagedClusterSet` to restrict the `PlacementDecision` to only authorized clusters.  Since normal users do not have create, update or patch for `PlacementDecision`, they can not effect the decision outcome.

The `PlaceManfiestWork.Spec.PlacementRef` is namespace scoped, so you can not use a `PlacementDecision` that is not in the same namespace. This also controls access, as a user can only reference Placement's that are in the same namespace where they have permission to create the `PlaceManifestWork` resource. 

For `Placement` to create a `PlacementDecision`, it must be bound to a `ManagedClusterSet` via a namespace binding. The user will not be able to target clusters that their `Placement` resource does not have permission to access.

# How it works
The CRD is simple and draws from the [ManifestWorkSpec](https://github.com/open-cluster-management-io/api/blob/f4d9773affaf693a4a94a7e4caca134c40dde3bd/work/v1/types.go#L30). It's Spec contains a reference to the ManifetWorkSpec, and an object reference to put a Placement resource for cluster targeting. The `Placement` resource name is referenced as a label on its `PlacementDecision` which is how the controller is able to look up the `PlacementDecision` resource.

The controller updates the conditions for the `PlaceManifestWork` tracking whether it has found the `PlacementDecision` to use for distribution of the ManifestWorkSpec and how successful it was creating or updating `ManifestWorks`

The controller will reconcile the `PlaceManifestWork` against all the existing `ManifestWork` each time it reconciles. If no change occured to the `PlaceManifestWork` resource, the reconcile happens every 120s.  When reconciling, first the `PlacementDecisions.status.decisions` is reconciled. The ManifestWorkSpec is updated or added to all clusters in the PlacementDecision (will loop through multi-page decisions). Once all chosen clusters have had their ManifestWork updated or created, it will delete any ManifestWork that are no longer part of the PlacementDecision status.

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
* Creating a ManagedClusterSetBinding
  ```shell
  kubectl apply -f samples/clustersetbinding.yaml
  ```
* Creates a placement resource in the default namespace targeting the all-clusters clusterSet
    ```shell
    kubectl -n default apply -f samples/placement.yaml
    ```

* Creates the PlaceManifest work in the default namespace
  ```shell
  kubectl -n default apply -f samples/place-manifestwork.yaml
  ```



# Enhnacements
* See the [Issues tab](https://github.com/jnpacker/work-placement/issues/)

