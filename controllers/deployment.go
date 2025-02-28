/*
Copyright 2022.

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

package controllers

import (
	"fmt"

	buildv1alpha1 "github.com/kairos-io/osbuilder/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func genDeploymentLabel(s string) map[string]string {
	return map[string]string{
		"osbuild": "workload" + s,
	}
}

// TODO: Handle registry auth
// TODO: This shells out, but needs ENV_VAR with key refs mapping
func unpackContainer(id, containerImage, pullImage string, pullOptions buildv1alpha1.Pull) v1.Container {
	return v1.Container{
		ImagePullPolicy: v1.PullAlways,
		Name:            fmt.Sprintf("pull-image-%s", id),
		Image:           containerImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Args: []string{
			fmt.Sprintf(
				"luet util unpack %s %s",
				pullImage,
				"/rootfs",
			),
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "rootfs",
				MountPath: "/rootfs",
			},
		},
	}
}

func createImageContainer(containerImage string, pushOptions buildv1alpha1.Push) v1.Container {
	return v1.Container{
		ImagePullPolicy: v1.PullAlways,
		Name:            "create-image",
		Image:           containerImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Args: []string{
			fmt.Sprintf(
				"tar -czvpf test.tar -C /rootfs . && luet util pack %s test.tar image.tar && mv image.tar /public",
				pushOptions.ImageName,
			),
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "rootfs",
				MountPath: "/rootfs",
			},
			{
				Name:      "public",
				MountPath: "/public",
			},
		},
	}
}

func osReleaseContainer(containerImage string) v1.Container {
	return v1.Container{
		ImagePullPolicy: v1.PullAlways,
		Name:            "os-release",
		Image:           containerImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Args: []string{
			"cp -rfv /etc/os-release /rootfs/etc/os-release",
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "config",
				MountPath: "/etc/os-release",
				SubPath:   "os-release",
			},
			{
				Name:      "rootfs",
				MountPath: "/rootfs",
			},
		},
	}
}

func (r *OSArtifactReconciler) genDeployment(artifact buildv1alpha1.OSArtifact, svc *v1.Service) *appsv1.Deployment {
	// TODO: svc is unused, but could be used in the future to generate the Netboot URL
	objMeta := metav1.ObjectMeta{
		Name:            artifact.Name,
		Namespace:       artifact.Namespace,
		OwnerReferences: genOwner(artifact),
	}

	pushImage := artifact.Spec.PushOptions.Push

	privileged := false
	serviceAccount := false

	cmd := fmt.Sprintf(
		"/entrypoint.sh --debug --name %s build-iso --date=false --output /public dir:/rootfs",
		artifact.Name,
	)

	volumeMounts := []v1.VolumeMount{
		{
			Name:      "public",
			MountPath: "/public",
		},
		{
			Name:      "rootfs",
			MountPath: "/rootfs",
		},
	}

	if artifact.Spec.GRUBConfig != "" {
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      "config",
			MountPath: "/iso/iso-overlay/boot/grub2/grub.cfg",
			SubPath:   "grub.cfg",
		})
	}

	cloudImgCmd := fmt.Sprintf(
		"/raw-images.sh /rootfs /public/%s.raw",
		artifact.Name,
	)

	if artifact.Spec.CloudConfig != "" {
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      "config",
			MountPath: "/iso/iso-overlay/cloud_config.yaml",
			SubPath:   "config",
		})

		cloudImgCmd += " /iso/iso-overlay/cloud_config.yaml"
	}

	if artifact.Spec.CloudConfig != "" || artifact.Spec.GRUBConfig != "" {
		cmd = fmt.Sprintf(
			"/entrypoint.sh --debug --name %s build-iso --date=false --overlay-iso /iso/iso-overlay --output /public dir:/rootfs",
			artifact.Name,
		)
	}

	buildIsoContainer := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "build-iso",
		Image:           r.ToolImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Args: []string{
			cmd,
		},
		VolumeMounts: volumeMounts,
	}

	buildCloudImageContainer := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "build-cloud-image",
		Image:           r.ToolImage,

		Command: []string{"/bin/bash", "-cxe"},
		Args: []string{
			cloudImgCmd,
		},
		VolumeMounts: volumeMounts,
	}

	if artifact.Spec.DiskSize != "" {
		buildCloudImageContainer.Env = []v1.EnvVar{{
			Name:  "EXTEND",
			Value: artifact.Spec.DiskSize,
		}}
	}

	extractNetboot := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "build-netboot",
		Image:           r.ToolImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Env: []v1.EnvVar{{
			Name:  "URL",
			Value: artifact.Spec.NetbootURL,
		}},
		Args: []string{
			fmt.Sprintf(
				"/netboot.sh /public/%s.iso /public/%s",
				artifact.Name,
				artifact.Name,
			),
		},
		VolumeMounts: volumeMounts,
	}

	buildAzureCloudImageContainer := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "build-azure-cloud-image",
		Image:           r.ToolImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Args: []string{
			fmt.Sprintf(
				"/azure.sh /public/%s.raw /public/%s.vhd",
				artifact.Name,
				artifact.Name,
			),
		},
		VolumeMounts: volumeMounts,
	}

	buildGCECloudImageContainer := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "build-gce-cloud-image",
		Image:           r.ToolImage,
		Command:         []string{"/bin/bash", "-cxe"},
		Args: []string{
			fmt.Sprintf(
				"/gce.sh /public/%s.raw /public/%s.gce.raw",
				artifact.Name,
				artifact.Name,
			),
		},
		VolumeMounts: volumeMounts,
	}

	servingContainer := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "serve",
		Ports:           []v1.ContainerPort{v1.ContainerPort{Name: "http", ContainerPort: 80}},
		Image:           r.ServingImage,
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "public",
				MountPath: "/usr/share/nginx/html",
			},
		},
	}

	pod := v1.PodSpec{
		AutomountServiceAccountToken: &serviceAccount,
		Volumes: []v1.Volume{
			{
				Name:         "public",
				VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
			},
			{
				Name:         "rootfs",
				VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
			},
			{
				Name: "config",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{Name: artifact.Name}}},
			},
		},
	}

	pod.InitContainers = []v1.Container{unpackContainer("baseimage", r.ToolImage, artifact.Spec.ImageName, artifact.Spec.PullOptions)}

	for i, bundle := range artifact.Spec.Bundles {
		pod.InitContainers = append(pod.InitContainers, unpackContainer(fmt.Sprint(i), r.ToolImage, bundle, artifact.Spec.PullOptions))
	}

	if artifact.Spec.OSRelease != "" {
		pod.InitContainers = append(pod.InitContainers, osReleaseContainer(r.ToolImage))

	}

	if artifact.Spec.ISO || artifact.Spec.Netboot {
		pod.InitContainers = append(pod.InitContainers, buildIsoContainer)
	}

	if artifact.Spec.Netboot {
		pod.InitContainers = append(pod.InitContainers, extractNetboot)
	}

	if artifact.Spec.CloudImage || artifact.Spec.AzureImage || artifact.Spec.GCEImage {
		pod.InitContainers = append(pod.InitContainers, buildCloudImageContainer)
	}

	if artifact.Spec.AzureImage {
		pod.InitContainers = append(pod.InitContainers, buildAzureCloudImageContainer)
	}

	if artifact.Spec.GCEImage {
		pod.InitContainers = append(pod.InitContainers, buildGCECloudImageContainer)
	}

	if pushImage {
		pod.InitContainers = append(pod.InitContainers, createImageContainer(r.ToolImage, artifact.Spec.PushOptions))
	}

	pod.Containers = []v1.Container{servingContainer}

	deploymentLabels := genDeploymentLabel(artifact.Name)
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: objMeta,

		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: deploymentLabels},
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deploymentLabels,
				},
				Spec: pod,
			},
		},
	}
}
