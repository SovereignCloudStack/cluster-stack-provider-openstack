# Copyright 2024 The Kubernetes Authors.
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

# This is the Dockerfile for the Builder Image that is used by the CI
# pipelines.
# If you make changes to this Dockerfile run `make builder-image-push`.

# Install Lychee
FROM docker.io/lycheeverse/lychee:0.15.1 as lychee
# Install Hadolint
FROM docker.io/hadolint/hadolint:v2.12.0-alpine@sha256:7dba9a9f1a0350f6d021fb2f6f88900998a4fb0aaf8e4330aa8c38544f04db42 as hadolint

FROM cgr.dev/chainguard/wolfi-base:latest as wolfi

# update: datasource=github-tags depName=kubernetes-sigs/cluster-api
ENV CLUSTERCTL_VERSION="v1.7.3"
# update: datasource=github-tags depName=golangci/golangci-lint
ENV GOLANGCI_VERSION="v1.59.1"
# update: datasource=github-tags depName=kubernetes-sigs/kind
ENV KIND_VERSION="v0.23.0"
# update: datasource=github-tags depName=kubernetes/kubernetes
ARG KUBECTL_VERSION="v1.27.3"
# update: datasource=github-tags depName=kubernetes-sigs/kustomize extractVersion=^kustomize\/v(?<version>.+)$
ENV KUSTOMIZE_VERSION="v5.3.0"
# update: datasource=github-tags depName=aquasecurity/trivy
ENV TRIVY_VERSION="v0.52.2"
# update: datasource=github-tags depName=kubernetes-sigs/controller-tools
ENV CONTROLLER_GEN_VERSION="v0.15.0"

# hadolint ignore=DL3018
RUN apk add -U --no-cache \
    curl \
    clusterctl=~${CLUSTERCTL_VERSION#v} \
    controller-gen=~${CONTROLLER_GEN_VERSION#v} \
    kind=~${KIND_VERSION#v} \
    kubectl=~${KUBECTL_VERSION#v} \
    kustomize=~${KUSTOMIZE_VERSION#v} \
    trivy=~${TRIVY_VERSION#v} 

WORKDIR /
RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s ${GOLANGCI_VERSION}

############################
# CSPO Build Image Base #
############################
FROM docker.io/library/golang:1.21.8-bullseye

# update: datasource=github-tags depName=adrienverge/yamllint versioning=semver
ENV YAMLLINT_VERSION="v1.35.1"

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install -qy --no-install-recommends \
    gnupg python3 python3-pip \
    file zip unzip jq gettext \
    protobuf-compiler libprotobuf-dev \
    libsystemd-dev jq && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    pip install --no-cache-dir \
    yamllint==${YAMLLINT_VERSION}

COPY --from=wolfi /usr/bin/clusterctl /usr/bin/clusterctl
COPY --from=wolfi /usr/bin/controller-gen /usr/bin/controller-gen
COPY --from=wolfi /bin/golangci-lint /usr/bin/golangci-lint
COPY --from=wolfi /usr/bin/kubectl /usr/bin/kubectl
COPY --from=wolfi /usr/bin/kind /usr/bin/kind
COPY --from=wolfi /usr/bin/kustomize /usr/bin/kustomize
COPY --from=wolfi /usr/bin/trivy /usr/bin/trivy
COPY --from=lychee /usr/local/bin/lychee /usr/bin/lychee
COPY --from=hadolint /bin/hadolint /usr/bin/hadolint

ENV GOCACHE=/go/cache

COPY build.sh /
RUN chmod +x /build.sh
ENTRYPOINT ["/build.sh"]
