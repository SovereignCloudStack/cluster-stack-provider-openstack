# -*- mode: Python -*-
load("ext://uibutton", "cmd_button", "location", 'text_input')
load("ext://restart_process", "docker_build_with_restart")

kustomize_cmd = "./hack/tools/bin/kustomize"
envsubst_cmd = "./hack/tools/bin/envsubst"
sed_cmd = "sed 's/:=\"\"//g'"
tools_bin = "./hack/tools/bin"

#Add tools to path
os.putenv("PATH", os.getenv("PATH") + ":" + tools_bin)

update_settings(k8s_upsert_timeout_secs = 60)  # on first tilt up, often can take longer than 30 seconds


settings = {
    "allowed_contexts": [
        "kind-cspo",
    ],
    "local_mode": False,
    "deploy_cert_manager": True,
    "preload_images_for_kind": True,
    "kind_cluster_name": "cspo",
    "capi_version": "v1.6.0",
    "cso_version": "v0.1.0-alpha.7",
    "capo_version": "v0.10.4",
    "cert_manager_version": "v1.13.2",
    "kustomize_substitutions": {
    },
}

# global settings
settings.update(read_yaml(
    "tilt-settings.yaml",
    default = {},
))

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if "allowed_contexts" in settings:
    allow_k8s_contexts(settings.get("allowed_contexts"))

if "default_registry" in settings:
    default_registry(settings.get("default_registry"))

# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(capi_uri, envsubst_cmd)
    local(cmd, quiet = True)
    if settings.get("extra_args"):
        extra_args = settings.get("extra_args")
        if extra_args.get("core"):
            core_extra_args = extra_args.get("core")
            if core_extra_args:
                for namespace in ["capi-system", "capi-webhook-system"]:
                    patch_args_with_extra_args(namespace, "capi-controller-manager", core_extra_args)
        if extra_args.get("kubeadm-bootstrap"):
            kb_extra_args = extra_args.get("kubeadm-bootstrap")
            if kb_extra_args:
                patch_args_with_extra_args("capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", kb_extra_args)

tilt_dockerfile_header_cso = """
FROM ghcr.io/sovereigncloudstack/cso:{} as builder

FROM docker.io/library/alpine:3.18.0 as tilt
WORKDIR /
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /manager /manager
COPY .release/ /tmp/downloads/cluster-stacks/
COPY local_cso.yaml /local_cso.yaml
""".format(settings.get("cso_version"))

def deploy_cso():
    version = settings.get("cso_version")
    cso_uri = "https://github.com/SovereignCloudStack/cluster-stack-operator/releases/download/{}/cso-infrastructure-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(cso_uri, envsubst_cmd)
    local(cmd, quiet = True)

def deploy_local_cso():
    yaml_cso = './local_cso.yaml'

    entrypoint = ["/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    docker_build_with_restart(
        ref = "ghcr.io/sovereigncloudstack/cso-test",
        context = ".",
        dockerfile_contents = tilt_dockerfile_header_cso,
        target = "tilt",
        entrypoint = entrypoint,
        live_update = [
            sync("./local_cso.yaml", "/local_cso.yaml"), # reload when we change the manifest
            sync("./.release", "/tmp/downloads/cluster-stacks"),
        ],
    )
    k8s_yaml(yaml_cso)
    k8s_resource(workload = "cso-controller-manager", labels = ["CSO"])
    k8s_resource(
        objects = [
            "cso-system:namespace",
            "clusteraddons.clusterstack.x-k8s.io:customresourcedefinition",
            "clusterstackreleases.clusterstack.x-k8s.io:customresourcedefinition",
            "clusterstacks.clusterstack.x-k8s.io:customresourcedefinition",
            "cso-controller-manager:serviceaccount",
            "cso-leader-election-role:role",
            "cso-manager-role:clusterrole",
            "cso-leader-election-rolebinding:rolebinding",
            "cso-manager-rolebinding:clusterrolebinding",
            "cso-serving-cert:certificate",
            "cso-selfsigned-issuer:issuer",
        ],
        new_name = "cso-misc",
        labels = ["CSO"],
    )

def deploy_capo():
    version = settings.get("capo_version")
    capo_uri = "https://github.com/kubernetes-sigs/cluster-api-provider-openstack/releases/download/{}/infrastructure-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(capo_uri, envsubst_cmd)
    local(cmd, quiet = True)

def prepare_environment():
    local("kubectl create namespace cluster --dry-run=client -o yaml | kubectl apply -f -")

    # if it's already present then don't copy
    if not os.path.exists('.clusterstack.yaml'):
        local("cp config/cspo/clusterstack.yaml .clusterstack.yaml")

    if not os.path.exists('.secret.yaml'):
        local("cp config/cspo/secret.yaml .secret.yaml")

    if not os.path.exists('.cluster.yaml'):
        local("cp config/cspo/cluster.yaml .cluster.yaml")

def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local("kubectl get deployments {} -n {} -o jsonpath='{{.spec.template.spec.containers[0].args}}'".format(name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/0/args",
            "value": args,
        }]
        local("kubectl patch deployment {} -n {} --type json -p='{}'".format(name, namespace, str(encode_json(patch)).replace("\n", "")))

# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)

def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, args):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("name"):
                    container.get("args").extend(args)

def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")

## This should have the same versions as the Dockerfile
if settings.get("local_mode"):
    tilt_dockerfile_header_cspo = """
    FROM docker.io/library/alpine:3.18.0 as tilt
    WORKDIR /
    COPY .tiltbuild/manager .
    COPY .release/ /tmp/downloads/cluster-stacks/
    """
else:
    tilt_dockerfile_header_cspo = """
    FROM docker.io/library/alpine:3.18.0 as tilt
    WORKDIR /
    COPY manager .
    """


# Build cspo and add feature gates
def deploy_cspo():
    # yaml = str(kustomizesub("./hack/observability")) # build an observable kind deployment by default
    if settings.get("local_mode"):
        yaml = str(kustomizesub("./config/localmode"))
    else:
        yaml = str(kustomizesub("./config/default"))
    local_resource(
        name = "cspo-components",
        cmd = ["sh", "-ec", sed_cmd, yaml, "|", envsubst_cmd],
        labels = ["cspo"],
    )

    # Forge the build command
    # ldflags = "-extldflags \"-static\" " + str(local("hack/version.sh")).rstrip("\n")
    build_env = "CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
    build_cmd = "{build_env} go build -o .tiltbuild/manager cmd/main.go".format(
        build_env = build_env,
        # ldflags = ldflags,
    )
    # Set up a local_resource build of the provider's manager binary.
    local_resource(
        "cspo-manager",
        cmd = "mkdir -p .tiltbuild; " + build_cmd,
        deps = ["api", "cmd", "config", "internal", "vendor", "pkg", "go.mod", "go.sum"],
        labels = ["cspo"],
    )

    entrypoint = ["/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    if settings.get("local_mode"):
        docker_build_with_restart(
            ref = "ghcr.io/sovereigncloudstack/cspo-staging",
            context = ".",
            dockerfile_contents = tilt_dockerfile_header_cspo,
            target = "tilt",
            entrypoint = entrypoint,
            live_update = [
                sync(".tiltbuild/manager", "/manager"),
                sync(".release", "/tmp/downloads/cluster-stacks"),
            ],
            ignore = ["templates"],
        )
    else:
        docker_build_with_restart(
            ref = "ghcr.io/sovereigncloudstack/cspo-staging",
            context = "./.tiltbuild/",
            dockerfile_contents = tilt_dockerfile_header_cspo,
            target = "tilt",
            entrypoint = entrypoint,
            live_update = [
                sync(".tiltbuild/manager", "/manager"),
            ],
            ignore = ["templates"],
        )
    k8s_yaml(blob(yaml))
    k8s_resource(workload = "cspo-controller-manager", labels = ["cspo"])
    k8s_resource(
        objects = [
            "cspo-system:namespace",
            "cspo-controller-manager:serviceaccount",
            "cspo-leader-election-role:role",
            "cspo-manager-role:clusterrole",
            "cspo-leader-election-rolebinding:rolebinding",
            "cspo-manager-rolebinding:clusterrolebinding",
            "cspo-cluster-stack-variables:secret",
            "openstackclusterstackreleases.infrastructure.clusterstack.x-k8s.io:customresourcedefinition",
            "openstackclusterstackreleasetemplates.infrastructure.clusterstack.x-k8s.io:customresourcedefinition",
            "openstacknodeimagereleases.infrastructure.clusterstack.x-k8s.io:customresourcedefinition",
            # "cspo-serving-cert:certificate",
            # "cspo-selfsigned-issuer:issuer", # uncomment when you add webhook code.
        ],
        new_name = "cspo-misc",
        labels = ["cspo"],
    )

def create_secret():
    cmd = "cat .secret.yaml | {} | kubectl apply -f -".format(envsubst_cmd)
    local_resource('supersecret', cmd, labels=["clouds-yaml-secret"])

def base64_encode(to_encode):
    encode_blob = local("echo '{}' | tr -d '\n' | base64 - | tr -d '\n'".format(to_encode), quiet = True)
    return str(encode_blob)

def base64_encode_file(path_to_encode):
    encode_blob = local("cat {} | tr -d '\n' | base64 - | tr -d '\n'".format(path_to_encode), quiet = True)
    return str(encode_blob)

def read_file_from_path(path_to_read):
    str_blob = local("cat {} | tr -d '\n'".format(path_to_read), quiet = True)
    return str(str_blob)

def base64_decode(to_decode):
    decode_blob = local("echo '{}' | base64 --decode -".format(to_decode), quiet = True)
    return str(decode_blob)

def ensure_envsubst():
    if not os.path.exists(envsubst_cmd):
        local("make {}".format(os.path.abspath(envsubst_cmd)))

def ensure_kustomize():
    if not os.path.exists(kustomize_cmd):
        local("make {}".format(os.path.abspath(kustomize_cmd)))

def kustomizesub(folder):
    yaml = local("hack/kustomize-sub.sh {}".format(folder), quiet = True)
    return yaml

def waitforsystem():
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-bootstrap-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-control-plane-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capo-system")

def deploy_observability():
    k8s_yaml(blob(str(local("{} build {}".format(kustomize_cmd, "./hack/observability/"), quiet = True))))

    k8s_resource(workload = "promtail", extra_pod_selectors = [{"app": "promtail"}], labels = ["observability"])
    k8s_resource(workload = "loki", extra_pod_selectors = [{"app": "loki"}], labels = ["observability"])
    k8s_resource(workload = "grafana", port_forwards = "3000", extra_pod_selectors = [{"app": "grafana"}], labels = ["observability"])

##############################
# Actual work happens here
##############################
ensure_envsubst()
ensure_kustomize()

include_user_tilt_files()

load("ext://cert_manager", "deploy_cert_manager")

if settings.get("deploy_cert_manager"):
    deploy_cert_manager(version=settings.get("cert_manager_version"))

if settings.get("deploy_observability"):
    deploy_observability()

deploy_capi()

if settings.get("local_mode"):
    deploy_local_cso()
else:
    deploy_cso()

deploy_cspo()

deploy_capo()

waitforsystem()

prepare_environment()

create_secret()

cmd_button(
    "apply workload cluster",
    argv=["make", "apply-workload-cluster-openstack"],
    location=location.NAV,
    icon_name="add_circle",
)

cmd_button(
    "delete workload cluster",
    argv=["make", "delete-workload-cluster-openstack"],
    location=location.NAV,
    icon_name="cancel",
)

cmd_button(
    "apply clusterstack",
    argv=["make", "apply-clusterstack"],
    location=location.NAV,
    icon_name="bolt",
)

cmd_button(
    "delete clusterstack",
    argv=["make", "delete-clusterstack"],
    location=location.NAV,
    icon_name="delete",
)

