# Quickstart

This section guides you through all the necessary steps to create a workload Kubernetes cluster on top of the OpenStack infrastructure. The guide describes a path that utilizes the [clusterctl] CLI tool to manage the lifecycle of a [CAPI] management cluster and employs [kind] to create a local non-production management cluster.

Note that it is a common practice to create a temporary, local [bootstrap cluster](https://cluster-api.sigs.k8s.io/reference/glossary#bootstrap-cluster) which is then used to provision a target [management cluster](https://cluster-api.sigs.k8s.io/reference/glossary#management-cluster) on the selected infrastructure.

## Prerequisites

- Install [Docker] and [kind]
- Install [kubectl]
- Install [clusterctl]
- Install [jq]
- Install [go]  # installation of the Go package `envsubst` is required to enable the expansion of variables specified in CSPO and CSO manifests.

## Initialize the management cluster

Create the kind cluster:

```bash
kind create cluster
```

Transform the Kubernetes cluster into a management cluster by using `clusterctl init` and bootstrap it with CAPI and Cluster API Provider OpenStack ([CAPO]) components:

```bash
# Enable Cluster Class CAPI experimental feature
export CLUSTER_TOPOLOGY=true

# Install CAPI and CAPO components
clusterctl init --infrastructure openstack
```

### Create a secret for OpenStack access

To enable communication between the CSPO and the Cluster API Provider for OpenStack (CAPO) with the OpenStack API, it is necessary to generate a secret containing the access data (clouds.yaml).
Ensure that this secret is located in the identical namespace as the other Custom Resources.

> [!NOTE]  
> The default value of `cloudName` is configured as `openstack`. This setting can be overridden by including the `cloudName` key in the secret. Also, be aware that the name of the secret is expected to be `openstack` unless it is not set differently in OpenStackClusterStackReleaseTemplate in `identityRef.name` field.

```bash
kubectl create secret generic openstack --from-file=clouds.yaml=path/to/clouds.yaml

# Label the created secrets so they are automatically moved to the target cluster later.

kubectl label secret openstack clusterctl.cluster.x-k8s.io/move=
```

### CSO and CSPO variables preparation

The CSO and CSPO must be directed to the Cluster Stacks repository housing releases for the OpenStack provider.
Modify and export the following environment variables if you wish to redirect CSO and CSPO to an alternative Git repository

Be aware that GitHub enforces limitations on the number of API requests per unit of time. To overcome this,
it is recommended to configure a personal access token for authenticated calls. This will significantly increase the rate limit for GitHub API requests.

```bash
export GIT_PROVIDER_B64=Z2l0aHVi  # github
export GIT_ORG_NAME_B64=U292ZXJlaWduQ2xvdWRTdGFjaw==  # SovereignCloudStack
export GIT_REPOSITORY_NAME_B64=Y2x1c3Rlci1zdGFja3M=  # cluster-stacks
export GIT_ACCESS_TOKEN_B64=<my-github-access-token>
```

### CSO and CSPO deployment

Install the [envsubst] Go package. It is required to enable the expansion of variables specified in CSPO and CSO manifests.

```bash
GOBIN=/tmp go install github.com/drone/envsubst/v2/cmd/envsubst@latest
```

Get the latest CSO release version and apply CSO manifests to the management cluster.

```bash
# Get the latest CSO release version
CSO_VERSION=$(curl https://api.github.com/repos/SovereignCloudStack/cluster-stack-operator/releases/latest -s | jq .name -r)
# Apply CSO manifests
curl -sSL https://github.com/SovereignCloudStack/cluster-stack-operator/releases/download/${CSO_VERSION}/cso-infrastructure-components.yaml | /tmp/envsubst | kubectl apply -f -
```

Get the latest CSPO release version and apply CSPO manifests to the management cluster.

```bash
# Get the latest CSPO release version
CSPO_VERSION=$(curl https://api.github.com/repos/SovereignCloudStack/cluster-stack-provider-openstack/releases/latest -s | jq .name -r)
# Apply CSPO manifests
curl -sSL https://github.com/SovereignCloudStack/cluster-stack-provider-openstack/releases/download/${CSPO_VERSION}/cspo-infrastructure-components.yaml | /tmp/envsubst | kubectl apply -f -
```

## Create the workload cluster

To transfer the credentials stored in the mentioned secret [above](#create-a-secret-for-openstack-access) to the operator,
create an `OpenStackClusterStackReleaseTemplate` object.
Refer to the `examples/cspotemplate.yaml` file for more details.

Next, apply this template to the management cluster:

```bash
kubectl apply -f <path-to-openstack-clusterstack-release-template>
```

Proceed to apply the `ClusterStack` to the management cluster. For more details, refer to `examples/clusterstack.yaml`:

```bash
kubectl apply -f <path-to-openstack-clusterstack>
```

Please be patient and wait for the operator to execute the necessary tasks.
If your `ClusterStack` object encounters no errors and reports usable versions, as well as the `openstackclusterstackrelease` and `openstacknodeimagereleases` are ready, you can deploy a workload cluster.
This can be done by applying the cluster-template.
Refer to the example of this template in `examples/cluster.yaml`:

```bash
kubectl apply -f <path-to-cluster-template>
```

Utilize a convenient CLI `clusterctl` to investigate the health of the cluster:

```bash
clusterctl describe cluster <cluster-name>
```

Once the cluster is provisioned and in good health, you can retrieve its kubeconfig and establish communication with the newly created workload cluster:

```bash
# Get the workload cluster kubeconfig
clusterctl get kubeconfig <cluster-name> > kubeconfig.yaml
# Communicate with the workload cluster
kubectl --kubeconfig kubeconfig.yaml get nodes
```

<!-- links -->
[Docker]: https://www.docker.com/
[kind]: https://kind.sigs.k8s.io/
[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[clusterctl]: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl
[CAPO]: https://github.com/kubernetes-sigs/cluster-api-provider-openstack
[CAPI]: https://cluster-api.sigs.k8s.io/
[go]: https://go.dev/doc/install
[envsubst]: https://github.com/drone/envsubst
[jq]: https://jqlang.github.io/jq/download/
