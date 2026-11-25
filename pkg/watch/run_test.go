package watch

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSource returns a scripted snapshot per call and cancels ctx after `stopAfter` calls.
type fakeSource struct {
	mu        sync.Mutex
	calls     int
	stopAfter int
	cancel    context.CancelFunc
	snapshot  func(call int) Snapshot
}

func (f *fakeSource) Collect(_ context.Context, opts Options, now time.Time) Snapshot {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()

	snap := Snapshot{Time: now, Options: opts}
	if f.snapshot != nil {
		snap = f.snapshot(call)
		snap.Time, snap.Options = now, opts
	}
	if call == f.stopAfter {
		// Cancel after rendering this refresh: Run checks ctx after Collect,
		// so make the last call render by cancelling on the next tick instead.
		go func() { time.Sleep(5 * time.Millisecond); f.cancel() }()
	}
	return snap
}

func TestRun_RefreshesUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := &fakeSource{stopAfter: 3, cancel: cancel, snapshot: func(call int) Snapshot {
		if call == 2 {
			return Snapshot{Errors: []string{"nodes: temporary failure"}}
		}
		return Snapshot{TotalNodes: call}
	}}

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, src, Options{Nodes: true}, &out, RunConfig{
			Interval: 20 * time.Millisecond,
			Now:      func() time.Time { return refTime },
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "cancellation is a normal exit")
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}

	s := out.String()
	assert.Equal(t, 3, src.calls)
	assert.Equal(t, 3, strings.Count(s, "k8i watch | every 20ms"), "one screen per refresh")
	assert.Contains(t, s, "all 1 nodes are Ready")
	assert.Contains(t, s, "nodes: temporary failure", "errors are shown and the loop keeps running")
	assert.Contains(t, s, "all 3 nodes are Ready")
	assert.NotContains(t, s, clearScreen, "screen is not cleared when ClearScreen is false")
}

func TestRun_ClearsScreenAndRedetectsWidth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := &fakeSource{stopAfter: 2, cancel: cancel}
	widths := []int{40, 60}
	var widthCalls int

	var out bytes.Buffer
	err := Run(ctx, src, Options{Nodes: true}, &out, RunConfig{
		Interval:    10 * time.Millisecond,
		ClearScreen: true,
		Now:         func() time.Time { return refTime },
		Width: func() int {
			w := widths[min(widthCalls, len(widths)-1)]
			widthCalls++
			return w
		},
	})
	require.NoError(t, err)

	screens := strings.Split(out.String(), clearScreen)
	require.Len(t, screens, 3, "clear sequence precedes each of the 2 screens")
	assert.Contains(t, screens[1], "\n"+strings.Repeat("=", 40)+"\n")
	assert.Contains(t, screens[2], "\n"+strings.Repeat("=", 60)+"\n")
}

func TestRun_AlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := &fakeSource{}
	var out bytes.Buffer

	require.NoError(t, Run(ctx, src, Options{Nodes: true}, &out, RunConfig{Interval: time.Hour}))
	assert.LessOrEqual(t, src.calls, 1)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestRun_WriteErrorStops(t *testing.T) {
	err := Run(context.Background(), &fakeSource{}, Options{Nodes: true}, failingWriter{}, RunConfig{Interval: time.Hour})
	assert.EqualError(t, err, "broken pipe")
}

// hangingSource blocks until its ctx is done, like an unresponsive API server.
type hangingSource struct {
	mu    sync.Mutex
	calls int
}

func (h *hangingSource) Collect(ctx context.Context, opts Options, now time.Time) Snapshot {
	h.mu.Lock()
	h.calls++
	h.mu.Unlock()
	<-ctx.Done()
	return Snapshot{Time: now, Options: opts, Errors: []string{"nodes: " + ctx.Err().Error()}}
}

func TestRun_CollectTimeoutKeepsLoopAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := &hangingSource{}
	var (
		mu  sync.Mutex
		out bytes.Buffer
	)
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return out.Write(p)
	})

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, src, Options{Nodes: true}, w, RunConfig{
			Interval:       10 * time.Millisecond,
			CollectTimeout: 20 * time.Millisecond,
			Now:            func() time.Time { return refTime },
		})
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Count(out.String(), "context deadline exceeded") >= 2
	}, 5*time.Second, 5*time.Millisecond, "a hanging refresh must time out and the loop must continue")

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
