package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/watch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validWatchFlags() watchFlags {
	return watchFlags{interval: watch.DefaultInterval, eventsSince: watch.DefaultEventsSince, color: "auto"}
}

func TestWatchCmd_RegisteredWithDefaults(t *testing.T) {
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"watch"})
	require.NoError(t, err)
	require.Equal(t, "watch", cmd.Name())

	for name, def := range map[string]string{"nodes": "enable", "pods": "enable", "events": "disable"} {
		f := cmd.Flags().Lookup(name)
		require.NotNil(t, f, name)
		assert.Equal(t, def, f.DefValue, "--%s default", name)
		assert.Equal(t, "enable|disable", f.Value.Type(), name)
		assert.Equal(t, "enable", f.NoOptDefVal, "bare --%s means enable", name)
	}
	assert.Equal(t, "[]", cmd.Flags().Lookup("namespaces").DefValue, "all namespaces by default")
	assert.Equal(t, "5s", cmd.Flags().Lookup("interval").DefValue)
	assert.Equal(t, "15m0s", cmd.Flags().Lookup("events-since").DefValue)
	assert.NotNil(t, cmd.Flags().Lookup("context"))
	assert.NotNil(t, cmd.Flags().Lookup("color"))
}

// parseWatch parses args with the real watch command and returns the
// resulting options (or the parse/validation error).
func parseWatch(t *testing.T, args ...string) (watch.Options, error) {
	t.Helper()
	root := NewRootCommand()

	var got watch.Options
	orig := runWatch
	t.Cleanup(func() { runWatch = orig })
	runWatch = func(_ context.Context, _ watchFlags, opts watch.Options) error {
		got = opts
		return nil
	}

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"watch"}, args...))
	err := root.Execute()
	return got, err
}

func TestWatchCmd_DefaultsShowNodesAndPods(t *testing.T) {
	opts, err := parseWatch(t)
	require.NoError(t, err, "plain 'k8i watch' must work without flags")
	assert.True(t, opts.Nodes)
	assert.True(t, opts.Pods)
	assert.False(t, opts.Events)
	assert.Empty(t, opts.Namespaces)
}

func TestWatchCmd_ToggleValues(t *testing.T) {
	tests := []struct {
		args                []string
		nodes, pods, events bool
	}{
		{[]string{"--nodes=disable"}, false, true, false},
		{[]string{"--pods=disable"}, true, false, false},
		{[]string{"--events=enable"}, true, true, true},
		{[]string{"--events"}, true, true, true},
		{[]string{"--nodes=enable", "--pods=enable"}, true, true, false},
		{[]string{"--nodes=false", "--pods=off", "--events=yes"}, false, false, true},
		{[]string{"--nodes=DISABLE"}, false, true, false},
		{[]string{"--nodes=disabled", "--events=enabled"}, false, true, true},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			opts, err := parseWatch(t, tt.args...)
			require.NoError(t, err)
			assert.Equal(t, tt.nodes, opts.Nodes, "nodes")
			assert.Equal(t, tt.pods, opts.Pods, "pods")
			assert.Equal(t, tt.events, opts.Events, "events")
		})
	}
}

func TestWatchCmd_ToggleErrors(t *testing.T) {
	_, err := parseWatch(t, "--nodes=maybe")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid value "maybe": expected enable or disable`)

	_, err = parseWatch(t, "--nodes=disable", "--pods=disable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all sections are disabled")

	_, err = parseWatch(t, "--nodes", "disable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pass the value with '=', e.g. --nodes=disable or --pods=disable or --events=disable")

	_, err = parseWatch(t, "something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown argument "something"`)

	_, err = parseWatch(t, "--pods=disable", "--namespaces", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--namespaces has no effect with --pods=disable")
}

func TestToggleValue(t *testing.T) {
	var b bool
	v := newToggleValue(true, &b)
	assert.True(t, b)
	assert.Equal(t, "enable", v.String())
	require.NoError(t, v.Set("disable"))
	assert.False(t, b)
	assert.Equal(t, "disable", v.String())
	require.NoError(t, v.Set(" On "))
	assert.True(t, b)
	assert.Error(t, v.Set(""))
}

func TestWatchCmd_NamespacesFlagParsing(t *testing.T) {
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"watch"})
	require.NoError(t, err)
	require.NoError(t, cmd.ParseFlags([]string{"--namespaces", "default,prod", "--namespaces", "kube-system"}))

	got, err := cmd.Flags().GetStringSlice("namespaces")
	require.NoError(t, err)
	assert.Equal(t, []string{"default", "prod", "kube-system"}, got)
}

func TestBuildWatchOptions_Valid(t *testing.T) {
	f := validWatchFlags()
	f.nodes, f.pods, f.events = true, true, true
	f.namespaces = []string{" default ", "prod"}
	f.eventsSince = time.Hour

	opts, err := buildWatchOptions(f)
	require.NoError(t, err)
	assert.Equal(t, watch.Options{
		Nodes: true, Pods: true, Events: true,
		Namespaces:  []string{"default", "prod"},
		EventsSince: time.Hour,
	}, opts)
}

func TestBuildWatchOptions_SingleSections(t *testing.T) {
	for _, set := range []func(*watchFlags){
		func(f *watchFlags) { f.nodes = true },
		func(f *watchFlags) { f.pods = true },
		func(f *watchFlags) { f.events = true },
	} {
		f := validWatchFlags()
		set(&f)
		_, err := buildWatchOptions(f)
		assert.NoError(t, err)
	}
}

func TestBuildWatchOptions_Errors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(f *watchFlags)
		errSub string
	}{
		{"nothing enabled", func(f *watchFlags) {}, "nothing to watch"},
		{"namespaces without pods", func(f *watchFlags) {
			f.nodes = true
			f.namespaces = []string{"default"}
		}, "--namespaces has no effect with --pods=disable"},
		{"empty namespaces", func(f *watchFlags) {
			f.pods = true
			f.namespaces = []string{" ", ""}
		}, "--namespaces must contain at least one namespace"},
		{"interval too small", func(f *watchFlags) {
			f.nodes = true
			f.interval = 500 * time.Millisecond
		}, "--interval must be at least 1s"},
		{"negative events window", func(f *watchFlags) {
			f.events = true
			f.eventsSince = -time.Minute
		}, "--events-since must not be negative"},
		{"bad color", func(f *watchFlags) {
			f.nodes = true
			f.color = "maybe"
		}, "invalid --color value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validWatchFlags()
			tt.mutate(&f)
			_, err := buildWatchOptions(f)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSub)
		})
	}
}

func TestParseColorFlag(t *testing.T) {
	assert.Nil(t, parseColorFlag("auto"))
	assert.Nil(t, parseColorFlag(""))
	require.NotNil(t, parseColorFlag("TRUE"))
	assert.True(t, *parseColorFlag("true"))
	assert.False(t, *parseColorFlag("false"))
}
