// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2022-present Datadog, Inc.

//go:build trivy

package sbom

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/DataDog/datadog-agent/pkg/aggregator/sender"
	"github.com/DataDog/datadog-agent/pkg/autodiscovery/integration"
	"github.com/DataDog/datadog-agent/pkg/collector/check"
	core "github.com/DataDog/datadog-agent/pkg/collector/corechecks"
	ddConfig "github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/config/remote"
	"github.com/DataDog/datadog-agent/pkg/config/remote/data"
	"github.com/DataDog/datadog-agent/pkg/remoteconfig/state"
	"github.com/DataDog/datadog-agent/pkg/security/utils"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/workloadmeta"
)

const (
	checkName    = "sbom"
	metricPeriod = 15 * time.Minute
)

func init() {
	core.RegisterCheck(checkName, CheckFactory)
}

// Config holds the container_image check configuration
type Config struct {
	ChunkSize                       int `yaml:"chunk_size"`
	NewSBOMMaxLatencySeconds        int `yaml:"new_sbom_max_latency_seconds"`
	ContainerPeriodicRefreshSeconds int `yaml:"periodic_refresh_seconds"`
	HostPeriodicRefreshSeconds      int `yaml:"host_periodic_refresh_seconds"`
	HostHeartbeatValiditySeconds    int `yaml:"host_heartbeat_validity_seconds"`
}

type configValueRange struct {
	min          int
	max          int
	defaultValue int
}

var /* const */ (
	chunkSizeValueRange = &configValueRange{
		min:          1,
		max:          100,
		defaultValue: 1,
	}

	newSBOMMaxLatencySecondsValueRange = &configValueRange{
		min:          1,   // 1 seconds
		max:          300, // 5 min
		defaultValue: 30,  // 30 seconds
	}

	containerPeriodicRefreshSecondsValueRange = &configValueRange{
		min:          60,     // 1 min
		max:          604800, // 1 week
		defaultValue: 3600,   // 1 hour
	}

	hostPeriodicRefreshSecondsValueRange = &configValueRange{
		min:          60,     // 1 min
		max:          604800, // 1 week
		defaultValue: 3600,   // 1 hour
	}

	hostHeartbeatValiditySeconds = &configValueRange{
		min:          60,        // 1 min
		max:          604800,    // 1 week
		defaultValue: 3600 * 24, // 1 day
	}
)

func validateValue(val *int, valueRange *configValueRange) {
	if *val == 0 {
		*val = valueRange.defaultValue
	} else if *val < valueRange.min {
		*val = valueRange.min
	} else if *val > valueRange.max {
		*val = valueRange.max
	}
}

// Parse parses the configuration
func (c *Config) Parse(data []byte) error {
	if err := yaml.Unmarshal(data, c); err != nil {
		return err
	}

	validateValue(&c.ChunkSize, chunkSizeValueRange)
	validateValue(&c.NewSBOMMaxLatencySeconds, newSBOMMaxLatencySecondsValueRange)
	validateValue(&c.ContainerPeriodicRefreshSeconds, containerPeriodicRefreshSecondsValueRange)
	validateValue(&c.HostPeriodicRefreshSeconds, hostPeriodicRefreshSecondsValueRange)
	validateValue(&c.HostHeartbeatValiditySeconds, hostHeartbeatValiditySeconds)

	return nil
}

// Check reports SBOM
type Check struct {
	core.CheckBase
	workloadmetaStore workloadmeta.Store
	instance          *Config
	processor         *processor
	sender            sender.Sender
	stopCh            chan struct{}
	rcClient          *remote.Client
}

// CheckFactory registers the sbom check
func CheckFactory() check.Check {
	return &Check{
		CheckBase:         core.NewCheckBase(checkName),
		workloadmetaStore: workloadmeta.GetGlobalStore(),
		instance:          &Config{},
		stopCh:            make(chan struct{}),
	}
}

// Configure parses the check configuration and initializes the sbom check
func (c *Check) Configure(senderManager sender.SenderManager, integrationConfigDigest uint64, config, initConfig integration.Data, source string) error {
	if !ddConfig.Datadog.GetBool("sbom.enabled") {
		return errors.New("collection of SBOM is disabled")
	}

	if err := c.CommonConfigure(senderManager, integrationConfigDigest, initConfig, config, source); err != nil {
		return err
	}

	if err := c.instance.Parse(config); err != nil {
		return err
	}

	sender, err := c.GetSender()
	if err != nil {
		return err
	}

	c.sender = sender
	sender.SetNoIndex(true)

	if c.processor, err = newProcessor(
		c.workloadmetaStore,
		sender,
		c.instance.ChunkSize,
		time.Duration(c.instance.NewSBOMMaxLatencySeconds)*time.Second,
		ddConfig.Datadog.GetBool("sbom.host.enabled"),
		time.Duration(c.instance.HostHeartbeatValiditySeconds)*time.Second); err != nil {
		return err
	}

	agentVersion, err := utils.GetAgentSemverVersion()
	if err != nil {
		return fmt.Errorf("failed to parse agent version: %v", err)
	}

	c.rcClient, err = remote.NewUnverifiedGRPCClient("core-agent", agentVersion.String(), []data.Product{"DEBUG"}, 3*time.Second)
	if err != nil {
		return err
	}

	done := make(chan map[string]interface{}, 100)
	c.rcClient.Subscribe("DEBUG", func(configs map[string]state.RawConfig, _ func(string, state.ApplyStatus)) {
		defer c.sender.Commit()

		for _, cfg := range configs {
			var data map[string]string
			if err := json.Unmarshal(cfg.Config, &data); err != nil {
				log.Warnf("Failed to parse agent task: %w", err)
			}

			taskType := data["type"]
			region, found := data["region"]
			if !found {
				log.Errorf("No region in SBOM scan request")
			}

			tags := []string{
				fmt.Sprintf("region:%s", region),
				fmt.Sprintf("type:%s", taskType),
			}

			c.sender.Count("datadog.sidescanner.scans.started", 1.0, "", tags)
			switch taskType {
			case "sbom-ebs-scan":
				target, found := data["id"]
				if !found {
					log.Errorf("No target in SBOM scan request")
				}
				hostname, found := data["hostname"]
				if !found {
					log.Errorf("No hostname specified in SBOM scan request")
				}
				c.processor.processEBS("ebs:"+target, region, hostname, done)
			case "sbom-lambda-scan":
				functionName, found := data["function_name"]
				if !found {
					log.Errorf("No function name specified in SBOM scan request")
				}
				c.processor.processLambda(functionName, region, done)
			default:
				log.Errorf("Unsupported scan request type '%s'", taskType)
			}
		}
	})

	go func() {
		for cookie := range done {
			tags := cookie["tags"].([]string)
			c.sender.Count("datadog.sidescanner.scans.finished", 1.0, "", tags)
			c.sender.Histogram("datadog.sidescanner.scans.duration",
				float64(time.Since(cookie["startTime"].(time.Time)).Milliseconds()), "", tags)
		}
	}()

	return nil
}

// Run starts the sbom check
func (c *Check) Run() error {
	log.Infof("Starting long-running check %q", c.ID())
	defer log.Infof("Shutting down long-running check %q", c.ID())

	c.rcClient.Start()

	imgEventsCh := c.workloadmetaStore.Subscribe(
		checkName,
		workloadmeta.NormalPriority,
		workloadmeta.NewFilter(
			[]workloadmeta.Kind{
				workloadmeta.KindContainerImageMetadata,
				workloadmeta.KindContainer,
			},
			workloadmeta.SourceAll,
			workloadmeta.EventTypeAll,
		),
	)

	// Trigger an initial scan on host
	c.processor.processHostRefresh()

	c.sendUsageMetrics()

	containerPeriodicRefreshTicker := time.NewTicker(time.Duration(c.instance.ContainerPeriodicRefreshSeconds) * time.Second)
	defer containerPeriodicRefreshTicker.Stop()

	hostPeriodicRefreshTicker := time.NewTicker(time.Duration(c.instance.HostPeriodicRefreshSeconds) * time.Second)
	defer hostPeriodicRefreshTicker.Stop()

	metricTicker := time.NewTicker(metricPeriod)
	defer metricTicker.Stop()

	for {
		select {
		case eventBundle := <-imgEventsCh:
			c.processor.processContainerImagesEvents(eventBundle)
		case <-containerPeriodicRefreshTicker.C:
			c.processor.processContainerImagesRefresh(c.workloadmetaStore.ListImages())
		case <-hostPeriodicRefreshTicker.C:
			c.processor.processHostRefresh()
		case <-metricTicker.C:
			c.sendUsageMetrics()
		case <-c.stopCh:
			c.processor.stop()
			return nil
		}
	}
}

func (c *Check) sendUsageMetrics() {
	c.sender.Count("datadog.agent.sbom.container_images.running", 1.0, "", nil)

	if ddConfig.Datadog.GetBool("sbom.host.enabled") {
		c.sender.Count("datadog.agent.sbom.hosts.running", 1.0, "", nil)
	}

	c.sender.Commit()
}

// Stop stops the sbom check
func (c *Check) Stop() {
	close(c.stopCh)
}

// Interval returns 0. It makes sbom a long-running check
func (c *Check) Interval() time.Duration {
	return 0
}
