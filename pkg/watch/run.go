package watch

import (
	"bytes"
	"context"
	"io"
	"time"
)

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
	Interval time.Duration
	// Screen redraws the view in place on a terminal (only changed lines are
	// rewritten). When false, each refresh is appended to the output.
	Screen bool
	Render RenderConfig     // base render options
	Now    func() time.Time // defaults to time.Now
	// Size re-detects the terminal width and height on each refresh
	// (0 = unknown). nil keeps Render.TermWidth and no height limit.
	Size func() (width, height int)
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

	scr := newScreen(w)

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

		height := 0
		if cfg.Size != nil {
			rc.TermWidth, height = cfg.Size()
		}

		// Render into a buffer and write once to avoid flicker.
		var buf bytes.Buffer
		RenderSnapshot(&buf, snap, rc)

		if cfg.Screen {
			if err := scr.Draw(buf.String(), rc.TermWidth, height); err != nil {
				return err
			}
		} else {
			if !first {
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			if _, err := w.Write(buf.Bytes()); err != nil {
				return err
			}
		}

		timer.Reset(interval)
	}
}
