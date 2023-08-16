// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux

package testutil

import (
	"sync"

	"github.com/DataDog/datadog-agent/pkg/network"
	"github.com/DataDog/datadog-agent/pkg/network/netlink"
)

type delayedConntracker struct {
	netlink.Conntracker

	mux          sync.Mutex
	numDelays    int
	delayPerConn map[network.ConnectionStatsByteKey]int
}

// NewDelayedConntracker returns a netlink.Conntracker that returns `nil` for `numDelays`
// consecutive times. After that lookups are routed to the actual Conntracker implementation.
func NewDelayedConntracker(ctr netlink.Conntracker, numDelays int) netlink.Conntracker {
	return &delayedConntracker{
		Conntracker:  ctr,
		numDelays:    numDelays,
		delayPerConn: make(map[network.ConnectionStatsByteKey]int),
	}
}

func (ctr *delayedConntracker) GetTranslationForConn(c network.ConnectionStats) *network.IPTranslation {
	ctr.mux.Lock()
	defer ctr.mux.Unlock()

	key := c.ByteKey()
	delays := ctr.delayPerConn[key]
	if delays < ctr.numDelays {
		ctr.delayPerConn[key]++
		return nil
	}

	return ctr.Conntracker.GetTranslationForConn(c)
}
