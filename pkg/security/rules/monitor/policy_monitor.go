//go:generate go run github.com/mailru/easyjson/easyjson -gen_build_flags=-mod=mod -no_std_marshalers $GOFILE

// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package monitor holds rules related files
package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/DataDog/datadog-go/v5/statsd"

	"github.com/DataDog/datadog-agent/comp/dogstatsd/constants"
	"github.com/DataDog/datadog-agent/pkg/security/events"
	"github.com/DataDog/datadog-agent/pkg/security/metrics"
	"github.com/DataDog/datadog-agent/pkg/security/secl/compiler/eval"
	"github.com/DataDog/datadog-agent/pkg/security/secl/model"
	"github.com/DataDog/datadog-agent/pkg/security/secl/rules"
	"github.com/DataDog/datadog-agent/pkg/security/utils"
	"github.com/DataDog/datadog-agent/pkg/tagger/collectors"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/version"
)

const (
	// policyMetricRate defines how often the policy metric will be sent
	policyMetricRate = 30 * time.Second
)

// Policy describes policy related information
type Policy struct {
	Name    string
	Source  string
	Version string
}

// RuleStatus defines status of rules
type RuleStatus = map[eval.RuleID]string

// PolicyMonitor defines a policy monitor
type PolicyMonitor struct {
	sync.RWMutex

	statsdClient         statsd.ClientInterface
	policies             map[string]Policy
	rules                RuleStatus
	perRuleMetricEnabled bool
}

// SetPolicies add policies to the monitor
func (p *PolicyMonitor) SetPolicies(policies []*rules.Policy, mErrs *multierror.Error) {
	p.Lock()
	defer p.Unlock()

	p.policies = map[string]Policy{}

	for _, policy := range policies {
		p.policies[policy.Name] = Policy{Name: policy.Name, Source: policy.Source, Version: policy.Version}

		for _, rule := range policy.Rules {
			p.rules[rule.ID] = "loaded"
		}

		if mErrs != nil && mErrs.Errors != nil {
			for _, err := range mErrs.Errors {
				if rerr, ok := err.(*rules.ErrRuleLoad); ok {
					p.rules[rerr.Definition.ID] = string(rerr.Type())
				}
			}
		}
	}
}

// ReportHeartbeatEvent sends HeartbeatEvents reporting the current set of policies
func (p *PolicyMonitor) ReportHeartbeatEvent(sender events.EventSender) {
	p.RLock()
	rule, events := NewHeartbeatEvents(p.policies)
	p.RUnlock()

	for _, event := range events {
		sender.SendEvent(rule, event, nil, "")
	}
}

// Start the monitor
func (p *PolicyMonitor) Start(ctx context.Context) {
	go func() {
		timerMetric := time.NewTicker(policyMetricRate)
		defer timerMetric.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-timerMetric.C:
				p.RLock()
				for _, policy := range p.policies {
					tags := []string{
						"policy_name:" + policy.Name,
						"policy_source:" + policy.Source,
						"policy_version:" + policy.Version,
						"agent_version:" + version.AgentVersion,
					}

					if err := p.statsdClient.Gauge(metrics.MetricPolicy, 1, tags, 1.0); err != nil {
						log.Error(fmt.Errorf("failed to send policy metric: %w", err))
					}
				}

				if p.perRuleMetricEnabled {
					for id, status := range p.rules {
						tags := []string{
							"rule_id:" + id,
							fmt.Sprintf("status:%v", status),
							constants.CardinalityTagPrefix + collectors.LowCardinalityString,
						}

						if err := p.statsdClient.Gauge(metrics.MetricRulesStatus, 1, tags, 1.0); err != nil {
							log.Error(fmt.Errorf("failed to send policy metric: %w", err))
						}
					}
				}
				p.RUnlock()
			}
		}
	}()
}

// NewPolicyMonitor returns a new Policy monitor
func NewPolicyMonitor(statsdClient statsd.ClientInterface, perRuleMetricEnabled bool) *PolicyMonitor {
	return &PolicyMonitor{
		statsdClient:         statsdClient,
		policies:             make(map[string]Policy),
		rules:                make(map[string]string),
		perRuleMetricEnabled: perRuleMetricEnabled,
	}
}

// RuleSetLoadedReport represents the rule and the custom event related to a RuleSetLoaded event, ready to be dispatched
type RuleSetLoadedReport struct {
	Rule  *rules.Rule
	Event *events.CustomEvent
}

// ReportRuleSetLoaded reports to Datadog that new ruleset was loaded
func ReportRuleSetLoaded(sender events.EventSender, statsdClient statsd.ClientInterface, policies []*PolicyState) {
	rule, event := NewRuleSetLoadedEvent(policies)

	if err := statsdClient.Count(metrics.MetricRuleSetLoaded, 1, []string{}, 1.0); err != nil {
		log.Error(fmt.Errorf("failed to send ruleset_loaded metric: %w", err))
	}

	sender.SendEvent(rule, event, nil, "")
}

// RuleState defines a loaded rule
// easyjson:json
type RuleState struct {
	ID         string            `json:"id"`
	Version    string            `json:"version,omitempty"`
	Expression string            `json:"expression"`
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// PolicyState is used to report policy was loaded
// easyjson:json
type PolicyState struct {
	Name    string       `json:"name"`
	Version string       `json:"version"`
	Source  string       `json:"source"`
	Rules   []*RuleState `json:"rules"`
}

// RulesetLoadedEvent is used to report that a new ruleset was loaded
// easyjson:json
type RulesetLoadedEvent struct {
	events.CustomEventCommonFields
	Policies []*PolicyState `json:"policies"`
}

// ToJSON marshal using json format
func (e RulesetLoadedEvent) ToJSON() ([]byte, error) {
	return utils.MarshalEasyJSON(e)
}

// HeartbeatEvent is used to report the policies that has been loaded
// easyjson:json
type HeartbeatEvent struct {
	events.CustomEventCommonFields
	Policy *PolicyState `json:"policy"`
}

// ToJSON marshal using json format
func (e HeartbeatEvent) ToJSON() ([]byte, error) {
	return utils.MarshalEasyJSON(e)
}

// PolicyStateFromRuleDefinition returns a policy state based on the rule definition
func PolicyStateFromRuleDefinition(def *rules.RuleDefinition) *PolicyState {
	return &PolicyState{
		Name:    def.Policy.Name,
		Version: def.Policy.Version,
		Source:  def.Policy.Source,
	}
}

// RuleStateFromDefinition returns a rule state based on the rule definition
func RuleStateFromDefinition(def *rules.RuleDefinition, status string, message string) *RuleState {
	return &RuleState{
		ID:         def.ID,
		Version:    def.Version,
		Expression: def.Expression,
		Status:     status,
		Message:    message,
		Tags:       def.Tags,
	}
}

// NewPoliciesState returns the states of policies and rules
func NewPoliciesState(ruleSets map[string]*rules.RuleSet, err *multierror.Error) []*PolicyState {
	mp := make(map[string]*PolicyState)

	var policyState *PolicyState
	var exists bool

	for _, rs := range ruleSets {
		for _, rule := range rs.GetRules() {
			ruleDef := rule.Definition
			policyName := ruleDef.Policy.Name

			if policyState, exists = mp[policyName]; !exists {
				policyState = PolicyStateFromRuleDefinition(ruleDef)
				mp[policyName] = policyState
			}
			policyState.Rules = append(policyState.Rules, RuleStateFromDefinition(ruleDef, "loaded", ""))
		}
	}

	// rules ignored due to errors
	if err != nil && err.Errors != nil {
		for _, err := range err.Errors {
			if rerr, ok := err.(*rules.ErrRuleLoad); ok {
				policyName := rerr.Definition.Policy.Name

				if _, exists := mp[policyName]; !exists {
					policyState = PolicyStateFromRuleDefinition(rerr.Definition)
					mp[policyName] = policyState
				} else {
					policyState = mp[policyName]
				}
				policyState.Rules = append(policyState.Rules, RuleStateFromDefinition(rerr.Definition, string(rerr.Type()), rerr.Err.Error()))
			}
		}
	}

	var policies []*PolicyState
	for _, policy := range mp {
		policies = append(policies, policy)
	}

	return policies
}

// NewRuleSetLoadedEvent returns the rule (e.g. ruleset_loaded) and a populated custom event for a new_rules_loaded event
func NewRuleSetLoadedEvent(policies []*PolicyState) (*rules.Rule, *events.CustomEvent) {
	evt := RulesetLoadedEvent{
		Policies: policies,
	}
	evt.FillCustomEventCommonFields()

	return events.NewCustomRule(events.RulesetLoadedRuleID, events.RulesetLoadedRuleDesc),
		events.NewCustomEvent(model.CustomRulesetLoadedEventType, evt)
}

// NewHeartbeatEvents returns the rule (e.g. heartbeat) and a populated custom event for a heartbeat event
func NewHeartbeatEvents(policies map[string]Policy) (*rules.Rule, []*events.CustomEvent) {
	var evts []*events.CustomEvent

	for _, policy := range policies {
		var policyState = PolicyState{
			Name:    policy.Name,
			Version: policy.Version,
			Source:  policy.Source,
			Rules:   nil, // The rules that have been loaded at startup are not reported in the heartbeat event
		}

		evt := HeartbeatEvent{
			Policy: &policyState,
		}
		evt.FillCustomEventCommonFields()
		evts = append(evts, events.NewCustomEvent(model.CustomHeartbeatEventType, evt))
	}

	return events.NewCustomRule(events.HeartbeatRuleID, events.HeartbeatRuleDesc),
		evts
}