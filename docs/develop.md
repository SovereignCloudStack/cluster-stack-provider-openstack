# Develop Cluster Stack Provider OpenStack

Developing our operator is quite straightforward. First, you need to install some basic prerequisites:

- Docker
- Go

Next, configure your environment variables. Once that's done, you can initiate development using the local Kind cluster and the Tilt UI to create a workload cluster that comes pre-configured.

## Setting Tilt up

1. Install Docker and Go. We expect you to run on a Linux OS.
2. Create an `.envrc` file and specify the values you need. See the `.envrc.sample` for details.

## Developing with Tilt

![tilt](./images/tilt.png "Tilt")

Operator development requires a lot of iteration, and the “build, tag, push, update deployment” workflow can be very tedious. Tilt makes this process much simpler by watching for updates and automatically building and deploying them. To build a kind cluster and to start Tilt, run:

```shell
make tilt-up
```

> To access the Tilt UI please go to: `http://localhost:10351`

You should make sure that everything in the UI looks green. If not, you can trigger the Tilt workflow again.

If everything is green, then you can already check for your clusterstack that has been deployed. You can use a tool like k9s to have a look at the management cluster and its custom resources.

In case your clusterstack shows that it is ready, you can deploy a workload cluster. This could be done through the Tilt UI, by pressing the button in the top right corner `Create Workload Cluster`. This triggers the `make create-workload-cluster-openstack`, which uses the environment variables and the cluster-template.

To interact with your freshly created workload cluster, you can use these commands:

```shell
make get-kubeconfig-workload-cluster #KUBECONFIG for the workload cluster is placed here: ".workload-cluster-kubeconfig.yaml"
export KUBECONFIG=$PWD/.workload-cluster-kubeconfig.yaml
```

In case you want to change some code, you can do so and see that Tilt triggers on save. It will update the container of the operator automatically.

If you want to change something in your ClusterStack or Cluster custom resources, you can have a look at `.cluster.yaml` and `.clusterstack.yaml`, which Tilt uses.

To tear down the workload cluster, click on the `Delete Workload Cluster` button in the top right corner of the Tilt UI. This action triggers the execution of `make delete-workload-cluster-openstack`. After a few minutes, the resources should be successfully deleted.

To tear down the kind cluster, use:

```shell
make delete-bootstrap-cluster
```

If you have any trouble finding the right command, then you can use `make help` to get a list of all available make targets.
