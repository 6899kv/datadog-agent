// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux_bpf

package offsetguess

import (
	"fmt"
	"math"
	"time"

	"golang.org/x/sys/unix"

	manager "github.com/DataDog/ebpf-manager"

	"github.com/DataDog/datadog-agent/pkg/collector/corechecks/ebpf/probe/ebpfcheck"
	"github.com/DataDog/datadog-agent/pkg/ebpf/bytecode"
	"github.com/DataDog/datadog-agent/pkg/network/config"
	"github.com/DataDog/datadog-agent/pkg/network/ebpf/probes"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

var zero uint64

const (
	// The source port is much further away in the inet sock.
	thresholdInetSock = 2000

	notApplicable = 99999 // An arbitrary large number to indicate that the value should be ignored
)

type guessSubject int

const (
	structSock guessSubject = iota
	structSocket
	structFlowI4
	structFlowI6
	structSKBuff
	structNFConn
)

var stateString = map[GuessState]string{
	StateUninitialized: "uninitialized",
	StateChecking:      "checking",
	StateChecked:       "checked",
	StateReady:         "ready",
}

func (s GuessState) String() string {
	return stateString[s]
}

var whatString = map[GuessWhat]string{
	GuessSAddr:     "source address",
	GuessDAddr:     "destination address",
	GuessFamily:    "family",
	GuessSPort:     "source port",
	GuessDPort:     "destination port",
	GuessNetNS:     "network namespace",
	GuessRTT:       "round trip time",
	GuessDAddrIPv6: "destination address IPv6",

	// Guess offsets in struct flowi4
	GuessSAddrFl4: "source address flowi4",
	GuessDAddrFl4: "destination address flowi4",
	GuessSPortFl4: "source port flowi4",
	GuessDPortFl4: "destination port flowi4",

	// Guess offsets in struct flowi6
	GuessSAddrFl6: "source address flowi6",
	GuessDAddrFl6: "destination address flowi6",
	GuessSPortFl6: "source port flowi6",
	GuessDPortFl6: "destination port flowi6",

	GuessSocketSK:              "sk field on struct socket",
	GuessSKBuffSock:            "sk field on struct sk_buff",
	GuessSKBuffTransportHeader: "transport header field on struct sk_buff",
	GuessSKBuffHead:            "head field on struct sk_buff",

	GuessCtTupleOrigin: "conntrack origin tuple",
	GuessCtTupleReply:  "conntrack reply tuple",
	GuessCtStatus:      "conntrack guess",
	GuessCtNet:         "conntrack network namespace",
}

func (w GuessWhat) String() string {
	return whatString[w]
}

func (g *GuessStatus) setProcessName(name string) {
	if len(name) > ProcCommMaxLen { // Truncate process name if needed
		name = name[:ProcCommMaxLen]
	}
	copy(g.Proc.Comm[:], name)
}

type OffsetGuesser interface {
	Manager() *manager.Manager
	Probes(c *config.Config) (map[string]struct{}, error)
	Guess(c *config.Config) ([]manager.ConstantEditor, error)
	Close()
}

func idPair(name probes.ProbeFuncName) manager.ProbeIdentificationPair {
	return manager.ProbeIdentificationPair{
		EBPFFuncName: name,
		UID:          "offset",
	}
}

func enableProbe(enabled map[probes.ProbeFuncName]struct{}, name probes.ProbeFuncName) {
	enabled[name] = struct{}{}
}

func setupOffsetGuesser(guesser OffsetGuesser, config *config.Config, buf bytecode.AssetReader) error {
	// Enable kernel probes used for offset guessing.
	offsetMgr := guesser.Manager()
	offsetOptions := manager.Options{
		RLimit: &unix.Rlimit{
			Cur: math.MaxUint64,
			Max: math.MaxUint64,
		},
	}
	enabledProbes, err := guesser.Probes(config)
	if err != nil {
		return fmt.Errorf("unable to configure offset guessing probes: %w", err)
	}

	for _, p := range offsetMgr.Probes {
		if _, enabled := enabledProbes[p.EBPFFuncName]; !enabled {
			offsetOptions.ExcludedFunctions = append(offsetOptions.ExcludedFunctions, p.EBPFFuncName)
		}
	}
	for funcName := range enabledProbes {
		offsetOptions.ActivatedProbes = append(
			offsetOptions.ActivatedProbes,
			&manager.ProbeSelector{ProbeIdentificationPair: idPair(funcName)},
		)
	}
	if err := offsetMgr.InitWithOptions(buf, offsetOptions); err != nil {
		return fmt.Errorf("could not load bpf module for offset guessing: %s", err)
	}
	ebpfcheck.AddNameMappings(offsetMgr, "npm_offsetguess")
	if err := offsetMgr.Start(); err != nil {
		return fmt.Errorf("could not start offset ebpf manager: %s", err)
	}

	return nil
}

func RunOffsetGuessing(cfg *config.Config, buf bytecode.AssetReader, newGuesser func() (OffsetGuesser, error)) (editors []manager.ConstantEditor, err error) {
	// Offset guessing has been flaky for some customers, so if it fails we'll retry it up to 5 times
	start := time.Now()
	for i := 0; i < 5; i++ {
		err = func() error {
			guesser, err := newGuesser()
			if err != nil {
				return err
			}

			if err = setupOffsetGuesser(guesser, cfg, buf); err != nil {
				return err
			}

			editors, err = guesser.Guess(cfg)
			guesser.Close()
			return err
		}()

		if err == nil {
			log.Infof("offset guessing complete (took %v)", time.Since(start))
			return editors, nil
		}

		time.Sleep(1 * time.Second)
	}

	return nil, err
}
