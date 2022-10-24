// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package sysprobeconfig

import (
	"os"
	"strings"
	"testing"

	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core/internal"
	"github.com/DataDog/datadog-agent/pkg/config"
)

// cfg implements the Component.
type cfg struct {
	// this component is currently implementing a thin wrapper around pkg/config,
	// and uses globals in that package.
	config.ConfigReader

	// warnings are the warnings generated during setup
	warnings *config.Warnings
}

type dependencies struct {
	fx.In

	Params internal.BundleParams
}

func newConfig(deps dependencies) (Component, error) {
	warnings, err := setupConfig(
		deps.Params.SysProbeConfFilePath,
	)
	if err != nil {
		return nil, err
	}

	return &cfg{config.SystemProbe, warnings}, nil
}

func (c *cfg) Warnings() *config.Warnings {
	return c.warnings
}

func newMock(deps dependencies, t testing.TB) Component {
	old := config.SystemProbe
	config.SystemProbe = config.NewConfig("mock", "XXXX", strings.NewReplacer())
	c := &cfg{
		warnings: &config.Warnings{},
	}

	// call InitSystemProbeConfig to set defaults.
	config.InitSystemProbeConfig(config.SystemProbe)

	// Viper's `GetXxx` methods read environment variables at the time they are
	// called, if those names were passed explicitly to BindEnv*(), so we must
	// also strip all `DD_` environment variables for the duration of the test.
	oldEnv := os.Environ()
	for _, kv := range oldEnv {
		if strings.HasPrefix(kv, "DD_") {
			kvslice := strings.SplitN(kv, "=", 2)
			os.Unsetenv(kvslice[0])
		}
	}
	t.Cleanup(func() {
		for _, kv := range oldEnv {
			kvslice := strings.SplitN(kv, "=", 2)
			os.Setenv(kvslice[0], kvslice[1])
		}
	})

	// swap the existing config back at the end of the test.
	t.Cleanup(func() { config.SystemProbe = old })

	return c
}

func (c *cfg) Set(key string, value interface{}) {
	config.SystemProbe.Set(key, value)
}
