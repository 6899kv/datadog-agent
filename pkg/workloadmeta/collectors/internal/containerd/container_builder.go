// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build containerd

package containerd

import (
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"

	cutil "github.com/DataDog/datadog-agent/pkg/util/containerd"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/workloadmeta"
)

// buildWorkloadMetaContainer generates a workloadmeta.Container from a containerd.Container
func buildWorkloadMetaContainer(namespace string, container containerd.Container, containerdClient cutil.ContainerdItf) (workloadmeta.Container, error) {
	if container == nil {
		return workloadmeta.Container{}, fmt.Errorf("cannot build workloadmeta container from nil containerd container")
	}

	info, err := containerdClient.Info(namespace, container)
	if err != nil {
		return workloadmeta.Container{}, err
	}

	spec, err := containerdClient.Spec(namespace, container)
	if err != nil {
		return workloadmeta.Container{}, err
	}

	envs, err := cutil.EnvVarsFromSpec(spec)
	if err != nil {
		return workloadmeta.Container{}, err
	}

	image, err := workloadmeta.NewContainerImage(info.Image)
	if err != nil {
		log.Debugf("cannot split image name %q: %s", info.Image, err)
	}

	status, err := containerdClient.Status(namespace, container)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return workloadmeta.Container{}, err
		}

		// The container exists, but there isn't a task associated to it. That
		// means that the container is not running, which is all we need to know
		// in this function (we can set any status != containerd.Running).
		status = containerd.Unknown
	}

	networkIPs := make(map[string]string)
	ip, err := extractIP(namespace, container, containerdClient)
	if err != nil {
		log.Debugf("cannot get IP of container %s", err)
	} else if ip == "" {
		log.Debugf("no IPs for container")
	} else {
		networkIPs[""] = ip
	}

	// Some attributes in workloadmeta.Container cannot be fetched from
	// containerd. I've marked those as "Not available".
	return workloadmeta.Container{
		EntityID: workloadmeta.EntityID{
			Kind: workloadmeta.KindContainer,
			ID:   container.ID(),
		},
		EntityMeta: workloadmeta.EntityMeta{
			Name:   "", // Not available
			Labels: info.Labels,
		},
		Image:   image,
		EnvVars: envs,
		Ports:   nil, // Not available
		Runtime: workloadmeta.ContainerRuntimeContainerd,
		State: workloadmeta.ContainerState{
			Running:    status == containerd.Running,
			Status:     extractStatus(status),
			CreatedAt:  info.CreatedAt,
			StartedAt:  info.CreatedAt, // StartedAt not available in containerd, mapped to CreatedAt
			FinishedAt: time.Time{},    // Not available
		},
		NetworkIPs: networkIPs,
		Hostname:   spec.Hostname,
		PID:        0, // Not available
	}, nil
}

func extractStatus(status containerd.ProcessStatus) workloadmeta.ContainerStatus {
	switch status {
	case containerd.Paused, containerd.Pausing:
		return workloadmeta.ContainerStatusPaused
	case containerd.Created:
		return workloadmeta.ContainerStatusCreated
	case containerd.Running:
		return workloadmeta.ContainerStatusRunning
	case containerd.Stopped:
		return workloadmeta.ContainerStatusStopped
	}

	return workloadmeta.ContainerStatusUnknown
}
