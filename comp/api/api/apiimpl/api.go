// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package apiimpl implements the internal Agent API which exposes endpoints such as config, flare or status
package apiimpl

import (
	"net"

	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/aggregator/demultiplexer"
	"github.com/DataDog/datadog-agent/comp/api/api"
	"github.com/DataDog/datadog-agent/comp/core/flare"
	"github.com/DataDog/datadog-agent/comp/core/secrets"
	"github.com/DataDog/datadog-agent/comp/core/status"
	"github.com/DataDog/datadog-agent/comp/core/workloadmeta"
	"github.com/DataDog/datadog-agent/comp/dogstatsd/replay"
	dogstatsdServer "github.com/DataDog/datadog-agent/comp/dogstatsd/server"
	dogstatsddebug "github.com/DataDog/datadog-agent/comp/dogstatsd/serverDebug"
	logsAgent "github.com/DataDog/datadog-agent/comp/logs/agent"
	"github.com/DataDog/datadog-agent/comp/metadata/host"
	"github.com/DataDog/datadog-agent/comp/metadata/inventoryagent"
	"github.com/DataDog/datadog-agent/comp/metadata/inventorychecks"
	"github.com/DataDog/datadog-agent/comp/metadata/inventoryhost"
	"github.com/DataDog/datadog-agent/pkg/aggregator/sender"
	remoteconfig "github.com/DataDog/datadog-agent/pkg/config/remote/service"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
	"github.com/DataDog/datadog-agent/pkg/util/optional"
)

// Module defines the fx options for this component.
func Module() fxutil.Module {
	return fxutil.Component(
		fx.Provide(newAPIServer))
}

type apiServer struct {
	flare           flare.Component
	dogstatsdServer dogstatsdServer.Component
	capture         replay.Component
	serverDebug     dogstatsddebug.Component
	hostMetadata    host.Component
	invAgent        inventoryagent.Component
	demux           demultiplexer.Component
	invHost         inventoryhost.Component
	secretResolver  secrets.Component
	invChecks       inventorychecks.Component
	statusComponent status.Component
}

type dependencies struct {
	fx.In

	Flare           flare.Component
	DogstatsdServer dogstatsdServer.Component
	Capture         replay.Component
	ServerDebug     dogstatsddebug.Component
	HostMetadata    host.Component
	InvAgent        inventoryagent.Component
	Demux           demultiplexer.Component
	InvHost         inventoryhost.Component
	SecretResolver  secrets.Component
	InvChecks       inventorychecks.Component
	StatusComponent status.Component
}

var _ api.Component = (*apiServer)(nil)

func newAPIServer(deps dependencies) api.Component {
	return &apiServer{
		flare:           deps.Flare,
		dogstatsdServer: deps.DogstatsdServer,
		capture:         deps.Capture,
		serverDebug:     deps.ServerDebug,
		hostMetadata:    deps.HostMetadata,
		invAgent:        deps.InvAgent,
		demux:           deps.Demux,
		invHost:         deps.InvHost,
		secretResolver:  deps.SecretResolver,
		invChecks:       deps.InvChecks,
		statusComponent: deps.StatusComponent,
	}
}

// StartServer creates the router and starts the HTTP server
func (server *apiServer) StartServer(
	configService *remoteconfig.Service,
	wmeta workloadmeta.Component,
	logsAgent optional.Option[logsAgent.Component],
	senderManager sender.DiagnoseSenderManager,
) error {
	return StartServers(configService,
		server.flare,
		server.dogstatsdServer,
		server.capture,
		server.serverDebug,
		wmeta,
		logsAgent,
		senderManager,
		server.hostMetadata,
		server.invAgent,
		server.demux,
		server.invHost,
		server.secretResolver,
		server.invChecks,
		server.statusComponent,
	)
}

// StopServer closes the connection and the server
// stops listening to new commands.
func (server *apiServer) StopServer() {
	StopServers()
}

// ServerAddress returns the server address.
func (server *apiServer) ServerAddress() *net.TCPAddr {
	return ServerAddress()
}
