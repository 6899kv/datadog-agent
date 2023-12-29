// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package bootstrap implements 'updater bootstrap'.
package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/cmd/updater/command"
	"github.com/DataDog/datadog-agent/pkg/updater"

	"github.com/spf13/cobra"
)

// Commands returns the global params commands
func Commands(global *command.GlobalParams) []*cobra.Command {
	var defaultTimeout time.Duration
	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstraps the package with the first version.",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			return bootstrap(global.Package, defaultTimeout)
		},
	}
	bootstrapCmd.Flags().DurationVarP(&defaultTimeout, "default-timeout", "T", 3*time.Minute, "default timeout to bootstrap with")
	return []*cobra.Command{bootstrapCmd}
}

func bootstrap(pkg string, defaultTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	orgConfig, err := updater.NewOrgConfig()
	if err != nil {
		return fmt.Errorf("could not create org config: %w", err)
	}
	err = updater.Install(ctx, orgConfig, pkg)
	if err != nil {
		return fmt.Errorf("could not install package: %w", err)
	}
	return nil
}