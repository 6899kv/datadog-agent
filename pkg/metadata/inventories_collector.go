// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package metadata

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/metadata/inventories"
	"github.com/DataDog/datadog-agent/pkg/status/expvarcollector"
	"github.com/DataDog/datadog-agent/pkg/util/hostname"
	"github.com/DataDog/datadog-agent/pkg/util/log"

	"github.com/DataDog/datadog-agent/pkg/serializer"
)

func init() {
	expvarcollector.RegisterExpvarReport("inventories", func() (interface{}, error) {
		inventories := expvar.Get("inventories")
		var inventoriesStats map[string]interface{}
		if inventories != nil {
			inventoriesStatsJSON := []byte(inventories.String())
			json.Unmarshal(inventoriesStatsJSON, &inventoriesStats) //nolint:errcheck
		}

		checkMetadata := map[string]map[string]string{}
		if data, ok := inventoriesStats["check_metadata"]; ok {
			for _, instances := range data.(map[string]interface{}) {
				for _, instance := range instances.([]interface{}) {
					metadata := map[string]string{}
					checkHash := ""
					for k, v := range instance.(map[string]interface{}) {
						if vStr, ok := v.(string); ok {
							if k == "config.hash" {
								checkHash = vStr
							} else if k != "config.provider" {
								metadata[k] = vStr
							}
						}
					}
					if checkHash != "" && len(metadata) != 0 {
						checkMetadata[checkHash] = metadata
					}
				}
			}
		}

		return checkMetadata, nil
	})
}

const (
	// defaultInventoriesMinInterval is the default value for inventories_min_interval, in seconds
	defaultInventoriesMinInterval = 5 * 60
	// defaultInventoriesMaxInterval is the default value for inventories_max_interval, in seconds
	defaultInventoriesMaxInterval = 10 * 60
)

type inventoriesCollector struct {
	coll inventories.CollectorInterface
	sc   *Scheduler
}

func getPayload(ctx context.Context, coll inventories.CollectorInterface, withConfigs bool) (*inventories.Payload, error) {
	hostnameDetected, err := hostname.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to submit inventories metadata payload, no hostname: %s", err)
	}

	return inventories.GetPayload(ctx, hostnameDetected, coll, withConfigs), nil
}

// Send collects the data needed and submits the payload
func (c inventoriesCollector) Send(ctx context.Context, s serializer.MetricSerializer) error {
	if s == nil {
		return nil
	}

	payload, err := getPayload(ctx, c.coll, true)
	if err != nil {
		return err
	} else if payload != nil {
		if err := s.SendMetadata(payload); err != nil {
			return fmt.Errorf("unable to submit inventories payload, %s", err)
		}
	}

	return nil
}

// getMinInterval gets the inventories_min_interval value, applying the default if it is zero.
func getMinInterval() time.Duration {
	minInterval := time.Duration(config.Datadog.GetInt("inventories_min_interval")) * time.Second
	if minInterval <= 0 {
		minInterval = defaultInventoriesMinInterval * time.Second
	}
	return minInterval
}

// getMaxInterval gets the inventories_max_interval value, applying the default if it is zero.
func getMaxInterval() time.Duration {
	maxInterval := time.Duration(config.Datadog.GetInt("inventories_max_interval")) * time.Second
	if maxInterval <= 0 {
		maxInterval = defaultInventoriesMaxInterval * time.Second
	}
	return maxInterval
}

// Init initializes the inventory metadata collection. This should be called in
// all agents that wish to track inventory, after configuration is initialized.
func (c inventoriesCollector) Init() error {
	return inventories.StartMetadataUpdatedGoroutine(c.sc, getMinInterval())
}

// SetupInventories registers the inventories collector into the Scheduler and, if configured, schedules it
func SetupInventories(sc *Scheduler, coll inventories.CollectorInterface) error {
	if !config.Datadog.GetBool("enable_metadata_collection") {
		log.Debugf("Metadata collection disabled: inventories payload will not be collected nor sent")
		return nil
	}
	if !config.Datadog.GetBool("inventories_enabled") {
		log.Debugf("inventories metadata is disabled: inventories payload will not be collected nor sent")
		return nil
	}

	ic := inventoriesCollector{
		coll: coll,
		sc:   sc,
	}
	RegisterCollector("inventories", ic)

	if err := sc.AddCollector("inventories", getMaxInterval()); err != nil {
		return err
	}

	expvar.Publish("inventories", expvar.Func(func() interface{} {
		log.Debugf("Creating inventory payload for expvar")
		p, err := getPayload(context.TODO(), coll, false)
		if err != nil {
			log.Errorf("Could not create inventory payload for expvar: %s", err)
			return &inventories.Payload{}
		}
		return p
	}))

	return nil
}
