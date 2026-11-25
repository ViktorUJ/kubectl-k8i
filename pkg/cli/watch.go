package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/terminal"
	"github.com/kubectl-k8i/pkg/watch"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// watchFlags holds the raw flag values of the watch subcommand.
type watchFlags struct {
	context     string
	nodes       bool
	pods        bool
	events      bool
	namespaces  []string
	interval    time.Duration
	eventsSince time.Duration
	color       string
}

// newWatchCmd creates the "watch" subcommand.
func newWatchCmd() *cobra.Command {
	f := watchFlags{}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Continuously show not-ready nodes, not-ready pods and warning events",
		Long: `The watch command keeps running and refreshes the screen every --interval,
showing only what needs attention in the cluster. Press Ctrl+C to exit.

Sections (each is off by default, enable at least one):
  --nodes    nodes whose Ready condition is not True (NotReady / Unknown),
             with active pressure conditions and the reason/message
  --pods     pods that are neither Ready nor Completed (Pending, Failed,
             CrashLoopBackOff, ImagePullBackOff, Init:*, Running but not ready, ...).
             The namespace list is fetched first, then pods are listed per namespace.
             Use --namespaces to limit the namespaces (default: all namespaces).
  --events   Warning events from the whole cluster seen within --events-since

API errors do not stop the command: they are shown in the ERRORS section and
the data is requested again on the next refresh.`,
		Example: `  # Watch not-ready nodes
  kubectl k8i watch --nodes

  # Watch not-ready pods in all namespaces
  kubectl k8i watch --pods

  # Watch not-ready pods only in selected namespaces
  kubectl k8i watch --pods --namespaces default,production
  kubectl k8i watch --pods --namespaces default --namespaces production

  # Watch everything, refresh every 10 seconds
  kubectl k8i watch --nodes --pods --events --interval 10s

  # Warning events from the last hour
  kubectl k8i watch --events --events-since 1h`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := buildWatchOptions(f)
			if err != nil {
				return err
			}
			return runWatchCmd(cmd.Context(), f, opts)
		},
	}

	cmd.Flags().StringVar(&f.context, "context", "", "Kubernetes context to use")
	cmd.Flags().BoolVar(&f.nodes, "nodes", false, "Show nodes that are not Ready")
	cmd.Flags().BoolVar(&f.pods, "pods", false, "Show pods that are neither Ready nor Completed")
	cmd.Flags().BoolVar(&f.events, "events", false, "Show cluster Warning events")
	cmd.Flags().StringSliceVar(&f.namespaces, "namespaces", nil, "Namespaces to watch for --pods (comma-separated or repeatable; default: all)")
	cmd.Flags().DurationVar(&f.interval, "interval", watch.DefaultInterval, "Refresh interval (minimum 1s)")
	cmd.Flags().DurationVar(&f.eventsSince, "events-since", watch.DefaultEventsSince, "Show only Warning events seen within this window (0 = no limit)")
	cmd.Flags().StringVar(&f.color, "color", "auto", "Force enable or disable ANSI colors (true/false/auto)")

	_ = cmd.RegisterFlagCompletionFunc("context", completeContexts)
	_ = cmd.RegisterFlagCompletionFunc("namespaces", completeNamespaces)
	_ = cmd.RegisterFlagCompletionFunc("interval", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"2s", "5s", "10s", "30s", "1m"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("events-since", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"5m", "15m", "30m", "1h", "0"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("color", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"true", "false", "auto"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// buildWatchOptions validates the watch flags and converts them to watch.Options.
func buildWatchOptions(f watchFlags) (watch.Options, error) {
	if !f.nodes && !f.pods && !f.events {
		return watch.Options{}, fmt.Errorf("nothing to watch: enable at least one of --nodes, --pods, --events")
	}

	var namespaces []string
	for _, ns := range f.namespaces {
		if ns = strings.TrimSpace(ns); ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	if len(f.namespaces) > 0 && len(namespaces) == 0 {
		return watch.Options{}, fmt.Errorf("--namespaces must contain at least one namespace name")
	}
	if len(namespaces) > 0 && !f.pods {
		return watch.Options{}, fmt.Errorf("--namespaces can only be used together with --pods")
	}
	if f.interval < watch.MinInterval {
		return watch.Options{}, fmt.Errorf("--interval must be at least %s, got %s", watch.MinInterval, f.interval)
	}
	if f.eventsSince < 0 {
		return watch.Options{}, fmt.Errorf("--events-since must not be negative, got %s", f.eventsSince)
	}
	if err := validateColorFlag(f.color); err != nil {
		return watch.Options{}, err
	}

	return watch.Options{
		Nodes:       f.nodes,
		Pods:        f.pods,
		Events:      f.events,
		Namespaces:  namespaces,
		EventsSince: f.eventsSince,
	}, nil
}

// validateColorFlag checks the --color value.
func validateColorFlag(v string) error {
	switch strings.ToLower(v) {
	case "", "auto", "true", "false":
		return nil
	}
	return fmt.Errorf("invalid --color value %q: expected true, false or auto", v)
}

// parseColorFlag converts the --color value to the *bool used by color.NewColorConfig.
func parseColorFlag(v string) *bool {
	switch strings.ToLower(v) {
	case "true":
		t := true
		return &t
	case "false":
		f := false
		return &f
	}
	return nil
}

// runWatchCmd connects to the cluster and runs the refresh loop until ctx is cancelled.
func runWatchCmd(ctx context.Context, f watchFlags, opts watch.Options) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if f.context != "" {
		overrides.CurrentContext = f.context
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	contextName := f.context
	if contextName == "" {
		if raw, err := kubeConfig.RawConfig(); err == nil {
			contextName = raw.CurrentContext
		}
	}

	isTTY := terminal.IsTerminal(os.Stdout.Fd())
	collector := watch.NewCollector(clientset, watch.WatchRetryConfig())

	return watch.Run(ctx, collector, opts, os.Stdout, watch.RunConfig{
		Interval:    f.interval,
		ClearScreen: isTTY,
		Render: watch.RenderConfig{
			Color:   color.NewColorConfig(parseColorFlag(f.color)),
			Context: contextName,
		},
		Width: func() int { return terminal.GetTerminalWidth(0) },
	})
}
