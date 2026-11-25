package main

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
)

func TestSilenceClientGoLogs(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	silenceClientGoLogs()
	// The same kind of message client-go prints when its rate limiter waits.
	klog.InfoS("Waited before sending request", "delay", "1.7s", "reason", "client-side throttling, not priority and fairness")
	klog.Warning("some warning")
	klog.Flush()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Empty(t, string(out), "client-go logs must not reach stderr")
}
