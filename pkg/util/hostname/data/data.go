// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package data provides utilities for hostname and the hostname provider
package data

const (
	ConfigProvider  = "configuration"
	FargateProvider = "fargate"
)

// Data contains hostname and the hostname provider
type Data struct {
	Hostname string
	Provider string
}

// FromConfiguration returns true if the hostname was found through the configuration file
func (h Data) FromConfiguration() bool {
	return h.Provider == ConfigProvider
}

// FromFargate returns true if the hostname was found through Fargate
func (h Data) FromFargate() bool {
	return h.Provider == FargateProvider
}
