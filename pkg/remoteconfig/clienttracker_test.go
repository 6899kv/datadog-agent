// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2022-present Datadog, Inc.

package remoteconfig

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/proto/pbgo"
	"github.com/stretchr/testify/assert"
)

func TestClients(t *testing.T) {
	testTTL := time.Second * 5
	clients := NewClientTracker(testTTL)
	testClient1 := &pbgo.Client{
		Id: "client1",
	}
	testClient2 := &pbgo.Client{
		Id: "client2",
	}
	now := time.Now()
	clients.Seen(testClient1, now)
	timestamp := now.Add(time.Second * 4) // 4s
	clients.Seen(testClient2, timestamp)
	assert.ElementsMatch(t, []*pbgo.Client{testClient1, testClient2}, clients.ActiveClients(timestamp))
	timestamp = timestamp.Add(time.Second * 3) // 7s
	assert.ElementsMatch(t, []*pbgo.Client{testClient2}, clients.ActiveClients(timestamp))
	timestamp = timestamp.Add(time.Second*2 + time.Nanosecond*1) // 10s1ns
	assert.ElementsMatch(t, []*pbgo.Client{}, clients.ActiveClients(timestamp))
	assert.Empty(t, clients.clients)
}
