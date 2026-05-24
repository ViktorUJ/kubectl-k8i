package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubectl-k8i/pkg/analyze"
	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/terminal"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1client "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

// newAnalyzeCmd creates the "analyze" subcommand.
func newAnalyzeCmd() *cobra.Command {
	var (
		contextFlag           string
		nodeFlag              string
		labelsFlag            string
		taintsFlag            string
		autoscalerFlag        string
		excludeNamespaceFlags []string
		outputFlag            string
		colorFlag             string
		debugFlag             bool
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze workloads running on selected nodes",
		Long: `Analyze shows all workloads (Deployments, StatefulSets, DaemonSets, standalone Pods)
running on the nodes selected by --node, --labels, --taints, or --autoscaler.

For each workload the command displays:
  - namespace, kind, name
  - number of running pods on the selected nodes
  - aggregated CPU requests / limits / usage (cores)
  - aggregated Memory requests / limits / usage (GB)

Results are sorted by namespace, then kind, then name.
Exactly one of --node, --labels, --taints, or --autoscaler must be provided.`,
		Example: `  # Analyze workloads on a specific node
  kubectl k8i analyze --node ip-10-0-1-100

  # Analyze workloads on nodes with a label selector
  kubectl k8i analyze --labels 'worker-type=spot'

  # Analyze workloads on nodes with a specific taint
  kubectl k8i analyze --taints 'dedicated=gpu'

  # Analyze workloads on all Karpenter-managed nodes
  kubectl k8i analyze --autoscaler karpenter

  # Analyze workloads on EKS nodegroup (CAS) nodes, exclude system namespaces
  kubectl k8i analyze --autoscaler cas --exclude-namespace kube-system

  # Exclude noisy system namespaces
  kubectl k8i analyze --labels 'worker-type=spot' \
    --exclude-namespace kube-system \
    --exclude-namespace monitoring

  # Output as JSON
  kubectl k8i analyze --node ip-10-0-1-100 -o json

  # Output as YAML
  kubectl k8i analyze --autoscaler spotio -o yaml`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate: exactly one selector must be set.
			setCount := 0
			if nodeFlag != "" {
				setCount++
			}
			if labelsFlag != "" {
				setCount++
			}
			if taintsFlag != "" {
				setCount++
			}
			if autoscalerFlag != "" {
				setCount++
			}
			if setCount == 0 {
				return fmt.Errorf("one of --node, --labels, --taints, or --autoscaler is required")
			}
			if setCount > 1 {
				return fmt.Errorf("only one of --node, --labels, --taints, or --autoscaler may be specified at a time")
			}

			if err := ValidateOutputFormat(outputFlag); err != nil {
				return err
			}

			cfg := model.AnalyzeConfig{
				Context:           contextFlag,
				NodeName:          nodeFlag,
				Labels:            labelsFlag,
				Taints:            taintsFlag,
				Autoscaler:        autoscalerFlag,
				ExcludeNamespaces: excludeNamespaceFlags,
				Output:            outputFlag,
				Debug:             debugFlag,
			}

			switch strings.ToLower(colorFlag) {
			case "true":
				t := true
				cfg.Color = &t
			case "false":
				f := false
				cfg.Color = &f
			default:
				cfg.Color = nil
			}

			return runAnalyzeCmd(cmd.Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&contextFlag, "context", "", "Kubernetes context to use")
	cmd.Flags().StringVar(&nodeFlag, "node", "", "Analyze workloads on this specific node")
	cmd.Flags().StringVar(&labelsFlag, "labels", "", "Analyze workloads on nodes matching this label selector")
	cmd.Flags().StringVar(&taintsFlag, "taints", "", "Analyze workloads on nodes with this taint (key or key=value)")
	cmd.Flags().StringVar(&autoscalerFlag, "autoscaler", "", "Analyze workloads on nodes managed by this autoscaler (karpenter, cas, spotio, x)")
	cmd.Flags().StringArrayVar(&excludeNamespaceFlags, "exclude-namespace", nil, "Exclude namespace from output (repeatable)")
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table, json, yaml")
	cmd.Flags().StringVar(&colorFlag, "color", "auto", "Force enable or disable ANSI colors (true/false/auto)")
	cmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable debug output to stderr")

	_ = cmd.RegisterFlagCompletionFunc("context", completeContexts)
	_ = cmd.RegisterFlagCompletionFunc("node", completeNodeNames)
	_ = cmd.RegisterFlagCompletionFunc("labels", completeLabelSelectors)
	_ = cmd.RegisterFlagCompletionFunc("taints", completeTaintKeys)
	_ = cmd.RegisterFlagCompletionFunc("autoscaler", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"karpenter", "cluster-autoscaler", "spotio", "x"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("exclude-namespace", completeNamespaces)
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("color", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"true", "false", "auto"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// runAnalyzeCmd executes the analyze pipeline.
func runAnalyzeCmd(ctx context.Context, cfg model.AnalyzeConfig) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if cfg.Context != "" {
		configOverrides.CurrentContext = cfg.Context
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Metrics client is non-fatal — usage shows as 0 if unavailable.
	var metricsClient metricsv1beta1client.MetricsV1beta1Interface
	if mc, err := metricsv1beta1client.NewForConfig(restConfig); err == nil {
		metricsClient = mc
	}

	c := analyze.NewCollector(clientset, metricsClient)

	workloads, err := c.CollectForNodes(ctx, cfg)
	if err != nil {
		return fmt.Errorf("analyze failed: %w", err)
	}

	switch cfg.Output {
	case "json":
		return analyze.WriteJSON(os.Stdout, workloads)
	case "yaml":
		return analyze.WriteYAML(os.Stdout, workloads)
	default:
		termWidth := terminal.GetTerminalWidth(120)
		cc := color.NewColorConfig(cfg.Color)
		analyze.RenderTable(os.Stdout, workloads, analyze.RenderConfig{
			Color:     cc,
			TermWidth: termWidth,
		})
		return nil
	}
}

// completeNodeNames returns available node names for tab completion.
func completeNodeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clientset, err := getClientsetForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	nodeList, err := clientset.CoreV1().Nodes().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	results := make([]string, 0, len(nodeList.Items))
	for _, n := range nodeList.Items {
		results = append(results, n.Name)
	}
	return results, cobra.ShellCompDirectiveNoFileComp
}
