/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterStackReleaseAssetsReadyCondition reports on whether the download of cluster stack release assets is complete.
	ClusterStackReleaseAssetsReadyCondition clusterv1beta1.ConditionType = "ClusterStackReleaseDownloaded"

	// ReleaseAssetsNotDownloadedYetReason is used when release assets are not yet downloaded.
	ReleaseAssetsNotDownloadedYetReason = "ReleaseAssetsNotDownloadedYet"

	// IssueWithReleaseAssetsReason is used when release assets have an issue.
	IssueWithReleaseAssetsReason = "IssueWithReleaseAssets"
)

const (
	// OpenStackNodeImageReleasesReadyCondition reports on whether all relevant OpenStackNodeImageRelease objects are ready.
	OpenStackNodeImageReleasesReadyCondition clusterv1beta1.ConditionType = "OpenStackNodeImageReleasesReady"

	// ProcessOngoingReason is used when the process of the OpenStackNodeImageRelease object is still ongoing.
	ProcessOngoingReason = "ProcessOngoing"
)

const (
	// GitAPIAvailableCondition is used when Git API is available.
	GitAPIAvailableCondition = "GitAPIAvailable"

	// GitTokenOrEnvVariableNotSetReason is used when user don't specify the token or environment variable.
	GitTokenOrEnvVariableNotSetReason = "GitTokenOrEnvVariableNotSet" //#nosec
)

const (
	// CloudAvailableCondition is used when cloud is available.
	CloudAvailableCondition = "CloudAvailable"

	// CloudNotSetReason is used when user don't specify a valid clouds.yaml inside a secret.
	CloudNotSetReason = "CloudNotSet"
)

const (
	// OpenStackImageServiceClientAvailableCondition is used when OpenStack Image Service Client is available.
	OpenStackImageServiceClientAvailableCondition = "OpenStackImageServiceClientAvailable"

	// OpenStackImageServiceClientNotSetReason is used when OpenStack Image Service Client is not available.
	OpenStackImageServiceClientNotSetReason = "OpenStackImageServiceClientNotSet"
)

const (
	// OpenStackImageImportStartCondition reports the image import start.
	OpenStackImageImportStartCondition = "OpenStackImageImportStart"

	// OpenStackImageImportNotStartReason is used when image import does not start yet.
	OpenStackImageImportNotStartReason = "OpenStackImageImportNotStartReason"
)

const (
	// OpenStackImageReadyCondition reports on whether the image of cluster stack release is imported and active.
	OpenStackImageReadyCondition clusterv1beta1.ConditionType = "OpenStackImageActive"

	// OpenStackImageNotCreatedYetReason is used when image is not yet created.
	OpenStackImageNotCreatedYetReason = "OpenStackImageNotCreateYet"

	// OpenStackImageNotImportedYetReason is used when image is not yet imported.
	OpenStackImageNotImportedYetReason = "OpenStackImageNotImportedYet"

	// OpenStackImageImportTimeOutReason is used when image import timeout.
	OpenStackImageImportTimeOutReason = "OpenStackImageImportTimeOutReason"

	// IssueWithOpenStackImageReason is used when image has an issue.
	IssueWithOpenStackImageReason = "IssueWithOpenStackImage"
)
