// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package encoding

import (
	"github.com/gogo/protobuf/jsonpb"
	"strings"
	"sync"

	model "github.com/DataDog/agent-payload/v5/process"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/network"
)

var (
	pSerializer = protoSerializer{}
	jSerializer = jsonSerializer{
		marshaller: jsonpb.Marshaler{
			EmitDefaults: true,
		},
	}
)

// Marshaler is an interface implemented by all Connections serializers
type Marshaler interface {
	Marshal(conns *model.Connections) ([]byte, error)
	ContentType() string
}

// Unmarshaler is an interface implemented by all Connections deserializers
type Unmarshaler interface {
	Unmarshal([]byte) (*model.Connections, error)
}

// GetMarshaler returns the appropriate Marshaler based on the given accept header
func GetMarshaler(accept string) Marshaler {
	if strings.Contains(accept, ContentTypeProtobuf) {
		return pSerializer
	}

	return jSerializer
}

// GetUnmarshaler returns the appropriate Unmarshaler based on the given content type
func GetUnmarshaler(ctype string) Unmarshaler {
	if strings.Contains(ctype, ContentTypeProtobuf) {
		return pSerializer
	}

	return jSerializer
}

type ConnectionsModeler struct {
	batchIdx int

	httpEncoder                 *httpEncoder
	http2Encoder                *http2Encoder
	kafkaEncoder                *kafkaEncoder
	connTelemetryMap            map[string]int64
	compilationTelemetryByAsset map[string]*model.RuntimeCompilationTelemetry
	kernelHeaderFetchResult     model.KernelHeaderFetchResult
	coreTelemetryByAsset        map[string]model.COREResult
	agentCfg                    *model.AgentConfiguration
	prebuiltEBPFAssets          []string
}

func InitConnectionsModeler(conns *network.Connections) *ConnectionsModeler {
	return &ConnectionsModeler{
		httpEncoder:                 newHTTPEncoder(conns.HTTP),
		http2Encoder:                newHTTP2Encoder(conns.HTTP2),
		kafkaEncoder:                newKafkaEncoder(conns.Kafka),
		connTelemetryMap:            FormatConnectionTelemetry(conns.ConnTelemetry),
		compilationTelemetryByAsset: FormatCompilationTelemetry(conns.CompilationTelemetryByAsset),
		kernelHeaderFetchResult:     model.KernelHeaderFetchResult(conns.KernelHeaderFetchResult),
		coreTelemetryByAsset:        FormatCORETelemetry(conns.CORETelemetryByAsset),
		prebuiltEBPFAssets:          conns.PrebuiltAssets,
		agentCfg: &model.AgentConfiguration{
			NpmEnabled: config.SystemProbe.GetBool("network_config.enabled"),
			UsmEnabled: config.SystemProbe.GetBool("service_monitoring_config.enabled"),
			DsmEnabled: config.SystemProbe.GetBool("data_streams_config.enabled"),
		},
	}
}

func (c *ConnectionsModeler) ModelConnections(conns *network.Connections) *model.Connections {
	// TODO: Use pool with max batch size
	agentConns := make([]*model.Connection, len(conns.Conns))
	routeIndex := make(map[string]RouteIdx)

	ipc := make(ipCache, len(conns.Conns)/2)
	dnsFormatter := newDNSFormatter(conns, ipc)
	tagsSet := network.NewTagsSet()

	for i, conn := range conns.Conns {
		agentConns[i] = FormatConnection(conn, routeIndex, c.httpEncoder, c.http2Encoder, c.kafkaEncoder, dnsFormatter, ipc, tagsSet)
	}

	routes := make([]*model.Route, len(routeIndex))
	for _, v := range routeIndex {
		routes[v.Idx] = &v.Route
	}

	// TODO: Use pool
	payload := new(model.Connections)
	payload.AgentConfiguration = c.agentCfg
	payload.Conns = agentConns
	payload.Domains = dnsFormatter.Domains()
	payload.Dns = dnsFormatter.DNS()
	payload.Routes = routes
	payload.Tags = tagsSet.GetStrings()

	// TODO: Add that only to the first batch
	payload.ConnTelemetryMap = c.connTelemetryMap
	payload.CompilationTelemetryByAsset = c.compilationTelemetryByAsset
	payload.KernelHeaderFetchResult = c.kernelHeaderFetchResult
	payload.CORETelemetryByAsset = c.coreTelemetryByAsset
	payload.PrebuiltEBPFAssets = c.prebuiltEBPFAssets

	return payload
}

var (
	modelConnectionPool = sync.Pool{
		New: func() any {
			return new(model.Connections)
		},
	}

	agentConnsPool = sync.Pool{
		New: func() any {
			b := make([]*model.Connection, 1000)
			return &b
		},
	}
)

func (c *ConnectionsModeler) ModelConnections2(conns *network.Connections) *model.Connections {
	// TODO: Use pool with max batch size
	agentConnsPtr := agentConnsPool.Get().(*[]*model.Connection)
	defer func() {
		agentConnsPool.Put(agentConnsPtr)
	}()
	agentConns := *agentConnsPtr
	agentConns = agentConns[:len(conns.Conns)]

	routeIndex := make(map[string]RouteIdx)

	ipc := make(ipCache, len(conns.Conns)/2)
	dnsFormatter := newDNSFormatter(conns, ipc)
	tagsSet := network.NewTagsSet()

	for i, conn := range conns.Conns {
		agentConns[i] = FormatConnection(conn, routeIndex, c.httpEncoder, c.http2Encoder, c.kafkaEncoder, dnsFormatter, ipc, tagsSet)
	}

	routes := make([]*model.Route, len(routeIndex))
	for _, v := range routeIndex {
		routes[v.Idx] = &v.Route
	}

	// TODO: Use pool
	payload := modelConnectionPool.Get().(*model.Connections)
	payload.AgentConfiguration = c.agentCfg
	payload.Conns = agentConns
	payload.Domains = dnsFormatter.Domains()
	payload.Dns = dnsFormatter.DNS()
	payload.Routes = routes
	payload.Tags = tagsSet.GetStrings()

	if c.batchIdx == 0 {
		payload.ConnTelemetryMap = c.connTelemetryMap
		payload.CompilationTelemetryByAsset = c.compilationTelemetryByAsset
		payload.KernelHeaderFetchResult = c.kernelHeaderFetchResult
		payload.CORETelemetryByAsset = c.coreTelemetryByAsset
		payload.PrebuiltEBPFAssets = c.prebuiltEBPFAssets
	} else {
		payload.ConnTelemetryMap = nil
		payload.CompilationTelemetryByAsset = nil
		payload.KernelHeaderFetchResult = 0
		payload.CORETelemetryByAsset = nil
		payload.PrebuiltEBPFAssets = nil
	}

	c.batchIdx++

	return payload
}
