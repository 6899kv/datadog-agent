// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package core implements the "core" bundle, providing services common to all
// agent flavors and binaries.
//
// The constituent components serve as utilities and are mostly independent of
// one another.  Other components should depend on any components they need.
//
// This bundle does not depend on any other bundles.

//go:build test

package core

import (
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameimpl"
	"github.com/DataDog/datadog-agent/comp/core/log"
	"github.com/DataDog/datadog-agent/comp/core/sysprobeconfig/sysprobeconfigimpl"
	"github.com/DataDog/datadog-agent/comp/core/telemetry"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
)

// team: agent-shared-components

// MockBundle defines the mock fx options for this bundle.
var MockBundle fxutil.BundleOptions

func init() {
	MockBundle = fxutil.Bundle(
		fx.Provide(func(params BundleParams) config.Params { return params.ConfigParams }),
		config.MockModule,
		fx.Supply(log.Params{}),
		log.MockModule,
		fx.Provide(func(params BundleParams) sysprobeconfigimpl.Params { return params.SysprobeConfigParams }),
		sysprobeconfigimpl.MockModule,
		telemetry.Module,
		hostnameimpl.MockModule,
	)
}
