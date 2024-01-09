# Develop Cluster Stack Provider OpenStack

Developing our operator is quite straightforward. First, you need to install some basic prerequisites:

- Docker
- Go

Next, configure your environment variables. Once that's done, you can initiate development using the local Kind cluster and the Tilt UI to create a workload cluster that comes pre-configured.

## Setting Tilt up

1. Install Docker and Go. We expect you to run on a Linux OS.
2. Create an `.envrc` file and specify the values you need. See the `.envrc.sample` for details.

## Developing with Tilt

![tilt](./pics/tilt.png "Tilt")

Operator development requires a lot of iteration, and the “build, tag, push, update deployment” workflow can be very tedious. Tilt makes this process much simpler by watching for updates and automatically building and deploying them. To build a kind cluster and to start Tilt, run:

```shell
make tilt-up
```

> To access the Tilt UI please go to: `http://localhost:10351`

You should make sure that everything in the UI looks green. If not, you can trigger the Tilt workflow again. To establish a connection to your OpenStack project, you must supply a secret containing the `clouds.yaml` file.

```bash
kubectl create secret generic <my-secret> --from-file=clouds.yaml=path/to/clouds.yaml -n cluster
```

To transfer the credentials stored in the secret mentioned above to the operator, the user must create an `OpenStackClusterStackReleaseTemplate` object and specify this secret in the `identityRef` field. The `clouds.yaml` file may contain one or more clouds, so the user must specify desired connection to a specific cloud by using the `cloudName` field. Refer to the `examples/cspotemplate.yaml` file for more details. Afterward, apply this template to the local Kind cluster, which was built by the previous `make` command.

```bash
kubectl apply -f <path-to-openstack-clusterstack-release-template>
```

Now, proceed to apply the `ClusterStack` to the local Kind cluster. For more details see `examples/clusterstack.yaml`.

```bash
kubectl apply -f <path-to-openstack-clusterstack>
```

Please be patient and wait for the operator to execute the necessary tasks. In case your `ClusterStack` object encounters no errors and `openstacknodeimagereleases` is ready, you can deploy a workload cluster. This could be done by applying cluster-template. See the example of this template in `examples/cluster.yaml`.

```bash
kubectl apply -f <path-to-cluster-template>
```

In case you want to change some code, you can do so and see that Tilt triggers on save. It will update the container of the operator automatically.

To tear down the workload cluster press the "Delete Workload Cluster" button. After a few minutes, the resources should be deleted.

To tear down the kind cluster, use:

```shell
make delete-bootstrap-cluster
```

If you have any trouble finding the right command, then you can use `make help` to get a list of all available make targets.
