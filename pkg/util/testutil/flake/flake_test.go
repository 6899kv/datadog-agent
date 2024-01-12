// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.Datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package flake

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTesting struct {
	*testing.T

	skipCallCount int
	failCallCount int
}

func newMockTesting(t *testing.T) *mockTesting {
	return &mockTesting{
		T: t,
	}
}

func (mt *mockTesting) Skip(_ ...any) {
	mt.skipCallCount++
	mt.SkipNow()
}

func (mt *mockTesting) Fail() {
	mt.failCallCount++
}

var trueValue bool = true
var falseValue bool = false

func TestFlake(t *testing.T) {
	t.Run("allow flake test", func(t *testing.T) {
		mt := newMockTesting(t)
		// set allow-flake-failure flag to true
		allowFlakeFailure = &trueValue
		skipFlake = &falseValue
		wrapAndRunFlakyTest(mt)
		assert.True(t, mt.Skipped())
		assert.False(t, mt.Failed())
		assert.Greater(t, mt.skipCallCount, 1)
		assert.Equal(t, 0, mt.failCallCount)
	})
	t.Run("skip flake test", func(t *testing.T) {
		mt := newMockTesting(t)
		// set skip-flake flag to true
		allowFlakeFailure = &falseValue
		skipFlake = &trueValue
		wrapAndRunFlakyTest(mt)
		assert.Equal(t, 1, mt.skipCallCount)
		assert.Equal(t, 0, mt.failCallCount)
		assert.True(t, mt.Skipped())
		assert.False(t, mt.Failed())
	})
	t.Run("skip flake test when both allow and skip are set", func(t *testing.T) {
		mt := newMockTesting(t)
		// set allow-flake-failure and skip-flake flag to true
		allowFlakeFailure = &trueValue
		skipFlake = &trueValue
		wrapAndRunFlakyTest(mt)
		assert.Equal(t, 1, mt.skipCallCount)
		assert.Equal(t, 0, mt.failCallCount)
		assert.True(t, mt.Skipped())
		assert.False(t, mt.Failed())
	})
}

func wrapAndRunFlakyTest(t *mockTesting) {
	t.Helper()
	wg := sync.WaitGroup{}
	// testing.T.Skip() calls runtime.Goexit() which terminates the goroutine
	// run the test in a separate goroutine to avoid terminating the test
	wg.Add(1)
	go func() {
		defer wg.Done()
		ft := Wrap(t)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				flipCoin := func() string {
					if rand.Intn(2) == 1 {
						return "heads"
					}
					return "tails"
				}
				coin := flipCoin()
				assert.Equal(ft, "heads", coin)
			}()
		}
	}()
	wg.Wait()
}
