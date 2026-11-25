package cli

import (
	"bytes"
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

	for _, name := range []string{"nodes", "pods", "events"} {
		f := cmd.Flags().Lookup(name)
		require.NotNil(t, f, name)
		assert.Equal(t, "false", f.DefValue, "--%s must be disabled by default", name)
	}
	assert.Equal(t, "[]", cmd.Flags().Lookup("namespaces").DefValue, "all namespaces by default")
	assert.Equal(t, "5s", cmd.Flags().Lookup("interval").DefValue)
	assert.Equal(t, "15m0s", cmd.Flags().Lookup("events-since").DefValue)
	assert.NotNil(t, cmd.Flags().Lookup("context"))
	assert.NotNil(t, cmd.Flags().Lookup("color"))
}

func TestWatchCmd_NoSectionsIsError(t *testing.T) {
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"watch"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enable at least one of --nodes, --pods, --events")
}

func TestWatchCmd_NamespacesFlagParsing(t *testing.T) {
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"watch"})
	require.NoError(t, err)
	require.NoError(t, cmd.ParseFlags([]string{"--pods", "--namespaces", "default,prod", "--namespaces", "kube-system"}))

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
		}, "--namespaces can only be used together with --pods"},
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
