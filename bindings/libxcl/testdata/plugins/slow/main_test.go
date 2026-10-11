package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitReturnsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	wait(ctx, 60_000)
	elapsed := time.Since(start)

	require.Less(t, elapsed, 5*time.Second, "wait did not return on a cancelled context")
}

func TestWaitSleepsForDelay(t *testing.T) {
	start := time.Now()
	wait(context.Background(), 50)
	elapsed := time.Since(start)

	require.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}
