# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

CONTROLLER_SHORT = cspo
CONTROLLER_NAME = cluster-stack-provider-openstack
IMAGE_PREFIX ?= ghcr.io/sovereigncloudstack

STAGING_IMAGE = $(CONTROLLER_SHORT)-staging
BUILDER_IMAGE = $(IMAGE_PREFIX)/$(CONTROLLER_SHORT)-builder
BUILDER_IMAGE_VERSION = $(shell cat .builder-image-version.txt)

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
.DEFAULT_GOAL:=help
GOTEST ?= go test

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

#############
# Variables #
#############

# Certain aspects of the build are done in containers for consistency (e.g. protobuf generation)
# If you have the correct tools installed and you want to speed up development you can run
# make BUILD_IN_CONTAINER=false target
# or you can override this with an environment variable
BUILD_IN_CONTAINER ?= true

# Boiler plate for building Docker containers.
TAG ?= dev
ARCH ?= amd64
# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always
# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

TIMEOUT := $(shell command -v timeout || command -v gtimeout)

# Directories
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := bin
TEST_DIR := test
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
export GOBIN := $(abspath $(TOOLS_BIN_DIR))

# Files
WORKER_CLUSTER_KUBECONFIG ?= ".workload-cluster-kubeconfig.yaml"
MGT_CLUSTER_KUBECONFIG ?= ".mgt-cluster-kubeconfig.yaml"

# Kubebuilder.
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.28.0

##@ Binaries
############
# Binaries #
############
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
controller-gen: $(CONTROLLER_GEN) ## Build a local copy of controller-gen
$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.13.0

KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize
$(KUSTOMIZE): # Build kustomize from tools folder.
	go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.7

TILT := $(abspath $(TOOLS_BIN_DIR)/tilt)
tilt: $(TILT) ## Build a local copy of tilt
$(TILT):
	@mkdir -p $(TOOLS_BIN_DIR)
	MINIMUM_TILT_VERSION=0.33.3 hack/ensure-tilt.sh

ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/envsubst)
envsubst: $(ENVSUBST) ## Build a local copy of envsubst
$(ENVSUBST): # Build envsubst from tools folder.
	go install github.com/drone/envsubst/v2/cmd/envsubst@latest

SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/setup-envtest)
setup-envtest: $(SETUP_ENVTEST) ## Build a local copy of setup-envtest
$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20231102085812-c30c66d67f47

CTLPTL := $(abspath $(TOOLS_BIN_DIR)/ctlptl)
ctlptl: $(CTLPTL) ## Build a local copy of ctlptl
$(CTLPTL):
	go install github.com/tilt-dev/ctlptl/cmd/ctlptl@v0.8.20

CLUSTERCTL := $(abspath $(TOOLS_BIN_DIR)/clusterctl)
clusterctl: $(CLUSTERCTL) ## Build a local copy of clusterctl
$(CLUSTERCTL):
	curl -sSLf https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.5.3/clusterctl-$$(go env GOOS)-$$(go env GOARCH) -o $(CLUSTERCTL)
	chmod a+rx $(CLUSTERCTL)

KIND := $(abspath $(TOOLS_BIN_DIR)/kind)
kind: $(KIND) ## Build a local copy of kind
$(KIND):
	go install sigs.k8s.io/kind@v0.20.0

KUBECTL := $(abspath $(TOOLS_BIN_DIR)/kubectl)
kubectl: $(KUBECTL) ## Build a local copy of kubectl
$(KUBECTL):
	curl -fsSL "https://dl.k8s.io/release/v1.27.3/bin/$$(go env GOOS)/$$(go env GOARCH)/kubectl" -o $(KUBECTL)
	chmod a+rx $(KUBECTL)

TRIVY := $(abspath $(TOOLS_BIN_DIR)/trivy)
trivy: $(TRIVY) ## Build a local copy of trivy
$(TRIVY):
	curl -sSL https://github.com/aquasecurity/trivy/releases/download/v0.45.1/trivy_0.45.1_Linux-64bit.tar.gz | tar xz -C $(TOOLS_BIN_DIR) trivy
	chmod a+rx $(TRIVY)

go-binsize-treemap := $(abspath $(TOOLS_BIN_DIR)/go-binsize-treemap)
go-binsize-treemap: $(go-binsize-treemap) # Build go-binsize-treemap from tools folder.
$(go-binsize-treemap):
	go install github.com/nikolaydubina/go-binsize-treemap@v0.2.0

go-cover-treemap := $(abspath $(TOOLS_BIN_DIR)/go-cover-treemap)
go-cover-treemap: $(go-cover-treemap) # Build go-cover-treemap from tools folder.
$(go-cover-treemap):
	go install github.com/nikolaydubina/go-cover-treemap@v1.3.0

GOTESTSUM := $(abspath $(TOOLS_BIN_DIR)/gotestsum)
gotestsum: $(GOTESTSUM) # Build gotestsum from tools folder.
$(GOTESTSUM):
	go install gotest.tools/gotestsum@v1.10.0

all-tools: $(GOTESTSUM) $(go-cover-treemap) $(go-binsize-treemap) $(KIND) $(PACKER) $(KUBECTL) $(CLUSTERCTL) $(CTLPTL) $(SETUP_ENVTEST) $(ENVSUBST) $(KUSTOMIZE) $(CONTROLLER_GEN) $(TRIVY)
	echo 'done'

##@ Development

env-vars-for-wl-cluster:
ifeq ($(wildcard tilt-settings.yaml),)
	@./hack/ensure-env-variables.sh GIT_PROVIDER_B64 GIT_ACCESS_TOKEN_B64 GIT_ORG_NAME_B64 GIT_REPOSITORY_NAME_B64 CLUSTER_TOPOLOGY CLUSTER_NAME SECRET_NAME CLOUD_NAME ENCODED_CLOUDS_YAML
else ifeq ($(shell awk '/local_mode:/ {print tolower($$2)}' tilt-settings.yaml),true)
	@./hack/ensure-env-variables.sh CLUSTER_TOPOLOGY CLUSTER_NAME SECRET_NAME CLOUD_NAME ENCODED_CLOUDS_YAML
else
	@./hack/ensure-env-variables.sh GIT_PROVIDER_B64 GIT_ACCESS_TOKEN_B64 GIT_ORG_NAME_B64 GIT_REPOSITORY_NAME_B64 CLUSTER_TOPOLOGY CLUSTER_NAME SECRET_NAME CLOUD_NAME ENCODED_CLOUDS_YAML	
endif

.PHONY: cluster
cluster: $(CTLPTL) $(KUBECTL) ## Creates kind-dev Cluster 
	./hack/kind-dev.sh

.PHONY: delete-bootstrap-cluster
delete-bootstrap-cluster: $(CTLPTL)  ## Deletes Kind-dev Cluster
	$(CTLPTL) delete cluster kind-cspo
	$(CTLPTL) delete registry cspo-registry

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.54.2
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Clean
#########
# Clean #
#########
.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated helper binaries
	rm -rf $(BIN_DIR)
	rm -rf $(TOOLS_BIN_DIR)

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_config_patch.yaml ./*manager_pull_policy.yaml

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

##@ Verify
##########
# Verify #
##########
.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate text exists in each file
	./hack/verify-boilerplate.sh

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	./hack/verify-shellcheck.sh

.PHONY: verify-starlark
verify-starlark: ## Verify Starlark Code
	./hack/verify-starlark.sh

.PHONY: verify-manifests ## Verify Manifests
verify-manifests:
	./hack/verify-manifests.sh

.PHONY: verify-container-images
verify-container-images: $(TRIVY) ## Verify container images
	$(TRIVY) image -q --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(IMAGE_PREFIX)/$(CONTROLLER_SHORT):latest

##@ Format
##########
# Format #
##########
.PHONY: format-golang
format-golang: ## Format the Go codebase and run auto-fixers if supported by the linter.
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v --fix
endif

.PHONY: format-starlark
format-starlark: ## Format the Starlark codebase
	./hack/verify-starlark.sh fix

.PHONY: format-yaml
format-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamlfixer --version
	yamlfixer -c .yamllint.yaml .
endif

##@ Lint
########
# Lint #
########

.PHONY: lint-golang
lint-golang: ## Lint Golang codebase
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v
endif

.PHONY: lint-golang-ci
lint-golang-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v --out-format=github-actions
endif

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml --strict .
endif

.PHONY: lint-yaml-ci
lint-yaml-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml . --format github
endif

DOCKERFILES=$(shell find . -not \( -path ./hack -prune \) -not \( -path ./vendor -prune \) -type f -regex ".*Dockerfile.*"  | tr '\n' ' ')
.PHONY: lint-dockerfile
lint-dockerfile: ## Lint Dockerfiles
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	hadolint --version
	hadolint -t error $(DOCKERFILES)
endif

lint-links: ## Link Checker
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run --rm -t -i \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	lychee --config .lychee.toml ./*.md  ./docs/**/*.md
endif

##@ Releasing
#############
# Releasing #
#############

## latest git tag for the commit, e.g., v0.3.10
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]." | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
RELEASE_DIR ?= out
RELEASE_NOTES_DIR := _releasenotes

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: test-release
test-release:
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/cspo-staging MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_config_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"
	$(MAKE) release-manifests

.PHONY: release-manifests
release-manifests: generate-manifests generate-go-deepcopy $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/cspo-infrastructure-components.yaml
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/cspo MANIFEST_TAG=$(RELEASE_TAG) TARGET_RESOURCE="./config/default/manager_config_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"
	## Build the manifests
	$(MAKE) release-manifests clean-release-git

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

##@ Images
##########
# Images #
##########

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' $(TARGET_RESOURCE)

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(TARGET_RESOURCE)

builder-image-promote-latest:
	./hack/ensure-env-variables.sh USERNAME PASSWORD
	skopeo copy --src-creds=$(USERNAME):$(PASSWORD) --dest-creds=$(USERNAME):$(PASSWORD) \
		docker://$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) \
		docker://$(BUILDER_IMAGE):latest

##@ Generate
############
# Generate #
############

.PHONY: generate-boilerplate
generate-boilerplate: ## Generates missing boilerplates
	./hack/ensure-boilerplate.sh

generate-manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
			paths=./api/... \
			paths=./internal/controller/... \
			crd:crdVersions=v1 \
			rbac:roleName=manager-role \
			output:crd:dir=./config/crd/bases \
			output:webhook:dir=./config/webhook \
			webhook
generate-go-deepcopy: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) \
		object:headerFile="./hack/boilerplate/boilerplate.generatego.txt" \
		paths="./api/..."

# support go modules
generate-modules: ## Generates missing go modules
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-provider-openstack$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	./hack/golang-modules-update.sh
endif

generate-modules-ci: generate-modules
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

##@ Testing
###########
# Testing #
###########

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env --bin-dir $(abspath $(TOOLS_BIN_DIR)) -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

.PHONY: test-integration ## Run integration tests
test-integration: test-integration-github test-integration-openstack
	echo done

.PHONY: test-unit
test-unit: test-unit-openstack ## Run unit tests
	echo done

.PHONY: test-unit-openstack
test-unit-openstack: $(SETUP_ENVTEST) $(GOTESTSUM)
	@mkdir -p $(shell pwd)/.coverage
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor \
	-covermode=atomic -coverprofile=.coverage/cover.out -p=4 ./internal/controller/...

.PHONY: test-integration-github
test-integration-github: $(SETUP_ENVTEST) $(GOTESTSUM)
	@mkdir -p $(shell pwd)/.coverage
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor \
	-covermode=atomic -coverprofile=.coverage/cover.out -p=1  ./internal/test/integration/github/...

.PHONY: test-integration-openstack
test-integration-openstack: $(SETUP_ENVTEST) $(GOTESTSUM)
	@mkdir -p $(shell pwd)/.coverage
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor \
	-covermode=atomic -coverprofile=.coverage/cover.out -p=1  ./internal/test/integration/openstack/...

##@ Main Targets
################
# Main Targets #
################
.PHONY: lint
lint: lint-golang lint-yaml lint-dockerfile lint-links ## Lint Codebase

.PHONY: format
format: format-starlark format-golang format-yaml ## Format Codebase

.PHONY: generate
generate: generate-manifests generate-go-deepcopy generate-boilerplate generate-modules ## Generate Files

ALL_VERIFY_CHECKS = boilerplate shellcheck starlark manifests
.PHONY: verify
verify: generate lint $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Verify all

.PHONY: modules
modules: generate-modules ## Update go.mod & go.sum

.PHONY: boilerplate
boilerplate: generate-boilerplate ## Ensure that your files have a boilerplate header

.PHONY: builder-image-push
builder-image-push: ## Build $(CONTROLLER_SHORT)-builder to a new version. For more information see README.
	BUILDER_IMAGE=$(BUILDER_IMAGE) ./hack/upgrade-builder-image.sh

.PHONY: test
test: test-unit test-integration ## Runs all unit and integration tests.

apply-workload-cluster-openstack: $(ENVSUBST) $(KUBECTL)
	cat .cluster.yaml | $(ENVSUBST) - | $(KUBECTL) apply -f -

delete-workload-cluster-openstack: $(ENVSUBST) $(KUBECTL)
	cat .cluster.yaml | $(ENVSUBST) - | $(KUBECTL) delete -f -

apply-clusterstack: $(ENVSUBST) $(KUBECTL)
	cat .clusterstack.yaml | $(ENVSUBST) - | $(KUBECTL) apply -f -

delete-clusterstack: $(ENVSUBST) $(KUBECTL)
	cat .clusterstack.yaml | $(ENVSUBST) - | $(KUBECTL) delete -f -

get-kubeconfig-workload-cluster:
	./hack/get-kubeconfig-of-workload-cluster.sh

.PHONY: tilt-up
tilt-up: env-vars-for-wl-cluster $(ENVSUBST) $(KUBECTL) $(KUSTOMIZE) $(TILT) cluster  ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	$(TILT) up --port=10351
