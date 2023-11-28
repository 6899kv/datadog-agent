// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build darwin

package consumer

import (
	"github.com/DataDog/datadog-agent/pkg/process/events/model"
	smodel "github.com/DataDog/datadog-agent/pkg/security/secl/model"
)

// Copy copies the necessary fields from the event received from the event monitor
func (p *ProcessConsumer) Copy(event *smodel.Event) any {
	return &model.ProcessEvent{
		EventType:      model.NewEventType(event.GetEventType().String()),
		CollectionTime: event.GetTimestamp(),
		Pid:            event.GetProcessPid(),
		ContainerID:    event.GetContainerId(),
		Ppid:           event.GetProcessPpid(),
		UID:            event.GetProcessUid(),
		GID:            event.GetProcessGid(),
		Username:       event.GetProcessUser(),
		Group:          event.GetProcessGroup(),
		Exe:            event.GetExecFilePath(),
		Cmdline:        event.GetProcessArgv(),
		ExecTime:       event.GetProcessExecTime(),
		ExitTime:       event.GetProcessExitTime(),
		ExitCode:       event.GetExitCode(),
	}
}
