// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package local

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core"
	"github.com/DataDog/datadog-agent/comp/core/workloadmeta"
	"github.com/DataDog/datadog-agent/pkg/tagger/collectors"
	"github.com/DataDog/datadog-agent/pkg/tagset"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
)

func TestTagBuilder(t *testing.T) {

	store := fxutil.Test[workloadmeta.Mock](t, fx.Options(
		core.MockBundle(),
		fx.Supply(workloadmeta.NewParams()),
		workloadmeta.MockModuleV2(),
	))
	tagger := NewTagger(store)
	tagger.Init(context.Background())
	defer tagger.Stop()

	tagger.tagStore.ProcessTagInfo([]*collectors.TagInfo{
		{
			Entity:       "entity_name",
			Source:       "stream",
			LowCardTags:  []string{"low1"},
			HighCardTags: []string{"high"},
		},
		{
			Entity:      "entity_name",
			Source:      "pull",
			LowCardTags: []string{"low2"},
		},
	})

	tb := tagset.NewHashlessTagsAccumulator()
	err := tagger.AccumulateTagsFor("entity_name", collectors.HighCardinality, tb)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"high", "low1", "low2"}, tb.Get())
}
