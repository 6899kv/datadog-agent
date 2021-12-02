// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package pipeline

import (
	"context"

	"github.com/DataDog/datadog-agent/pkg/logs/client"
	"github.com/DataDog/datadog-agent/pkg/logs/client/http"
	"github.com/DataDog/datadog-agent/pkg/logs/client/tcp"
	"github.com/DataDog/datadog-agent/pkg/logs/config"
	"github.com/DataDog/datadog-agent/pkg/logs/diagnostic"
	"github.com/DataDog/datadog-agent/pkg/logs/message"
	"github.com/DataDog/datadog-agent/pkg/logs/processor"
	"github.com/DataDog/datadog-agent/pkg/logs/sender"
)

// Pipeline processes and sends messages to the backend
type Pipeline struct {
	InputChan chan *message.Message
	processor *processor.Processor
	sender    *sender.Sender
}

// NewPipeline returns a new Pipeline
func NewPipeline(outputChan chan *message.Payload, processingRules []*config.ProcessingRule, endpoints *config.Endpoints, destinationsContext *client.DestinationsContext, diagnosticMessageReceiver diagnostic.MessageReceiver, serverless bool, pipelineID int) *Pipeline {
	mainDestinations := getDestinations(endpoints, destinationsContext, pipelineID)

	strategyInput := make(chan *message.Message, config.ChanSize)
	senderInput := make(chan *message.Payload, config.ChanSize)

	var logsSender *sender.Sender

	strategy := getStrategy(endpoints, serverless, pipelineID)
	strategy.Start(strategyInput, senderInput)
	logsSender = sender.NewSender(senderInput, outputChan, mainDestinations, config.ChanSize)

	var encoder processor.Encoder
	if serverless {
		encoder = processor.JSONServerlessEncoder
	} else if endpoints.UseHTTP {
		encoder = processor.JSONEncoder
	} else if endpoints.UseProto {
		encoder = processor.ProtoEncoder
	} else {
		encoder = processor.RawEncoder
	}

	inputChan := make(chan *message.Message, config.ChanSize)
	processor := processor.New(inputChan, strategyInput, processingRules, encoder, diagnosticMessageReceiver)

	return &Pipeline{
		InputChan: inputChan,
		processor: processor,
		sender:    logsSender,
	}
}

// Start launches the pipeline
func (p *Pipeline) Start() {
	p.sender.Start()
	p.processor.Start()
}

// Stop stops the pipeline
func (p *Pipeline) Stop() {
	p.processor.Stop()
	p.sender.Stop()
}

// Flush flushes synchronously the processor and sender managed by this pipeline.
func (p *Pipeline) Flush(ctx context.Context) {
	p.processor.Flush(ctx) // flush messages in the processor into the sender
}

func getDestinations(endpoints *config.Endpoints, destinationsContext *client.DestinationsContext, pipelineID int) *client.Destinations {
	reliable := []client.Destination{}
	additionals := []client.Destination{}

	if endpoints.UseHTTP {
		for _, endpoint := range endpoints.GetReliableEndpoints() {
			reliable = append(reliable, http.NewDestination(endpoint, http.JSONContentType, destinationsContext, endpoints.BatchMaxConcurrentSend, true, pipelineID))
		}
		for _, endpoint := range endpoints.GetUnReliableEndpoints() {
			additionals = append(additionals, http.NewDestination(endpoint, http.JSONContentType, destinationsContext, endpoints.BatchMaxConcurrentSend, false, pipelineID))
		}
		return client.NewDestinations(reliable, additionals)
	}
	for _, endpoint := range endpoints.GetReliableEndpoints() {
		reliable = append(reliable, tcp.NewDestination(endpoint, endpoints.UseProto, destinationsContext))
	}
	for _, endpoint := range endpoints.GetUnReliableEndpoints() {
		additionals = append(additionals, tcp.NewDestination(endpoint, endpoints.UseProto, destinationsContext))
	}
	return client.NewDestinations(reliable, additionals)
}

func getStrategy(endpoints *config.Endpoints, serverless bool, pipelineID int) sender.Strategy {
	if endpoints.UseHTTP || serverless {
		var encoder sender.ContentEncoding
		if endpoints.Main.UseCompression {
			encoder = sender.NewGzipContentEncoding(endpoints.Main.CompressionLevel)
		} else {
			encoder = sender.IdentityContentType
		}
		return sender.NewBatchStrategy(sender.ArraySerializer, endpoints.BatchWait, endpoints.BatchMaxSize, endpoints.BatchMaxContentSize, "logs", encoder)
	}
	return sender.StreamStrategy
}
