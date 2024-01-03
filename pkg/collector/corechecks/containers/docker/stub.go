// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2021-present Datadog, Inc.

//nolint:revive // TODO(CINT) Fix revive linter

//go:build !docker

package docker

import (
	"github.com/DataDog/datadog-agent/comp/core/workloadmeta"
	"github.com/DataDog/datadog-agent/pkg/collector/check"
)

const (
	Enabled   = false
	CheckName = "docker"
)

func NewFactory(store workloadmeta.Component) func() check.Check {
	return nil
}
