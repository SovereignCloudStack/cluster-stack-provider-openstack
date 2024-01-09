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

You should make sure that everything in the UI looks green. If not, you can trigger the Tilt workflow again. In the case of the `cspotemplate`, this might be necessary, as it cannot be applied right after the startup of the cluster and fails. Tilt unfortunately does not include a waiting period.

In case you want to change some code, you can do so and see that Tilt triggers on save. It will update the container of the operator automatically.

If you want to change something in your ClusterStack or Cluster custom resources, you can have a look at `.cluster`.yaml and `.clusterstack.yaml`, which Tilt uses.

To tear down the workload cluster press the "Delete Workload Cluster" button. After a few minutes, the resources should be deleted.

To tear down the kind cluster, use:

```shell
make delete-bootstrap-cluster
```

If you have any trouble finding the right command, then you can use `make help` to get a list of all available make targets.
