// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !windows && !darwin

package config

// defaultSystemProbeAddress is the default unix socket path to be used for connecting to the system probe
const defaultSystemProbeAddress = "/opt/datadog-agent/run/sysprobe.sock"
