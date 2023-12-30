// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build docker

// Package ecs implements the ECS Workloadmeta collector.
package ecs

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/DataDog/datadog-agent/comp/core/workloadmeta"
	"github.com/DataDog/datadog-agent/comp/core/workloadmeta/collectors/util"
	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/errors"
	ecsutil "github.com/DataDog/datadog-agent/pkg/util/ecs"
	ecsmeta "github.com/DataDog/datadog-agent/pkg/util/ecs/metadata"
	v1 "github.com/DataDog/datadog-agent/pkg/util/ecs/metadata/v1"
	v3or4 "github.com/DataDog/datadog-agent/pkg/util/ecs/metadata/v3or4"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/util/retry"

	"go.uber.org/fx"
)

const (
	collectorID   = "ecs"
	componentName = "workloadmeta-ecs"
)

type collector struct {
	id                               string
	store                            workloadmeta.Component
	catalog                          workloadmeta.AgentType
	metaV1                           v1.Client
	metaV3or4                        func(metaURI, metaVersion string) v3or4.Client
	clusterName                      string
	hasResourceTags                  bool
	collectResourceTags              bool
	resourceTags                     map[string]resourceTags
	seen                             map[workloadmeta.EntityID]struct{}
	orchestratorECSCollectionEnabled bool
}

type resourceTags struct {
	tags                  map[string]string
	containerInstanceTags map[string]string
}

// NewCollector returns a new ecs collector provider and an error
func NewCollector() (workloadmeta.CollectorProvider, error) {
	orchestratorECSCollectionEnabled := false
	if config.Datadog.GetBool("orchestrator_explorer.enabled") &&
		config.Datadog.GetBool("orchestrator_explorer.ecs_collection.enabled") {
		orchestratorECSCollectionEnabled = true
	}

	return workloadmeta.CollectorProvider{
		Collector: &collector{
			id:                               collectorID,
			resourceTags:                     make(map[string]resourceTags),
			seen:                             make(map[workloadmeta.EntityID]struct{}),
			catalog:                          workloadmeta.NodeAgent | workloadmeta.ProcessAgent,
			orchestratorECSCollectionEnabled: orchestratorECSCollectionEnabled,
		},
	}, nil
}

// GetFxOptions returns the FX framework options for the collector
func GetFxOptions() fx.Option {
	return fx.Provide(NewCollector)
}

func (c *collector) Start(ctx context.Context, store workloadmeta.Component) error {
	if !config.IsFeaturePresent(config.ECSEC2) {
		return errors.NewDisabled(componentName, "Agent is not running on ECS EC2")
	}

	var err error

	c.store = store
	c.metaV1, err = ecsmeta.V1()
	if err != nil {
		return err
	}

	// This only exists to allow overriding for testing
	c.metaV3or4 = func(metaURI, metaVersion string) v3or4.Client {
		return v3or4.NewClient(metaURI, metaVersion)
	}

	c.hasResourceTags = ecsutil.HasEC2ResourceTags()
	c.collectResourceTags = config.Datadog.GetBool("ecs_collect_resource_tags_ec2")

	instance, err := c.metaV1.GetInstance(ctx)
	if err == nil {
		c.clusterName = instance.Cluster
	} else {
		log.Warnf("cannot determine ECS cluster name: %s", err)
	}

	return nil
}

func (c *collector) Pull(ctx context.Context) error {
	tasks, err := c.metaV1.GetTasks(ctx)
	if err != nil {
		return err
	}

	// we always parse all the tasks coming from the API, as they are not
	// immutable: the list of containers in the task changes as containers
	// don't get added until they actually start running, and killed
	// containers will get re-created.
	c.store.Notify(c.parseTasks(ctx, tasks))

	return nil
}

func (c *collector) GetID() string {
	return c.id
}

func (c *collector) GetTargetCatalog() workloadmeta.AgentType {
	return c.catalog
}

func (c *collector) parseTasks(ctx context.Context, tasks []v1.Task) []workloadmeta.CollectorEvent {
	events := []workloadmeta.CollectorEvent{}
	seen := make(map[workloadmeta.EntityID]struct{})

	for _, task := range tasks {
		// We only want to collect tasks without a STOPPED status.
		if task.KnownStatus == "STOPPED" {
			continue
		}

		// If the orchestrator explorer ecs collection is enabled, collector will query the v4 task endpoint for each task
		// and store task and container in the store.
		if c.orchestratorECSCollectionEnabled {
			events = append(events, c.parseV4Task(ctx, task, seen)...)
			continue
		}

		entityID := workloadmeta.EntityID{
			Kind: workloadmeta.KindECSTask,
			ID:   task.Arn,
		}

		seen[entityID] = struct{}{}

		arnParts := strings.Split(task.Arn, "/")
		taskID := arnParts[len(arnParts)-1]
		taskContainers, containerEvents := c.parseTaskContainers(task, seen)

		entity := &workloadmeta.ECSTask{
			EntityID: entityID,
			EntityMeta: workloadmeta.EntityMeta{
				Name: taskID,
			},
			ClusterName: c.clusterName,
			Family:      task.Family,
			Version:     task.Version,
			LaunchType:  workloadmeta.ECSLaunchTypeEC2,
			Containers:  taskContainers,
		}

		// Only fetch tags if they're both available and used
		if c.hasResourceTags && c.collectResourceTags {
			rt := c.getResourceTags(ctx, entity)
			entity.ContainerInstanceTags = rt.containerInstanceTags
			entity.Tags = rt.tags
		}

		events = append(events, containerEvents...)
		events = append(events, workloadmeta.CollectorEvent{
			Source: workloadmeta.SourceNodeOrchestrator,
			Type:   workloadmeta.EventTypeSet,
			Entity: entity,
		})
	}

	for seenID := range c.seen {
		if _, ok := seen[seenID]; ok {
			continue
		}

		if c.hasResourceTags && seenID.Kind == workloadmeta.KindECSTask {
			delete(c.resourceTags, seenID.ID)
		}

		var entity workloadmeta.Entity
		switch seenID.Kind {
		case workloadmeta.KindECSTask:
			entity = &workloadmeta.ECSTask{EntityID: seenID}
		case workloadmeta.KindContainer:
			entity = &workloadmeta.Container{EntityID: seenID}
		default:
			log.Errorf("cannot handle expired entity of kind %q, skipping", seenID.Kind)
			continue
		}

		events = append(events, workloadmeta.CollectorEvent{
			Type:   workloadmeta.EventTypeUnset,
			Source: workloadmeta.SourceNodeOrchestrator,
			Entity: entity,
		})
	}

	c.seen = seen

	return events
}

func (c *collector) parseTaskContainers(
	task v1.Task,
	seen map[workloadmeta.EntityID]struct{},
) ([]workloadmeta.OrchestratorContainer, []workloadmeta.CollectorEvent) {
	taskContainers := make([]workloadmeta.OrchestratorContainer, 0, len(task.Containers))
	events := make([]workloadmeta.CollectorEvent, 0, len(task.Containers))

	for _, container := range task.Containers {
		containerID := container.DockerID
		taskContainers = append(taskContainers, workloadmeta.OrchestratorContainer{
			ID:   containerID,
			Name: container.Name,
		})
		entityID := workloadmeta.EntityID{
			Kind: workloadmeta.KindContainer,
			ID:   containerID,
		}

		seen[entityID] = struct{}{}

		events = append(events, workloadmeta.CollectorEvent{
			Source: workloadmeta.SourceNodeOrchestrator,
			Type:   workloadmeta.EventTypeSet,
			Entity: &workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: container.DockerName,
				},
			},
		})
	}

	return taskContainers, events
}

func (c *collector) parseV4Task(ctx context.Context, task v1.Task, seen map[workloadmeta.EntityID]struct{}) []workloadmeta.CollectorEvent {
	v4Task := c.getV4TaskWithTags(ctx, task)
	return util.ParseV4Task(v4Task, seen)
}

// getV4TaskWithTags fetches task and tasks from the metadata v4 API
func (c *collector) getV4TaskWithTags(ctx context.Context, task v1.Task) v3or4.Task {
	var metaURI string
	var metaVersion string
	for _, taskContainer := range task.Containers {
		containerID := taskContainer.DockerID
		container, err := c.store.GetContainer(containerID)
		if err != nil {
			log.Tracef("cannot find container %q found in task %s: %s", taskContainer, task.Arn, err)
			continue
		}

		uri, ok := container.EnvVars[v3or4.DefaultMetadataURIv4EnvVariable]
		if ok && uri != "" {
			metaURI = uri
			metaVersion = "v4"
			break
		}
	}

	if metaURI == "" {
		log.Errorf("failed to get client for metadata v3 or v4 API from task %s and the following containers: %v", task.Arn, task.Containers)
		return v1TaskToV4Task(task)
	}

	metaV3orV4 := c.metaV3or4(metaURI, metaVersion)

	var taskWithTagsRetry retry.Retrier
	var taskWithTags *v3or4.Task
	var err error

	_ = taskWithTagsRetry.SetupRetrier(&retry.Config{
		Name: "get-v4-task-with-tags",
		AttemptMethod: func() error {
			taskWithTags, err = metaV3orV4.GetTaskWithTags(ctx)
			return err
		},
		Strategy:   retry.RetryCount,
		RetryCount: 10,
		RetryDelay: 200 * time.Millisecond,
	})

	// retry 10 times with 200ms delay between each attempt
	// which means it will retry for 2 seconds for each task before giving up
	for {
		err = taskWithTagsRetry.TriggerRetry()
		if err == nil || (reflect.ValueOf(err).Kind() == reflect.Ptr && reflect.ValueOf(err).IsNil()) {
			break
		}

		if retry.IsErrPermaFail(err) {
			log.Errorf("failed to get task with tags from metadata %s API: %s", metaVersion, err)
			return v1TaskToV4Task(task)
		}
	}

	return *taskWithTags
}

// getResourceTags fetches task and container instance tags from the ECS API,
// and caches them for the lifetime of the task, to avoid hitting throttling
// limits from tasks being updated on every pull. Tags won't change in the
// store even if they're changed in the resources themselves, but at least that
// matches the old behavior present in the tagger.
func (c *collector) getResourceTags(ctx context.Context, entity *workloadmeta.ECSTask) resourceTags {
	rt, ok := c.resourceTags[entity.ID]
	if ok {
		return rt
	}

	var metaURI string
	var metaVersion string
	for _, taskContainer := range entity.Containers {
		container, err := c.store.GetContainer(taskContainer.ID)
		if err != nil {
			log.Tracef("cannot find container %q found in task %q: %s", taskContainer, entity.ID, err)
			continue
		}

		uri, ok := container.EnvVars[v3or4.DefaultMetadataURIv4EnvVariable]
		if ok && uri != "" {
			metaURI = uri
			metaVersion = "v4"
			break
		}

		uri, ok = container.EnvVars[v3or4.DefaultMetadataURIv3EnvVariable]
		if ok && uri != "" {
			metaURI = uri
			metaVersion = "v3"
			break
		}
	}

	if metaURI == "" {
		log.Errorf("failed to get client for metadata v3 or v4 API from task %q and the following containers: %v", entity.ID, entity.Containers)
		return rt
	}

	metaV3orV4 := c.metaV3or4(metaURI, metaVersion)
	taskWithTags, err := metaV3orV4.GetTaskWithTags(ctx)
	if err != nil {
		log.Errorf("failed to get task with tags from metadata %s API: %s", metaVersion, err)
		return rt
	}

	rt = resourceTags{
		tags:                  taskWithTags.TaskTags,
		containerInstanceTags: taskWithTags.ContainerInstanceTags,
	}

	c.resourceTags[entity.ID] = rt

	return rt
}

func v1TaskToV4Task(task v1.Task) v3or4.Task {
	result := v3or4.Task{
		TaskARN:       task.Arn,
		DesiredStatus: task.DesiredStatus,
		KnownStatus:   task.KnownStatus,
		Family:        task.Family,
		Version:       task.Version,
		Containers:    make([]v3or4.Container, 0, len(task.Containers)),
	}

	for _, container := range task.Containers {
		result.Containers = append(result.Containers, v3or4.Container{
			Name:       container.Name,
			DockerName: container.DockerName,
			DockerID:   container.DockerID,
		})
	}
	return result
}
