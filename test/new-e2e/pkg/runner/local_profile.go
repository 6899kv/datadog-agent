// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package runner

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/runner/parameters"
)

const (
	defaultLocalEnvironments string = "aws/agent-sandbox"
)

// NewLocalProfile creates a new local profile
func NewLocalProfile() (Profile, error) {
	envValueStore := parameters.NewEnvStore(EnvPrefix)

	configPath, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}

	var store parameters.Store
	if configPath != "" {
		configFileValueStore, err := parameters.NewConfigFileStore(configPath)
		if err != nil {
			return nil, fmt.Errorf("error when reading the config file %v: %v", configPath, err)
		}
		store = parameters.NewCascadingStore(envValueStore, configFileValueStore)
	} else {
		store = parameters.NewCascadingStore(envValueStore)
	}
	// inject default params
	environments, err := store.GetWithDefault(parameters.Environments, defaultLocalEnvironments)
	if err != nil {
		return nil, err
	}
	outputDir, err := getLocalOutputDir()
	if err != nil {
		return nil, err
	}
	return localProfile{baseProfile: newProfile("e2elocal", strings.Split(environments, " "), store, nil, outputDir)}, nil
}

func getLocalOutputDir() (string, error) {
	// walk up the directory tree to find the root of the project (where .git is)
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for wd != "" && wd != "/" {
		if _, err := os.Stat(filepath.Join(wd, ".git")); err == nil {
			return wd, nil
		}
		wd = filepath.Dir(wd)
	}
	// If we don't find .git then let base profile handle the default
	return "", nil
}

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to get the home dir")
	}
	configPath := path.Join(homeDir, ".test_infra_config.yaml")

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return configPath, nil
}

type localProfile struct {
	baseProfile
}

// NamePrefix returns a prefix to name objects based on local username
func (p localProfile) NamePrefix() string {
	// Stack names may only contain alphanumeric characters, hyphens, underscores, or periods.
	// As NamePrefix is used as stack name, we sanitize the user name.
	var username string
	user, err := user.Current()
	if err == nil {
		username = user.Username
	}

	if username == "" || username == "root" {
		username = "nouser"
	}

	if sepIdx := strings.Index(username, `\`); sepIdx != -1 {
		username = username[sepIdx+1:]
	}

	parts := strings.Split(username, ".")
	if numParts := len(parts); numParts > 1 {
		var usernameBuilder strings.Builder
		for _, part := range parts[0 : numParts-1] {
			usernameBuilder.WriteByte(part[0])
		}
		usernameBuilder.WriteString(parts[numParts-1])
		username = usernameBuilder.String()
	}

	username = strings.ToLower(username)
	username = strings.ReplaceAll(username, " ", "-")

	return username
}

// AllowDevMode returns if DevMode is allowed
func (p localProfile) AllowDevMode() bool {
	return true
}
