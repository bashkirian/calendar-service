package main

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvery(t *testing.T) {
	// synctest runs on a fake clock: minutes pass instantly and deterministically.
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var runs atomic.Int32
		done := make(chan struct{})
		go func() {
			every(ctx, 5*time.Minute, func(context.Context) { runs.Add(1) })
			close(done)
		}()

		synctest.Wait()
		require.EqualValues(t, 1, runs.Load(), "runs at start, so frequent deploys cannot starve the job")

		time.Sleep(4 * time.Minute)
		synctest.Wait()
		require.EqualValues(t, 1, runs.Load(), "waits for the interval")

		time.Sleep(6 * time.Minute)
		synctest.Wait()
		require.EqualValues(t, 3, runs.Load())

		cancel()
		<-done
	})
}
