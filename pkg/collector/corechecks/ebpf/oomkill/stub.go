// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !linux || !cgo

//nolint:revive // TODO(CINT) Fix revive linter
package oomkill

import "github.com/DataDog/datadog-agent/pkg/collector/check"

const (
	// Enabled is true if the check is enabled
	Enabled = false
	// CheckName is the name of the check
	CheckName = "oom_kill"
)

// New creates a new check instance
func New() check.Check {
	return nil
}
