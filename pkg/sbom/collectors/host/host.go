// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package host

import (
	"context"
	"fmt"
	"reflect"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/sbom"
	"github.com/DataDog/datadog-agent/pkg/sbom/collectors"
	"github.com/DataDog/datadog-agent/pkg/util/trivy"
)

const (
	collectorName = "host-collector"
)

type ScanRequest struct {
	Path string
}

func (r *ScanRequest) Collector() string {
	return collectorName
}

type HostCollector struct {
	trivyCollector *trivy.Collector
}

func (c *HostCollector) Init(cfg config.Config) error {
	trivyCollector, err := trivy.GetGlobalCollector(cfg)
	if err != nil {
		return err
	}
	c.trivyCollector = trivyCollector
	return nil
}

func (c *HostCollector) Scan(ctx context.Context, request sbom.ScanRequest, opts sbom.ScanOptions) (sbom.Report, error) {
	hostScanRequest, ok := request.(*ScanRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type '%s' for collector '%s'", reflect.TypeOf(request), collectorName)
	}

	return c.trivyCollector.ScanFilesystem(ctx, hostScanRequest.Path, opts)
}

func init() {
	collectors.RegisterCollector(collectorName, &HostCollector{})
}
