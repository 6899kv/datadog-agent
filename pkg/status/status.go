// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package status

import (
	"context"
	"encoding/json"
	"expvar"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-agent/cmd/agent/common"
	hostMetadataUtils "github.com/DataDog/datadog-agent/comp/metadata/host/utils"
	netflowServer "github.com/DataDog/datadog-agent/comp/netflow/server"
	"github.com/DataDog/datadog-agent/pkg/clusteragent/admission"
	"github.com/DataDog/datadog-agent/pkg/clusteragent/clusterchecks"
	"github.com/DataDog/datadog-agent/pkg/clusteragent/custommetrics"
	"github.com/DataDog/datadog-agent/pkg/clusteragent/externalmetrics"
	"github.com/DataDog/datadog-agent/pkg/clusteragent/orchestrator"
	"github.com/DataDog/datadog-agent/pkg/collector/check"
	checkid "github.com/DataDog/datadog-agent/pkg/collector/check/id"
	checkstats "github.com/DataDog/datadog-agent/pkg/collector/check/stats"
	"github.com/DataDog/datadog-agent/pkg/collector/python"
	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/config/utils"
	logsStatus "github.com/DataDog/datadog-agent/pkg/logs/status"
	"github.com/DataDog/datadog-agent/pkg/status/expvarcollector"
	"github.com/DataDog/datadog-agent/pkg/util/containers"
	"github.com/DataDog/datadog-agent/pkg/util/flavor"
	httputils "github.com/DataDog/datadog-agent/pkg/util/http"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes/apiserver"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/version"
)

var timeFormat = "2006-01-02 15:04:05.999 MST"

// GetStatus grabs the status from expvar and puts it into a map
func GetStatus(verbose bool) (map[string]interface{}, error) {
	stats, err := getCommonStatus()
	if err != nil {
		return nil, err
	}
	stats["verbose"] = verbose
	stats["config"] = getPartialConfig()
	metadata := stats["metadata"].(*hostMetadataUtils.Payload)
	hostTags := make([]string, 0, len(metadata.HostTags.System)+len(metadata.HostTags.GoogleCloudPlatform))
	hostTags = append(hostTags, metadata.HostTags.System...)
	hostTags = append(hostTags, metadata.HostTags.GoogleCloudPlatform...)
	stats["hostTags"] = hostTags

	pythonVersion := python.GetPythonVersion()
	stats["python_version"] = strings.Split(pythonVersion, " ")[0]
	stats["hostinfo"] = hostMetadataUtils.GetInformation()

	stats["JMXStatus"] = GetJMXStatus()
	stats["JMXStartupError"] = GetJMXStartupError()

	stats["logsStats"] = logsStatus.Get(verbose)

	stats["otlp"] = GetOTLPStatus()

	endpointsInfos, err := getEndpointsInfos()
	if endpointsInfos != nil && err == nil {
		stats["endpointsInfos"] = endpointsInfos
	} else {
		stats["endpointsInfos"] = nil
	}

	if config.Datadog.GetBool("cluster_agent.enabled") {
		stats["clusterAgentStatus"] = getDCAStatus()
	}

	if config.SystemProbe.GetBool("system_probe_config.enabled") {
		stats["systemProbeStats"] = GetSystemProbeStats(config.SystemProbe.GetString("system_probe_config.sysprobe_socket"))
	}

	stats["processAgentStatus"] = GetProcessAgentStatus()

	if !config.Datadog.GetBool("no_proxy_nonexact_match") {
		stats["TransportWarnings"] = httputils.GetNumberOfWarnings() > 0
		stats["NoProxyIgnoredWarningMap"] = httputils.GetProxyIgnoredWarnings()
		stats["NoProxyUsedInFuture"] = httputils.GetProxyUsedInFutureWarnings()
		stats["NoProxyChanged"] = httputils.GetProxyIgnoredWarnings()
	}

	if config.IsContainerized() {
		stats["adEnabledFeatures"] = config.GetDetectedFeatures()
		if common.AC != nil {
			stats["adConfigErrors"] = common.AC.GetAutodiscoveryErrors()
		}
		stats["filterErrors"] = containers.GetFilterErrors()
	}

	return stats, nil
}

// GetAndFormatStatus gets and formats the status all in one go
func GetAndFormatStatus() ([]byte, error) {
	s, err := GetStatus(true)
	if err != nil {
		return nil, err
	}

	statusJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	st, err := FormatStatus(statusJSON)
	if err != nil {
		return nil, err
	}

	return []byte(st), nil
}

// GetCheckStatusJSON gets the status of a single check as JSON
func GetCheckStatusJSON(c check.Check, cs *checkstats.Stats) ([]byte, error) {
	s, err := GetStatus(false)
	if err != nil {
		return nil, err
	}
	checks := s["runnerStats"].(map[string]interface{})["Checks"].(map[string]interface{})
	checks[c.String()] = make(map[checkid.ID]interface{})
	checks[c.String()].(map[checkid.ID]interface{})[c.ID()] = cs

	statusJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	return statusJSON, nil
}

// GetCheckStatus gets the status of a single check as human-readable text
func GetCheckStatus(c check.Check, cs *checkstats.Stats) ([]byte, error) {
	statusJSON, err := GetCheckStatusJSON(c, cs)
	if err != nil {
		return nil, err
	}

	st, err := renderCheckStats(statusJSON, c.String())
	if err != nil {
		return nil, err
	}

	return []byte(st), nil
}

// GetDCAStatus grabs the status from expvar and puts it into a map
func GetDCAStatus(verbose bool) (map[string]interface{}, error) {
	stats, err := getCommonStatus()
	if err != nil {
		return nil, err
	}

	stats["config"] = getDCAPartialConfig()
	stats["leaderelection"] = getLeaderElectionDetails()

	stats["logsStats"] = logsStatus.Get(verbose)

	endpointsInfos, err := getEndpointsInfos()
	if endpointsInfos != nil && err == nil {
		stats["endpointsInfos"] = endpointsInfos
	} else {
		stats["endpointsInfos"] = nil
	}

	apiCl, apiErr := apiserver.GetAPIClient()
	if apiErr != nil {
		stats["custommetrics"] = map[string]string{"Error": apiErr.Error()}
		stats["admissionWebhook"] = map[string]string{"Error": apiErr.Error()}
	} else {
		stats["custommetrics"] = custommetrics.GetStatus(apiCl.Cl)
		stats["admissionWebhook"] = admission.GetStatus(apiCl.Cl)
	}

	if config.Datadog.GetBool("external_metrics_provider.use_datadogmetric_crd") {
		stats["externalmetrics"] = externalmetrics.GetStatus()
	} else {
		stats["externalmetrics"] = apiserver.GetStatus()
	}

	if config.Datadog.GetBool("cluster_checks.enabled") {
		cchecks, err := clusterchecks.GetStats()
		if err != nil {
			log.Errorf("Error grabbing clusterchecks stats: %s", err)
		} else {
			stats["clusterchecks"] = cchecks
		}
	}

	stats["adEnabledFeatures"] = config.GetDetectedFeatures()
	if common.AC != nil {
		stats["adConfigErrors"] = common.AC.GetAutodiscoveryErrors()
	}
	stats["filterErrors"] = containers.GetFilterErrors()

	if config.Datadog.GetBool("orchestrator_explorer.enabled") {
		if apiErr != nil {
			stats["orchestrator"] = map[string]string{"Error": apiErr.Error()}
		} else {
			orchestratorStats := orchestrator.GetStatus(context.TODO(), apiCl.Cl)
			stats["orchestrator"] = orchestratorStats
		}
	}

	return stats, nil
}

// GetAndFormatDCAStatus gets and formats the DCA status all in one go.
func GetAndFormatDCAStatus() ([]byte, error) {
	s, err := GetDCAStatus(true)
	if err != nil {
		log.Infof("Error while getting status %q", err)
		return nil, err
	}
	statusJSON, err := json.Marshal(s)
	if err != nil {
		log.Infof("Error while marshalling %q", err)
		return nil, err
	}
	st, err := FormatDCAStatus(statusJSON)
	if err != nil {
		log.Infof("Error formatting the status %q", err)
		return nil, err
	}
	return []byte(st), nil
}

// GetAndFormatSecurityAgentStatus gets and formats the security agent status
func GetAndFormatSecurityAgentStatus(runtimeStatus, complianceStatus map[string]interface{}) ([]byte, error) {
	s, err := GetStatus(true)
	if err != nil {
		return nil, err
	}
	s["runtimeSecurityStatus"] = runtimeStatus
	s["complianceStatus"] = complianceStatus

	statusJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	st, err := FormatSecurityAgentStatus(statusJSON)
	if err != nil {
		return nil, err
	}

	return []byte(st), nil
}

// getDCAPartialConfig returns config parameters of interest for the status page.
func getDCAPartialConfig() map[string]string {
	conf := make(map[string]string)
	conf["log_level"] = config.Datadog.GetString("log_level")
	conf["confd_path"] = config.Datadog.GetString("confd_path")
	return conf
}

// getPartialConfig returns config parameters of interest for the status page
func getPartialConfig() map[string]string {
	conf := make(map[string]string)
	conf["log_file"] = config.Datadog.GetString("log_file")
	conf["log_level"] = config.Datadog.GetString("log_level")
	conf["confd_path"] = config.Datadog.GetString("confd_path")
	conf["additional_checksd"] = config.Datadog.GetString("additional_checksd")

	conf["fips_enabled"] = config.Datadog.GetString("fips.enabled")
	conf["fips_local_address"] = config.Datadog.GetString("fips.local_address")
	conf["fips_port_range_start"] = config.Datadog.GetString("fips.port_range_start")

	forwarderStorageMaxSizeInBytes := config.Datadog.GetInt("forwarder_storage_max_size_in_bytes")
	if forwarderStorageMaxSizeInBytes > 0 {
		conf["forwarder_storage_max_size_in_bytes"] = strconv.Itoa(forwarderStorageMaxSizeInBytes)
	}
	return conf
}

func getEndpointsInfos() (map[string]interface{}, error) {
	endpoints, err := utils.GetMultipleEndpoints(config.Datadog)
	if err != nil {
		return nil, err
	}

	endpointsInfos := make(map[string]interface{})

	// obfuscate the api keys
	for endpoint, keys := range endpoints {
		for i, key := range keys {
			if len(key) > 5 {
				keys[i] = key[len(key)-5:]
			}
		}
		endpointsInfos[endpoint] = keys
	}

	return endpointsInfos, nil
}

// getCommonStatus grabs the status from expvar and puts it into a map.
// It gets the status elements common to all Agent flavors.
func getCommonStatus() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	stats, errors := expvarcollector.Report(stats)

	if len(errors) > 0 {
		log.Errorf("Error Getting ExpVar Stats: %v", errors)
	}

	stats["version"] = version.AgentVersion
	stats["flavor"] = flavor.GetFlavor()
	stats["metadata"] = hostMetadataUtils.GetFromCache(context.TODO(), config.Datadog)
	stats["conf_file"] = config.Datadog.ConfigFileUsed()
	stats["pid"] = os.Getpid()
	stats["go_version"] = runtime.Version()
	stats["agent_start_nano"] = config.StartTime.UnixNano()
	stats["build_arch"] = runtime.GOARCH
	now := time.Now()
	stats["time_nano"] = now.UnixNano()
	stats["netflowStats"] = netflowServer.GetStatus()

	return stats, nil
}

// GetExpvarRunnerStats grabs the status of the runner from expvar
// and puts it into a CLCChecks struct
func GetExpvarRunnerStats() (CLCChecks, error) {
	runnerStatsJSON := []byte(expvar.Get("runner").String())
	return convertExpvarRunnerStats(runnerStatsJSON)
}

func convertExpvarRunnerStats(inputJSON []byte) (CLCChecks, error) {
	runnerStats := CLCChecks{}
	err := json.Unmarshal(inputJSON, &runnerStats)
	return runnerStats, err
}
