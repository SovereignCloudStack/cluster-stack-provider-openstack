# Copyright 2019 The Kubernetes Authors.
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

# Build the manager binary
FROM --platform=${BUILDPLATFORM} docker.io/library/golang:1.23.0-bullseye as build
ARG TARGETOS TARGETARCH

COPY . /src/cluster-stack-provider-openstack
WORKDIR /src/cluster-stack-provider-openstack
RUN --mount=type=cache,target=/root/.cache --mount=type=cache,target=/go/pkg \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -mod=vendor -trimpath -ldflags "-s -w ${LDFLAGS} -extldflags '-static'" \
    -o manager cmd/main.go

FROM --platform=${BUILDPLATFORM} docker.io/library/alpine:3.20.3
WORKDIR /
COPY --from=build /src/cluster-stack-provider-openstack/manager .

# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532:65532
ENTRYPOINT ["/manager"]
