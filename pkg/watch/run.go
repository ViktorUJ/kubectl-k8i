package watch

import (
	"bytes"
	"context"
	"io"
	"time"
)

// clearScreen moves the cursor home and clears the terminal.
const clearScreen = "\033[H\033[2J"

const (
	// DefaultInterval is the refresh interval used when none is given.
	DefaultInterval = 5 * time.Second
	// MinInterval is the smallest refresh interval accepted by the CLI.
	MinInterval = time.Second
	// DefaultEventsSince is the default window for warning events.
	DefaultEventsSince = 15 * time.Minute
	// DefaultCollectTimeout bounds a single refresh.
	DefaultCollectTimeout = 30 * time.Second
)

// Source produces watch snapshots. *Collector implements it.
type Source interface {
	Collect(ctx context.Context, opts Options, now time.Time) Snapshot
}

// RunConfig controls the refresh loop.
type RunConfig struct {
	Interval    time.Duration
	ClearScreen bool             // clear the terminal before each refresh (TTY only)
	Render      RenderConfig     // base render options
	Now         func() time.Time // defaults to time.Now
	Width       func() int       // re-detects terminal width on each refresh; nil keeps Render.TermWidth
	// CollectTimeout bounds a single refresh so a hanging API server cannot
	// freeze the screen. <=0 uses DefaultCollectTimeout.
	CollectTimeout time.Duration
}

// Run refreshes the watch view every Interval until ctx is cancelled.
// It never exits on API errors: they are shown in the view and retried on
// the next refresh. Returns nil when ctx is cancelled (e.g. Ctrl+C).
func Run(ctx context.Context, src Source, opts Options, w io.Writer, cfg RunConfig) error {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	collectTimeout := cfg.CollectTimeout
	if collectTimeout <= 0 {
		collectTimeout = DefaultCollectTimeout
	}
	rc := cfg.Render
	rc.Interval = interval

	timer := time.NewTimer(0)
	defer timer.Stop()

	for first := true; ; first = false {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}

		cctx, cancel := context.WithTimeout(ctx, collectTimeout)
		snap := src.Collect(cctx, opts, now())
		cancel()
		if ctx.Err() != nil {
			return nil
		}

		if cfg.Width != nil {
			rc.TermWidth = cfg.Width()
		}

		// Render into a buffer and write once to avoid flicker.
		var buf bytes.Buffer
		switch {
		case cfg.ClearScreen:
			buf.WriteString(clearScreen)
		case !first:
			buf.WriteString("\n")
		}
		RenderSnapshot(&buf, snap, rc)
		if _, err := w.Write(buf.Bytes()); err != nil {
			return err
		}

		timer.Reset(interval)
	}
}
