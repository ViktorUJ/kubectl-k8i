package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubectl-k8i/pkg/collector"
	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/debug"
	"github.com/kubectl-k8i/pkg/filter"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/output"
	"github.com/kubectl-k8i/pkg/render"
	"github.com/kubectl-k8i/pkg/retry"
	sortpkg "github.com/kubectl-k8i/pkg/sort"
	"github.com/kubectl-k8i/pkg/terminal"
	"github.com/kubectl-k8i/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1client "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

// NewRootCommand creates the cobra command with all flags for kubectl-k8i.
func NewRootCommand() *cobra.Command {
	var (
		contextFlag    string
		labelsFlag     string
		taintsFlag     string
		filterFlag     string
		sortFlag       string
		fargateFlag    bool
		colorFlag      string
		debugFlag      bool
		groupByFlag    string
		outputFlag     string
		noHeaders      bool
		deploymentFlag  string
		statefulSetFlag string
		namespaceFlag   string
	)

	cmd := &cobra.Command{
		Version: version.Version,
		Use:     "kubectl-k8i [flags]",
		Short:   "Display Kubernetes node resource information",
		Long: `kubectl-k8i displays detailed Kubernetes node resource information in a tabular
format with color-coded load percentages, node metadata, taints, and multiple
output formats (table, JSON, YAML).

Displays detailed information about Kubernetes nodes including:
  - Pod usage (current/max)
  - CPU resources (requests/limits/usage/capacity)
  - Memory resources (requests/limits/usage/capacity)
  - Load percentages with color coding
  - Node metadata (instance type, capacity type, zone, etc.)

Filter format:
  --filter 'attribute=value' where attribute can be:
    ec2_type       spot, od, x
    instance_type  m5.large, c5.xlarge, etc.
    arch           amd64, arm64
    zone           1a, 1b, etc. (last 2 characters)
    pool           nodepool name
    nodeclaim      Karpenter nodeclaim name
    taint          taint key or key=value
    autoscaler     karpenter, cas, spotio, x

Sort format:
  --sort 'column=direction' where:
    column:    name, pods, cpu_req, cpu_lim, cpu_use, cpu_cap, cpu_load,
               mem_req, mem_lim, mem_use, mem_cap, mem_load,
               ec2_type, instance_type, arch, zone, pool, age, taint, autoscaler
    direction: asc (ascending) or desc (descending)

Order of operations: labels → taints → filter → sort
  - Labels filter is applied at the Kubernetes API level during node selection
  - Taints filter is applied after collection, showing only nodes with matching taints
  - Filter is applied to the output data after collection
  - Sort is applied last to arrange the final results

Shell completion:
  To enable tab-completion for all flags, generate and install the completion script:
    kubectl k8i completion > kubectl_complete-k8i
    chmod +x kubectl_complete-k8i
    sudo mv kubectl_complete-k8i /usr/local/bin/
  After that, "kubectl k8i --<TAB>" will show available flags.`,
		Example: `  # Show all nodes (Fargate hidden by default)
  kubectl k8i

  # Show all nodes including Fargate
  kubectl k8i --fargate

  # Use a specific kube context
  kubectl k8i --context my-cluster

  # Show only spot nodes (API-level label filter)
  kubectl k8i --labels 'worker-type=spot'

  # Show only spot instances in output
  kubectl k8i --filter 'ec2_type=spot'

  # Show only on-demand instances in output
  kubectl k8i --filter 'ec2_type=od'

  # Sort by CPU load descending
  kubectl k8i --sort 'cpu_load=desc'

  # Sort by memory usage ascending
  kubectl k8i --sort 'mem_use=asc'

  # Sort by pod count descending
  kubectl k8i --sort 'pods=desc'

  # Sort by node age (youngest first)
  kubectl k8i --sort 'age=asc'

  # Output as JSON
  kubectl k8i -o json

  # Output as YAML
  kubectl k8i -o yaml

  # Group nodes by taint
  kubectl k8i --group-by taint

  # Suppress headers and annotations
  kubectl k8i --no-headers

  # Show only nodes with a specific taint key
  kubectl k8i --taints 'dedicated'

  # Show only nodes with a specific taint key=value
  kubectl k8i --taints 'dedicated=gpu'

  # Combined: filter by label, then by capacity type, then sort
  kubectl k8i --labels 'work_type=default' --filter 'ec2_type=spot' --sort 'mem_load=asc'

  # Install shell completion (tab-completion for flags)
  kubectl k8i completion > kubectl_complete-k8i
  chmod +x kubectl_complete-k8i
  sudo mv kubectl_complete-k8i /usr/local/bin/`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build RunConfig from flags.
			cfg := model.RunConfig{
				Context:     contextFlag,
				Labels:      labelsFlag,
				Taints:      taintsFlag,
				Filter:      filterFlag,
				Sort:        sortFlag,
				Fargate:     fargateFlag,
				Debug:       debugFlag,
				GroupBy:     groupByFlag,
				Output:      outputFlag,
				NoHeaders:   noHeaders,
				Deployment:  deploymentFlag,
				StatefulSet: statefulSetFlag,
				Namespace:   namespaceFlag,
			}

			// Handle --color flag: "true" → &true, "false" → &false, "auto"/empty → nil.
			switch strings.ToLower(colorFlag) {
			case "true":
				t := true
				cfg.Color = &t
			case "false":
				f := false
				cfg.Color = &f
			default:
				// "auto" or empty → nil (auto-detect)
				cfg.Color = nil
			}

			return runCommand(cmd.Context(), cfg)
		},
	}

	// Define flags.
	cmd.Flags().StringVar(&contextFlag, "context", "", "Kubernetes context to use")
	cmd.Flags().StringVar(&labelsFlag, "labels", "", "Kubernetes label selector to filter nodes at the API level")
	cmd.Flags().StringVar(&taintsFlag, "taints", "", "Filter nodes by taint (format: key or key=value)")
	cmd.Flags().StringVar(&filterFlag, "filter", "", "Filter output by node attribute (format: attribute=value)")
	cmd.Flags().StringVar(&sortFlag, "sort", "pool=asc", "Sort output by column and direction (format: column=direction)")
	cmd.Flags().BoolVar(&fargateFlag, "fargate", false, "Include Fargate nodes in the output")
	cmd.Flags().StringVar(&colorFlag, "color", "auto", "Force enable or disable ANSI colors (true/false/auto)")
	cmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable debug output to stderr")
	cmd.Flags().StringVar(&groupByFlag, "group-by", "", "Group nodes by attribute (currently only 'taint')")
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table, json, yaml")
	cmd.Flags().BoolVar(&noHeaders, "no-headers", false, "Suppress table header, separator, timestamp, and annotations")
	cmd.Flags().StringVar(&deploymentFlag, "deployment", "", "Show only nodes running pods of this deployment (format: namespace/name)")
	cmd.Flags().StringVar(&statefulSetFlag, "statefulset", "", "Show only nodes running pods of this statefulset (format: namespace/name)")
	cmd.Flags().StringVar(&namespaceFlag, "namespace", "", "Show only nodes running pods from this namespace")

	// Register dynamic completion functions for flags.
	_ = cmd.RegisterFlagCompletionFunc("labels", completeLabelSelectors)
	_ = cmd.RegisterFlagCompletionFunc("taints", completeTaintKeys)
	_ = cmd.RegisterFlagCompletionFunc("context", completeContexts)
	_ = cmd.RegisterFlagCompletionFunc("filter", completeFilterValues)
	_ = cmd.RegisterFlagCompletionFunc("sort", completeSortValues)
	_ = cmd.RegisterFlagCompletionFunc("deployment", completeDeployments)
	_ = cmd.RegisterFlagCompletionFunc("statefulset", completeStatefulSets)
	_ = cmd.RegisterFlagCompletionFunc("namespace", completeNamespaces)
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("color", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"true", "false", "auto"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("group-by", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"taint"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Add completion subcommand.
	cmd.AddCommand(newCompletionCmd())

	return cmd
}

// ParseDeployment validates and parses a deployment string in "namespace/name" format.
// Returns (namespace, name, error).
func ParseDeployment(deploymentStr string) (string, string, error) {
	idx := strings.Index(deploymentStr, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid deployment format %q: expected namespace/name", deploymentStr)
	}
	namespace := deploymentStr[:idx]
	name := deploymentStr[idx+1:]
	if namespace == "" {
		return "", "", fmt.Errorf("invalid deployment format %q: namespace cannot be empty", deploymentStr)
	}
	if name == "" {
		return "", "", fmt.Errorf("invalid deployment format %q: name cannot be empty", deploymentStr)
	}
	return namespace, name, nil
}

// ParseFilter validates and parses a filter string in "attribute=value" format.
// Returns (attribute, value, error).
func ParseFilter(filterStr string) (string, string, error) {
	idx := strings.Index(filterStr, "=")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid filter format %q: expected attribute=value", filterStr)
	}

	attribute := filterStr[:idx]
	value := filterStr[idx+1:]

	if attribute == "" {
		return "", "", fmt.Errorf("invalid filter format %q: attribute cannot be empty", filterStr)
	}
	if value == "" {
		return "", "", fmt.Errorf("invalid filter format %q: value cannot be empty", filterStr)
	}

	// Validate attribute is supported.
	supported := false
	for _, a := range filter.SupportedFilterAttributes {
		if a == attribute {
			supported = true
			break
		}
	}
	if !supported {
		return "", "", fmt.Errorf("unsupported filter attribute %q; supported attributes: %s",
			attribute, strings.Join(filter.SupportedFilterAttributes, ", "))
	}

	return attribute, value, nil
}

// ParseSort validates and parses a sort string in "column=direction" format.
// Returns (column, direction, error).
func ParseSort(sortStr string) (string, string, error) {
	idx := strings.Index(sortStr, "=")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid sort format %q: expected column=direction", sortStr)
	}

	column := sortStr[:idx]
	direction := sortStr[idx+1:]

	if column == "" {
		return "", "", fmt.Errorf("invalid sort format %q: column cannot be empty", sortStr)
	}
	if direction == "" {
		return "", "", fmt.Errorf("invalid sort format %q: direction cannot be empty", sortStr)
	}

	// Validate column is supported.
	supported := false
	for _, c := range sortpkg.SupportedSortColumns {
		if c == column {
			supported = true
			break
		}
	}
	if !supported {
		return "", "", fmt.Errorf("unsupported sort column %q; supported columns: %s",
			column, strings.Join(sortpkg.SupportedSortColumns, ", "))
	}

	// Validate direction.
	if direction != "asc" && direction != "desc" {
		return "", "", fmt.Errorf("invalid sort direction %q; supported directions: asc, desc", direction)
	}

	return column, direction, nil
}

// ValidateOutputFormat validates the output format string.
// Returns an error if format is not one of: table, json, yaml.
func ValidateOutputFormat(format string) error {
	switch format {
	case "table", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; supported formats: table, json, yaml", format)
	}
}

// runCommand executes the main pipeline: collect → enrich → filter → sort → group → output.
func runCommand(ctx context.Context, cfg model.RunConfig) error {
	// Validate output format.
	if err := ValidateOutputFormat(cfg.Output); err != nil {
		return err
	}

	// Validate filter if provided.
	var filterAttr, filterVal string
	if cfg.Filter != "" {
		var err error
		filterAttr, filterVal, err = ParseFilter(cfg.Filter)
		if err != nil {
			return err
		}
	}

	// Validate sort.
	sortCol, sortDir, err := ParseSort(cfg.Sort)
	if err != nil {
		return err
	}

	// Initialize debug logger.
	debugLogger := debug.NewDebugLogger(cfg.Debug)

	// Build kubeconfig.
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

	// Create Kubernetes clients.
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	metricsClient, err := metricsv1beta1client.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Build retry config with debug logger.
	retryConfig := retry.DefaultRetryConfig()
	retryConfig.DebugLogger = debugLogger

	// Progress reporter is nil — no progress output to stderr.
	var progressReporter func(step, total int, message string)

	// Create collector.
	c := collector.NewCollector(
		clientset,
		metricsClient,
		dynamicClient,
		cfg.Labels,
		progressReporter,
		retryConfig,
		debugLogger,
	)

	// Collect data.
	data, err := c.Collect(ctx)
	if err != nil {
		return fmt.Errorf("data collection failed: %w", err)
	}

	// Clear progress line (no-op since progress is disabled).
	_ = progressReporter

	// Enrich nodes.
	now := time.Now()
	nodes := c.EnrichNodes(data, now)

	debugLogger.LogDataProcessing("enriched_nodes", len(nodes))

	// Filter nodes by deployment if --deployment is set.
	if cfg.Deployment != "" {
		deplNamespace, deplName, err := ParseDeployment(cfg.Deployment)
		if err != nil {
			return err
		}
		nodeNames, err := c.NodeNamesForDeployment(ctx, deplNamespace, deplName)
		if err != nil {
			return fmt.Errorf("deployment filter failed: %w", err)
		}
		inputCount := len(nodes)
		filtered := nodes[:0]
		for _, n := range nodes {
			if _, ok := nodeNames[n.Name]; ok {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
		debugLogger.LogFilterSort("deployment", cfg.Deployment, inputCount, len(nodes))
	}

	// Filter nodes by statefulset if --statefulset is set.
	if cfg.StatefulSet != "" {
		stsNamespace, stsName, err := ParseDeployment(cfg.StatefulSet)
		if err != nil {
			return fmt.Errorf("invalid --statefulset value: %w", err)
		}
		nodeNames, err := c.NodeNamesForStatefulSet(ctx, stsNamespace, stsName)
		if err != nil {
			return fmt.Errorf("statefulset filter failed: %w", err)
		}
		inputCount := len(nodes)
		filtered := nodes[:0]
		for _, n := range nodes {
			if _, ok := nodeNames[n.Name]; ok {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
		debugLogger.LogFilterSort("statefulset", cfg.StatefulSet, inputCount, len(nodes))
	}

	// Filter nodes by namespace if --namespace is set.
	if cfg.Namespace != "" {
		nodeNames, err := c.NodeNamesForNamespace(ctx, cfg.Namespace)
		if err != nil {
			return fmt.Errorf("namespace filter failed: %w", err)
		}
		inputCount := len(nodes)
		filtered := nodes[:0]
		for _, n := range nodes {
			if _, ok := nodeNames[n.Name]; ok {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
		debugLogger.LogFilterSort("namespace", cfg.Namespace, inputCount, len(nodes))
	}

	// Hide Fargate nodes unless --fargate is set.
	if !cfg.Fargate {
		nodes = filter.HideFargateNodes(nodes)
		debugLogger.LogDataProcessing("after_fargate_filter", len(nodes))
	}

	// Apply taints filter if provided.
	if cfg.Taints != "" {
		inputCount := len(nodes)
		nodes = filter.FilterByTaints(nodes, cfg.Taints)
		debugLogger.LogFilterSort("taints", cfg.Taints, inputCount, len(nodes))
	}

	// Apply filter if provided.
	inputCount := len(nodes)
	if cfg.Filter != "" {
		nodes, err = filter.FilterNodes(nodes, filterAttr, filterVal)
		if err != nil {
			return fmt.Errorf("filter failed: %w", err)
		}
		debugLogger.LogFilterSort("filter", cfg.Filter, inputCount, len(nodes))
	}

	// Apply sort.
	inputCount = len(nodes)
	if err := sortpkg.SortNodes(nodes, sortCol, sortDir); err != nil {
		return fmt.Errorf("sort failed: %w", err)
	}
	debugLogger.LogFilterSort("sort", cfg.Sort, inputCount, len(nodes))

	// Output based on format.
	debugLogger.LogOutputFormat(cfg.Output)

	switch cfg.Output {
	case "table":
		// Detect terminal width.
		termWidth := terminal.GetTerminalWidth(200)
		debugLogger.LogTerminalWidth(termWidth, termWidth != 200)

		// Build color config.
		cc := color.NewColorConfig(cfg.Color)

		// Build render config.
		renderCfg := render.RenderConfig{
			Color:        cc,
			Filter:       cfg.Filter,
			Sort:         cfg.Sort,
			GroupByTaint: cfg.GroupBy == "taint",
			Timestamp:    now,
			TermWidth:    termWidth,
			NoHeaders:    cfg.NoHeaders,
		}

		render.RenderTable(os.Stdout, nodes, renderCfg)

	case "json", "yaml":
		formatter, err := output.NewFormatter(output.OutputFormat(cfg.Output))
		if err != nil {
			return fmt.Errorf("failed to create formatter: %w", err)
		}
		if err := formatter.Format(os.Stdout, nodes); err != nil {
			return fmt.Errorf("output formatting failed: %w", err)
		}
	}

	return nil
}

// completionScript is the kubectl plugin completion hook script template.
const completionScript = `#!/usr/bin/env sh
# kubectl plugin completion hook for k8i.
# Place this file as "kubectl_complete-k8i" in your $PATH.
kubectl-k8i __complete "$@"
`

// newCompletionCmd creates the "completion" subcommand that outputs the
// kubectl_complete-k8i script to stdout.
func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate kubectl plugin completion script",
		Long: `Generate and install shell tab-completion for kubectl-k8i.

This enables tab-completion for all flags when using "kubectl k8i --<TAB>".
Requires kubectl 1.26+ which supports plugin completion via kubectl_complete-* scripts.

Installation:

  1. Generate the completion script:

     kubectl k8i completion > kubectl_complete-k8i

  2. Make it executable:

     chmod +x kubectl_complete-k8i

  3. Move it to a directory in your $PATH:

     sudo mv kubectl_complete-k8i /usr/local/bin/

  Or as a one-liner:

     kubectl k8i completion > kubectl_complete-k8i && chmod +x kubectl_complete-k8i && sudo mv kubectl_complete-k8i /usr/local/bin/

  If installed via "make install", the completion script is set up automatically.

Verify:

  Open a new terminal session and type:

     kubectl k8i --<TAB><TAB>

  You should see all available flags (--context, --labels, --taints, --filter, etc.).

How it works:

  When you press TAB after "kubectl k8i", kubectl looks for an executable
  named "kubectl_complete-k8i" in your $PATH. This script calls
  "kubectl-k8i __complete" with the current arguments, and the built-in
  Cobra completion engine returns matching suggestions.

Supported shells: bash, zsh, fish (any shell with kubectl completion configured).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(os.Stdout, completionScript)
			return err
		},
	}
}

// completeLabelSelectors returns available node label keys from the cluster.
func completeLabelSelectors(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	nodes, err := getNodeListForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Collect unique label key=value pairs.
	seen := make(map[string]struct{})
	var results []string
	for _, node := range nodes {
		for k, v := range node.Labels {
			entry := k + "=" + v
			if _, ok := seen[entry]; !ok {
				seen[entry] = struct{}{}
				results = append(results, entry)
			}
		}
	}
	return results, cobra.ShellCompDirectiveNoFileComp
}

// completeTaintKeys returns available taint keys from cluster nodes.
func completeTaintKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	nodes, err := getNodeListForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	seen := make(map[string]struct{})
	var results []string
	for _, node := range nodes {
		for _, t := range node.Spec.Taints {
			// Offer both key and key=value.
			if _, ok := seen[t.Key]; !ok {
				seen[t.Key] = struct{}{}
				results = append(results, t.Key)
			}
			if t.Value != "" {
				kv := t.Key + "=" + t.Value
				if _, ok := seen[kv]; !ok {
					seen[kv] = struct{}{}
					results = append(results, kv)
				}
			}
		}
	}
	return results, cobra.ShellCompDirectiveNoFileComp
}

// completeContexts returns available kubeconfig contexts.
func completeContexts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var results []string
	for name := range config.Contexts {
		results = append(results, name)
	}
	return results, cobra.ShellCompDirectiveNoFileComp
}

// completeFilterValues returns available filter attribute=value suggestions.
func completeFilterValues(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// If user hasn't typed "=" yet, suggest attribute names.
	if !strings.Contains(toComplete, "=") {
		attrs := make([]string, len(filter.SupportedFilterAttributes))
		for i, a := range filter.SupportedFilterAttributes {
			attrs[i] = a + "="
		}
		return attrs, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// completeSortValues returns available sort column=direction suggestions.
func completeSortValues(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !strings.Contains(toComplete, "=") {
		cols := make([]string, len(sortpkg.SupportedSortColumns))
		for i, c := range sortpkg.SupportedSortColumns {
			cols[i] = c + "="
		}
		return cols, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
	// After "=", suggest full column=direction values.
	prefix := toComplete[:strings.Index(toComplete, "=")+1]
	return []string{prefix + "asc", prefix + "desc"}, cobra.ShellCompDirectiveNoFileComp
}

// getNodeListForCompletion fetches the node list from the cluster for completion purposes.
func getNodeListForCompletion(cmd *cobra.Command) ([]corev1.Node, error) {
	clientset, err := getClientsetForCompletion(cmd)
	if err != nil {
		return nil, err
	}
	nodeList, err := clientset.CoreV1().Nodes().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

// completeDeployments returns available deployments in "namespace/name" format.
func completeDeployments(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clientset, err := getClientsetForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Determine which namespace to search in based on what the user has typed so far.
	// If toComplete contains "/" — user already typed a namespace, list deployments in it.
	// Otherwise — list all namespaces and suggest "namespace/" prefixes.
	if idx := strings.Index(toComplete, "/"); idx >= 0 {
		namespace := toComplete[:idx]
		deplList, err := clientset.AppsV1().Deployments(namespace).List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		results := make([]string, 0, len(deplList.Items))
		for _, d := range deplList.Items {
			results = append(results, namespace+"/"+d.Name)
		}
		return results, cobra.ShellCompDirectiveNoFileComp
	}

	// No "/" yet — suggest namespace/ prefixes.
	nsList, err := clientset.CoreV1().Namespaces().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	results := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		results = append(results, ns.Name+"/")
	}
	return results, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// completeStatefulSets returns available statefulsets in "namespace/name" format.
func completeStatefulSets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clientset, err := getClientsetForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if idx := strings.Index(toComplete, "/"); idx >= 0 {
		namespace := toComplete[:idx]
		stsList, err := clientset.AppsV1().StatefulSets(namespace).List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		results := make([]string, 0, len(stsList.Items))
		for _, s := range stsList.Items {
			results = append(results, namespace+"/"+s.Name)
		}
		return results, cobra.ShellCompDirectiveNoFileComp
	}

	// No "/" yet — suggest namespace/ prefixes.
	nsList, err := clientset.CoreV1().Namespaces().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	results := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		results = append(results, ns.Name+"/")
	}
	return results, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// completeNamespaces returns available namespace names from the cluster.
func completeNamespaces(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clientset, err := getClientsetForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	nsList, err := clientset.CoreV1().Namespaces().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	results := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		results = append(results, ns.Name)
	}
	return results, cobra.ShellCompDirectiveNoFileComp
}

// getClientsetForCompletion builds a kubernetes clientset for completion functions.
func getClientsetForCompletion(cmd *cobra.Command) (kubernetes.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if ctx, _ := cmd.Flags().GetString("context"); ctx != "" {
		configOverrides.CurrentContext = ctx
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restConfig)
}
