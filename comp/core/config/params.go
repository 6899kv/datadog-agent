// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package config

import "github.com/DataDog/datadog-agent/cmd/agent/common"

// Params defines the parameters for this component.
type Params struct {
	// ConfFilePath is the path at which to look for configuration, usually
	// given by the --cfgpath command-line flag.
	ConfFilePath string

	// ConfigName is the root of the name of the configuration file.  The
	// comp/core/config component will search for a file with this name
	// in ConfFilePath, using a variety of extensions.  The default is
	// "datadog".
	ConfigName string

	// SysProbeConfFilePath is the path at which to look for system-probe
	// configuration, usually given by --sysprobecfgpath.  This is not used
	// unless ConfigLoadSysProbe is true.
	SysProbeConfFilePath string

	// ConfigLoadSysProbe determines whether to read the system-probe.yaml into
	// the component's config data.
	ConfigLoadSysProbe bool

	// SecurityAgentConfigFilePaths are the paths at which to look for security-aegnt
	// configuration, usually given by the --cfgpath command-line flag.
	SecurityAgentConfigFilePaths []string

	// ConfigLoadSecurityAgent determines whether to read the config from
	// SecurityAgentConfigFilePaths or from ConfFilePath.
	ConfigLoadSecurityAgent bool

	// ConfigLoadSecrets determines whether secrets in the configuration file
	// should be evaluated.  This is typically false for one-shot commands.
	ConfigLoadSecrets bool

	// ConfigMissingOK determines whether it is a fatal error if the config
	// file does not exist.
	ConfigMissingOK bool

	// DefaultConfPath determines the default configuration path.
	// if DefaultConfPath is empty, then no default configuration path is used.
	DefaultConfPath string
}

// CreateAgentParams creates a new Params for the Core Agent
func CreateAgentParams(confFilePath string, configLoadSecrets bool, options ...func(*Params)) Params {
	params := CreateParams(common.DefaultConfPath, options...)
	params.ConfFilePath = confFilePath
	params.ConfigLoadSecrets = configLoadSecrets
	return params
}

// CreateParams creates a new Params
func CreateParams(defaultConfPath string, options ...func(*Params)) Params {
	params := Params{
		DefaultConfPath: defaultConfPath,
	}
	for _, o := range options {
		o(&params)
	}
	return params
}

func WithConfigName(name string) func(*Params) {
	return func(b *Params) {
		b.ConfigName = name
	}
}
