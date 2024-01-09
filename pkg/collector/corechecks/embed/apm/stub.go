// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !apm && !windows && !linux

//nolint:revive // TODO(APM) Fix revive linter
package apm

import "github.com/DataDog/datadog-agent/pkg/collector/check"

const (
	// Enabled is true if the check is enabled
	Enabled = false
	// CheckName is the name of the check
	CheckName = "apm"
)

// Factory creates a new check instance
func Factory() check.Check {
	return nil
}
