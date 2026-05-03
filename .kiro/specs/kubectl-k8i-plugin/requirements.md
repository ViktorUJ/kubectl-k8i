# Requirements Document

## Introduction

This document describes the requirements for the Go plugin `kubectl-k8i` — a cross-platform replacement for the bash script `k8i.sh`. The plugin displays detailed information about Kubernetes node resources in a tabular format: pod usage, CPU, memory, load percentages with color coding, as well as node metadata (EC2 instance ID, instance type, capacity type, architecture, availability zone, nodepool, Karpenter nodeclaim, node age, taints). The plugin performs all operations in memory without temporary files, uses parallel data collection via goroutines, and is optimized for clusters with up to 1000 nodes and 10000 pods. The plugin must comply with krew requirements for publication in the official kubectl plugin repository and work on Windows, Linux, and macOS.

## Glossary

- **Plugin**: A Go binary named `kubectl-k8i` that kubectl discovers as a plugin by naming convention
- **Node_Table**: Tabular output containing information about Kubernetes cluster nodes
- **Resource_Parser**: Component that converts string representations of Kubernetes resources (CPU in millicores, memory in Ki/Mi/Gi) to numeric values
- **Label_Detector**: Component that extracts node metadata from Kubernetes labels of various providers (Karpenter, Spotinst, EKS)
- **Filter_Engine**: Component that filters nodes by metadata attributes (ec2_type, instance_type, arch, zone, pool, nodeclaim, autoscaler)
- **Sort_Engine**: Component that sorts nodes by a specified column in a given direction
- **Color_Renderer**: Component that applies ANSI colors to load percentages based on thresholds
- **Age_Formatter**: Component that converts an ISO timestamp of node creation to a human-readable age format
- **Data_Collector**: Component that collects node data via the Kubernetes API (nodes, top nodes, pod resources, nodeclaims)
- **CLI_Parser**: Component that parses the plugin's command-line arguments
- **Progress_Reporter**: Component that displays a progress bar during data collection
- **Taint_Analyzer**: Component that extracts, displays, and groups node taints to identify logical node groups (e.g., nodes of the same Karpenter nodepool with different names but identical taints)
- **Krew_Manifest**: YAML manifest file for publishing the plugin in the krew index
- **Output_Formatter**: Component that formats node data into various output formats (table, JSON, YAML)
- **Terminal_Detector**: Component that detects terminal width for adaptive table formatting
- **Retry_Wrapper**: Component that implements retry logic with exponential backoff for Kubernetes API requests
- **Autoscaler_Detector**: Logic for determining the autoscaler type (Karpenter, CAS, Spot.io) from node labels

## Requirements

### Requirement 1: CPU Resource Parsing

**User Story:** As a DevOps engineer, I want the plugin to correctly parse all Kubernetes CPU resource formats so that I can see accurate values in the table.

#### Acceptance Criteria

1. WHEN a CPU value with the "m" suffix is provided, THE Resource_Parser SHALL convert the value to cores by dividing by 1000
2. WHEN a CPU value without a suffix is provided, THE Resource_Parser SHALL interpret the value as whole cores
3. WHEN a CPU value is empty, null, or missing, THE Resource_Parser SHALL return zero
4. FOR ALL valid CPU strings, parsing then formatting then parsing SHALL produce an equivalent numeric value (round-trip property)

### Requirement 2: Memory Resource Parsing

**User Story:** As a DevOps engineer, I want the plugin to correctly parse all Kubernetes memory formats so that I can see accurate values in gigabytes.

#### Acceptance Criteria

1. WHEN a memory value with the "Ki" suffix is provided, THE Resource_Parser SHALL convert the value to gigabytes by dividing by 1048576
2. WHEN a memory value with the "Mi" suffix is provided, THE Resource_Parser SHALL convert the value to gigabytes by dividing by 1024
3. WHEN a memory value with the "Gi" suffix is provided, THE Resource_Parser SHALL interpret the value as gigabytes
4. WHEN a memory value is empty, null, or missing, THE Resource_Parser SHALL return zero
5. FOR ALL valid memory strings, parsing then formatting then parsing SHALL produce an equivalent numeric value (round-trip property)

### Requirement 3: Node Metadata Detection from Labels

**User Story:** As a DevOps engineer, I want to see node metadata (capacity type, nodepool, architecture, zone) to understand the cluster configuration.

#### Acceptance Criteria

1. THE Label_Detector SHALL extract capacity type by checking labels in the following priority order: `karpenter.sh/capacity-type`, `karpenter.k8s.aws/capacity-type`, `spotinst.io/node-lifecycle`, `eks.amazonaws.com/capacityType`
2. THE Label_Detector SHALL extract nodepool by checking labels in the following priority order: `karpenter.sh/nodepool`, `karpenter.k8s.aws/nodepool`, `spotinst.io/ocean-vng-id`, `eks.amazonaws.com/nodegroup`
3. WHEN the capacity type value is any variant of "on-demand" (case-insensitive, with or without hyphen) or "ON_DEMAND", THE Label_Detector SHALL normalize the value to "od"
4. WHEN the EKS nodegroup label value exceeds 15 characters, THE Label_Detector SHALL truncate the value to 15 characters
5. THE Label_Detector SHALL extract architecture from the `kubernetes.io/arch` label
6. THE Label_Detector SHALL extract availability zone from the `topology.kubernetes.io/zone` label and use only the last 2 characters
7. THE Label_Detector SHALL extract instance type from the `node.kubernetes.io/instance-type` label
8. WHEN a label is not found in any of the checked label keys, THE Label_Detector SHALL return "x" as the default value
9. THE Label_Detector SHALL extract EC2 instance ID from the node providerID field by matching the pattern `i-[A-Za-z0-9-]+`
10. WHEN the Karpenter nodeclaim name exceeds 20 characters, THE Label_Detector SHALL truncate the value to 20 characters
11. THE Label_Detector SHALL determine the autoscaler type for each node based on the following priority: if the node has label `karpenter.sh/nodepool` OR `karpenter.k8s.aws/nodepool`, the autoscaler type SHALL be "karpenter"; else if the node has label `spotinst.io/ocean-vng-id` OR `spotinst.io/node-lifecycle`, the autoscaler type SHALL be "spotio"; else if the node has label `eks.amazonaws.com/nodegroup`, the autoscaler type SHALL be "cas"; otherwise the autoscaler type SHALL be "x"
12. THE Label_Detector SHALL include the autoscaler type in the node metadata and display it in the node info column

### Requirement 4: Node Age Formatting

**User Story:** As a DevOps engineer, I want to see node ages in a human-readable format to quickly assess node lifetimes.

#### Acceptance Criteria

1. WHEN the node age is 1 day or more, THE Age_Formatter SHALL display the age in the format `{days}d{hours}h`
2. WHEN the node age is less than 1 day but 1 hour or more, THE Age_Formatter SHALL display the age in the format `{hours}h{minutes}m`
3. WHEN the node age is less than 1 hour, THE Age_Formatter SHALL display the age in the format `{minutes}m`
4. WHEN the creation timestamp is empty or null, THE Age_Formatter SHALL return "x"

### Requirement 5: Load Color Coding

**User Story:** As a DevOps engineer, I want to see color-coded load indicators for nodes to quickly identify overloaded nodes.

#### Acceptance Criteria

1. WHEN the load percentage is 60 or less, THE Color_Renderer SHALL display the value in green (ANSI code `\033[0;32m`)
2. WHEN the load percentage is greater than 60 and 80 or less, THE Color_Renderer SHALL display the value in yellow (ANSI code `\033[1;33m`)
3. WHEN the load percentage is greater than 80, THE Color_Renderer SHALL display the value in red (ANSI code `\033[0;31m`)
4. WHEN colors are disabled, THE Color_Renderer SHALL display the load percentage without ANSI escape codes
5. WHEN the load value is non-numeric, THE Color_Renderer SHALL display the value as-is without color

### Requirement 6: Data Collection from Kubernetes API

**User Story:** As a DevOps engineer, I want the plugin to collect node data via the Kubernetes API to see up-to-date cluster information.

#### Acceptance Criteria

1. THE Data_Collector SHALL retrieve node information using the Kubernetes API (equivalent to `kubectl get nodes -o json`)
2. THE Data_Collector SHALL retrieve node resource usage using the Kubernetes metrics API (equivalent to `kubectl top nodes`)
3. THE Data_Collector SHALL retrieve running pod resource requests and limits aggregated per node
4. THE Data_Collector SHALL retrieve Karpenter nodeclaim resources when the CRD exists in the cluster
5. IF the Karpenter nodeclaim CRD does not exist, THEN THE Data_Collector SHALL continue without nodeclaim data and set nodeclaim values to "x"
6. THE Data_Collector SHALL process only nodes with Ready condition status equal to True
7. WHILE collecting data, THE Progress_Reporter SHALL display a progress indicator on stderr
8. THE Data_Collector SHALL execute API calls for nodes, metrics, pods, and nodeclaims concurrently using goroutines to minimize total collection time
9. THE Data_Collector SHALL store all collected data in in-memory data structures without writing temporary files to disk
10. THE Data_Collector SHALL retrieve node taints from the Kubernetes API as part of node information collection

### Requirement 7: Tabular Output

**User Story:** As a DevOps engineer, I want to see node information in a structured table to quickly analyze the cluster state.

#### Acceptance Criteria

1. THE Node_Table SHALL display columns in the following order: NODE, PODS (used/max), CPU cores (req/lim/use/total), CPU LOAD, MEMORY GB (req/lim/use/total), MEM LOAD, Node info (ec2/type/spot/arch/zone/pool/nodeclaim/age)
2. THE Node_Table SHALL display a header with column names before the data rows
3. THE Node_Table SHALL display a separator line between the header and data rows
4. THE Node_Table SHALL display the data collection timestamp before the table
5. WHEN a filter is applied, THE Node_Table SHALL display the active filter before the table
6. WHEN a sort is applied, THE Node_Table SHALL display the active sort specification before the table
7. WHEN no nodes match the applied filter, THE Node_Table SHALL display a message indicating no nodes match the filter
8. WHEN the `--no-headers` flag is provided, THE Node_Table SHALL NOT display the header row with column names
9. WHEN the `--no-headers` flag is provided, THE Node_Table SHALL NOT display the separator line between header and data rows
10. WHEN the `--no-headers` flag is provided, THE Node_Table SHALL NOT display the data collection timestamp
11. WHEN the `--no-headers` flag is provided, THE Node_Table SHALL NOT display filter or sort annotations
12. WHEN the `--no-headers` flag is provided, THE Node_Table SHALL display only the data rows containing node information

### Requirement 8: Node Filtering

**User Story:** As a DevOps engineer, I want to filter the output by node attributes to see only the nodes I am interested in.

#### Acceptance Criteria

1. WHEN the `--filter` flag is provided with format `attribute=value`, THE Filter_Engine SHALL display only nodes matching the specified attribute and value
2. THE Filter_Engine SHALL support the following filter attributes: ec2_type, instance_type, arch, zone, pool, nodeclaim, taint, autoscaler
3. WHEN an unsupported filter attribute is provided, THE Filter_Engine SHALL return an error message listing supported attributes
4. WHEN the filter format is invalid (not `attribute=value`), THE Filter_Engine SHALL return an error message describing the expected format

### Requirement 9: Node Sorting

**User Story:** As a DevOps engineer, I want to sort nodes by various columns to quickly find nodes with the highest or lowest load.

#### Acceptance Criteria

1. WHEN the `--sort` flag is provided with format `column=direction`, THE Sort_Engine SHALL sort nodes by the specified column in the specified direction
2. THE Sort_Engine SHALL support the following sort columns: name, pods, cpu_req, cpu_lim, cpu_use, cpu_cap, cpu_load, mem_req, mem_lim, mem_use, mem_cap, mem_load, ec2_type, instance_type, arch, zone, pool, age, taint, autoscaler
3. THE Sort_Engine SHALL support "asc" and "desc" as sort directions
4. WHEN no sort is specified, THE Sort_Engine SHALL sort by pool in ascending order as the default
5. WHEN an unsupported sort column is provided, THE Sort_Engine SHALL return an error message listing supported columns
6. WHEN the sort format is invalid (not `column=direction`), THE Sort_Engine SHALL return an error message describing the expected format
7. WHEN sorting by numeric columns (pods, cpu_req, cpu_lim, cpu_use, cpu_cap, cpu_load, mem_req, mem_lim, mem_use, mem_cap, mem_load, age), THE Sort_Engine SHALL use numeric comparison
8. WHEN sorting by text columns (name, ec2_type, instance_type, arch, zone, pool, taint, autoscaler), THE Sort_Engine SHALL use lexicographic comparison

### Requirement 10: Command-Line Arguments

**User Story:** As a DevOps engineer, I want to control the plugin behavior via command-line arguments to customize the output for my needs.

#### Acceptance Criteria

1. THE CLI_Parser SHALL accept the `--context CONTEXT` flag to specify the Kubernetes context
2. THE CLI_Parser SHALL accept the `--labels SELECTOR` flag to filter nodes by Kubernetes label selector at the API level
3. THE CLI_Parser SHALL accept the `--filter FILTER` flag to filter output by node attributes
4. THE CLI_Parser SHALL accept the `--sort COLUMN=DIR` flag to sort output by the specified column and direction
5. THE CLI_Parser SHALL accept the `--fargate` flag to include Fargate nodes in the output
6. THE CLI_Parser SHALL accept the `--color true|false` flag to force enable or disable ANSI colors
7. THE CLI_Parser SHALL accept the `--debug true|false` flag to enable or disable debug output
8. THE CLI_Parser SHALL accept the `--help` flag to display usage information
9. THE CLI_Parser SHALL accept the `--group-by` flag with value `taint` to group nodes by common taint sets
10. WHEN no `--fargate` flag is provided, THE Plugin SHALL hide nodes with names starting with "fargate-"
11. WHEN a required flag value is missing, THE CLI_Parser SHALL return an error message indicating the missing value
12. WHEN an unknown flag is provided, THE CLI_Parser SHALL return an error message and suggest using `--help`
13. THE CLI_Parser SHALL accept the `--output FORMAT` flag (with `-o` as a short alias) to specify the output format, where FORMAT is one of: `table`, `json`, `yaml`
16. THE CLI_Parser SHALL accept the `--no-headers` flag to suppress table header, separator line, timestamp, and filter/sort annotations in the output
14. WHEN no `--output` flag is provided, THE CLI_Parser SHALL default to `table` format
15. WHEN an unsupported output format is provided, THE CLI_Parser SHALL return an error message listing supported formats (table, json, yaml)

### Requirement 11: Cross-Platform Compatibility

**User Story:** As a DevOps engineer, I want to use the plugin on Windows, Linux, and macOS so that I am not dependent on the operating system.

#### Acceptance Criteria

1. THE Plugin SHALL compile into native binaries for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64, arm64)
2. THE Plugin SHALL use only Go standard library and Kubernetes client-go for platform-independent operations
3. THE Plugin SHALL detect terminal color support on each platform and enable colors by default when the terminal supports them
4. THE Plugin SHALL handle file path separators correctly on each platform
5. THE Plugin SHALL work as a standalone binary with ZERO external dependencies — no jq, bc, awk, sed, grep, date, or any other external utility SHALL be required at runtime
6. THE Plugin SHALL NOT shell out to any external process for data processing, formatting, or output rendering — all operations SHALL be performed in-process using Go standard library and imported Go modules
7. THE Plugin SHALL provide built-in JSON and YAML output formats, eliminating the need for external tools like jq for data extraction

### Requirement 12: Krew Compatibility

**User Story:** As a plugin developer, I want the plugin to comply with krew requirements so that it can be published in the official repository.

#### Acceptance Criteria

1. THE Plugin SHALL be named `kubectl-k8i` following the kubectl plugin naming convention
2. THE Plugin SHALL include a krew manifest YAML file with plugin name, version, description, homepage, and platform binaries
3. THE Plugin SHALL include a LICENSE file
4. THE Plugin SHALL include a README.md with installation instructions, usage examples, and feature description
5. THE Plugin SHALL exit with code 0 on success and non-zero on failure
6. THE Plugin SHALL write error messages to stderr and normal output to stdout
7. THE Plugin SHALL support the `kubectl k8i` invocation syntax after installation via krew

### Requirement 13: Order of Operations

**User Story:** As a DevOps engineer, I want filtering and sorting to be applied in a defined order so that the result is predictable.

#### Acceptance Criteria

1. THE Plugin SHALL apply operations in the following order: label selector (API-level filtering), then output filter, then sort
2. WHEN both `--labels` and `--filter` are provided, THE Plugin SHALL first retrieve nodes matching the label selector from the API, then apply the output filter to the retrieved nodes
3. WHEN both `--filter` and `--sort` are provided, THE Plugin SHALL first filter nodes, then sort the filtered result

### Requirement 14: Error Handling

**User Story:** As a DevOps engineer, I want to receive clear error messages to quickly diagnose problems.

#### Acceptance Criteria

1. IF the Kubernetes API is unreachable, THEN THE Plugin SHALL return an error message indicating the connection failure
2. IF the metrics API (kubectl top) is unavailable, THEN THE Plugin SHALL display nodes with zero usage values and continue operation
3. IF no Ready nodes are found, THEN THE Plugin SHALL display a message indicating no nodes were found
4. IF the specified Kubernetes context does not exist, THEN THE Plugin SHALL return an error message indicating the invalid context
5. WHEN debug mode is enabled, THE Plugin SHALL write debug information to stderr
6. WHEN a transient API error occurs (network timeout, connection refused, 429 Too Many Requests, 5xx server error), THE Retry_Wrapper SHALL retry the request with exponential backoff
7. WHEN a permanent API error occurs (401 Unauthorized, 403 Forbidden, 404 Not Found for non-nodeclaim resources), THE Plugin SHALL NOT retry and SHALL immediately return the error
8. WHEN all retry attempts are exhausted, THE Plugin SHALL return the last error with a message indicating the number of retries attempted

### Requirement 15: Unit and Property-Based Testing

**User Story:** As a plugin developer, I want to have a comprehensive set of unit and property-based tests to ensure the correctness of each plugin component.

#### Acceptance Criteria

1. THE Plugin SHALL include unit tests for the Resource_Parser covering all CPU and memory format variations
2. THE Plugin SHALL include unit tests for the Label_Detector covering all label priority chains and normalization rules
3. THE Plugin SHALL include unit tests for the Age_Formatter covering all time range formats
4. THE Plugin SHALL include unit tests for the Color_Renderer covering all threshold boundaries
5. THE Plugin SHALL include unit tests for the Filter_Engine covering all supported attributes and error cases
6. THE Plugin SHALL include unit tests for the Sort_Engine covering all columns, directions, and numeric vs lexicographic comparison
7. THE Plugin SHALL include unit tests for the CLI_Parser covering all flags, missing values, and unknown flags
8. THE Plugin SHALL include property-based tests for the Resource_Parser round-trip property (parse then format then parse produces equivalent value)
9. THE Plugin SHALL include unit tests for the Taint_Analyzer covering taint extraction, display formatting, filtering by key and key=value, and grouping by common taint sets
10. THE Plugin SHALL achieve test coverage of 80 percent or higher for core logic packages
11. THE Plugin SHALL include integration tests using client-go fake clientset to simulate cluster scenarios with nodes containing various labels, taints, pods with resource requests and limits, and metrics data
12. THE Plugin SHALL include integration tests that verify the full Data_Collector pipeline produces correct aggregated results from fake Kubernetes API responses
13. THE Plugin SHALL include end-to-end output tests that verify the complete Node_Table output format given a set of mock nodes, pods, and metrics
14. WHEN filters, sorting, taint grouping, or fargate hiding are applied, THE end-to-end output tests SHALL verify the complete output matches expected tabular format
15. THE Plugin SHALL include Go benchmark tests (using `testing.B`) that verify processing of 1000 nodes and 10000 pods completes within acceptable time and memory limits
16. THE benchmark tests SHALL measure the in-memory data processing pipeline including resource aggregation, filtering, sorting, and table rendering
17. THE Plugin SHALL include error handling tests that verify graceful degradation when the metrics API is unavailable
18. THE Plugin SHALL include error handling tests that verify graceful degradation when the Karpenter nodeclaim CRD is missing
19. THE Plugin SHALL include error handling tests that verify correct behavior for an empty cluster with zero Ready nodes
20. THE Plugin SHALL include error handling tests that verify correct error reporting for an invalid Kubernetes context
21. THE Plugin SHALL include cross-platform tests that verify the `--color false` flag produces output containing no ANSI escape codes
22. THE Plugin SHALL include cross-platform tests that verify the Color_Renderer correctly detects terminal color support on the current platform
23. THE Plugin SHALL include unit tests for the Retry_Wrapper covering transient error retry, permanent error non-retry, maximum retry exhaustion, and exponential backoff timing
24. THE Plugin SHALL include property-based tests for the autoscaler type detection verifying the priority chain (karpenter > spotio > cas > x) across all label combinations
25. THE Plugin SHALL include unit tests for the enhanced debug mode verifying that API call details, retry attempts, data processing counts, and filter/sort operations are logged to stderr in structured format

### Requirement 16: Load Percentage Calculation

**User Story:** As a DevOps engineer, I want to see CPU and memory load percentages to quickly assess node utilization.

#### Acceptance Criteria

1. THE Plugin SHALL calculate CPU load percentage as `(cpu_usage_millicores * 100) / cpu_capacity_millicores`, rounded to an integer
2. THE Plugin SHALL calculate memory load percentage as `(memory_usage_gb * 100) / memory_capacity_gb`, rounded to an integer
3. WHEN the capacity value is zero, THE Plugin SHALL display the load percentage as zero
4. WHEN the usage value is zero or missing, THE Plugin SHALL display the load percentage as zero
5. THE Plugin SHALL pad single-digit load percentages with a leading zero (format `%02d`)

### Requirement 17: Node Taint Support

**User Story:** As a DevOps engineer, I want to see node taints and group nodes by common taints to identify logical node groups in clusters with Karpenter, where nodepool names may differ but taints define membership in the same logical group.

#### Acceptance Criteria

1. THE Taint_Analyzer SHALL extract all taints from each node and display them in the node info column in the format `key=value:effect` (or `key:effect` when value is empty)
2. WHEN the `--filter` flag is provided with `taint=KEY`, THE Filter_Engine SHALL display only nodes that have a taint with the specified key (regardless of value or effect)
3. WHEN the `--filter` flag is provided with `taint=KEY=VALUE`, THE Filter_Engine SHALL display only nodes that have a taint with the specified key and value (regardless of effect)
4. WHEN the `--sort` flag is provided with `taint=asc` or `taint=desc`, THE Sort_Engine SHALL sort nodes by their taint keys concatenated in alphabetical order
5. WHEN the `--group-by taint` flag is provided, THE Taint_Analyzer SHALL group nodes by their common taint sets and display a group separator between groups of nodes sharing identical taint sets
6. WHEN a node has no taints, THE Taint_Analyzer SHALL display "none" in the taints field
7. THE Taint_Analyzer SHALL display multiple taints separated by a comma

### Requirement 18: In-Memory Data Processing

**User Story:** As a DevOps engineer, I want the plugin to process all data in memory without using temporary files to ensure clean operation and avoid file system permission issues.

#### Acceptance Criteria

1. THE Plugin SHALL perform all data processing in memory without creating temporary files on disk
2. THE Plugin SHALL use the Go client-go library for all Kubernetes API interactions instead of shelling out to kubectl
3. THE Data_Collector SHALL use concurrent goroutines to collect nodes, metrics, pods, and nodeclaims data in parallel
4. THE Data_Collector SHALL use Go channels or sync.WaitGroup to synchronize parallel API calls and aggregate results
5. IF any parallel API call fails (except nodeclaims), THEN THE Data_Collector SHALL cancel remaining calls and return an error
6. THE Plugin SHALL use Go maps for O(1) lookups when aggregating pod resources per node and matching nodeclaims to nodes

### Requirement 19: Scalability

**User Story:** As a DevOps engineer, I want the plugin to work efficiently on large clusters to get information without long wait times.

#### Acceptance Criteria

1. THE Plugin SHALL handle clusters with up to 1000 nodes and up to 10000 pods
2. THE Data_Collector SHALL complete data collection for a cluster with 1000 nodes within 10 seconds under normal network conditions
3. THE Data_Collector SHALL use Kubernetes API list operations to retrieve all resources in a single API call per resource type (nodes, pods, metrics, nodeclaims) instead of per-node queries
4. THE Plugin SHALL use map-based data structures for O(1) lookups when associating pods with nodes, metrics with nodes, and nodeclaims with nodes
5. THE Plugin SHALL use field selectors in pod list operations (status.phase=Running) to minimize data transfer from the API server
6. WHEN processing node data, THE Plugin SHALL iterate over the collected data once per transformation step to maintain linear time complexity

### Requirement 20: Integration Testing

**User Story:** As a plugin developer, I want to have a separate set of integration tests to ensure that all plugin components work correctly together as a unified system.

#### Acceptance Criteria

1. THE Plugin SHALL include integration tests that create a realistic cluster scenario using client-go fake clientset with nodes of different instance types, capacity types, architectures, and availability zones
2. THE Plugin SHALL include integration tests that simulate nodes with Karpenter labels, Spotinst labels, and EKS labels to verify Label_Detector priority chains work correctly in the full pipeline
3. THE Plugin SHALL include integration tests that simulate pods with varying resource requests and limits distributed across multiple nodes to verify per-node resource aggregation
4. THE Plugin SHALL include integration tests that simulate metrics API responses with CPU and memory usage data to verify load percentage calculation in the full pipeline
5. THE Plugin SHALL include integration tests that verify the Filter_Engine correctly filters nodes by each supported attribute (ec2_type, instance_type, arch, zone, pool, nodeclaim, taint, autoscaler) when integrated with the Data_Collector
6. THE Plugin SHALL include integration tests that verify the Sort_Engine correctly sorts the complete Node_Table output by each supported column
7. THE Plugin SHALL include integration tests that verify the `--group-by taint` flag produces correctly grouped output with group separators between nodes sharing identical taint sets
8. THE Plugin SHALL include integration tests that verify Fargate nodes are hidden by default and shown when the `--fargate` flag is provided
9. THE Plugin SHALL include integration tests that verify the Plugin continues operation with zero usage values when the metrics API returns an error
10. THE Plugin SHALL include integration tests that verify the Plugin continues operation with "x" nodeclaim values when the Karpenter nodeclaim CRD does not exist
11. THE Plugin SHALL include integration tests that verify the complete output format matches expected tabular layout including header, separator, data rows, timestamp, and filter/sort annotations
12. THE Plugin SHALL tag integration tests with a Go build tag `integration` to allow separate execution from unit tests
13. THE Plugin SHALL include integration tests that verify the Retry_Wrapper correctly retries transient API errors and does not retry permanent errors when integrated with the Data_Collector
14. THE Plugin SHALL include integration tests that verify the Label_Detector correctly detects autoscaler type (karpenter, cas, spotio, x) for nodes with various label combinations
15. THE Plugin SHALL include integration tests that verify the enhanced debug mode outputs structured log entries to stderr covering API calls, retry attempts, data processing counts, and filter/sort operations


### Requirement 21: Multiple Output Formats (JSON, YAML, table)

**User Story:** As a DevOps engineer, I want to receive node data in various formats (table, JSON, YAML) to use the plugin output in scripts and pipelines without external utilities like jq.

#### Acceptance Criteria

1. WHEN the `--output table` flag is provided (or no `--output` flag), THE Plugin SHALL render the node data in the default tabular format
2. WHEN the `--output json` flag is provided, THE Plugin SHALL render all node data as a valid JSON array, where each element is a JSON object containing all fields of NodeInfo
3. WHEN the `--output yaml` flag is provided, THE Plugin SHALL render all node data as a valid YAML document containing a list of node objects with all fields of NodeInfo
4. FOR ALL node lists, the JSON output SHALL be parseable by Go's `encoding/json` standard library without errors
5. FOR ALL node lists, the YAML output SHALL be parseable by a standard YAML parser without errors
6. FOR ALL node lists, the JSON and YAML outputs SHALL contain the same set of node data as the table output — no nodes SHALL be added or omitted based on output format
7. WHEN the `--output json` or `--output yaml` flag is provided, THE Plugin SHALL NOT include ANSI color codes in the output
8. WHEN the `--output json` or `--output yaml` flag is provided, THE Plugin SHALL NOT include the progress indicator, timestamp header, or filter/sort annotations — only the structured data SHALL be output
9. THE Plugin SHALL use Go standard library `encoding/json` for JSON output and `gopkg.in/yaml.v3` (or equivalent) for YAML output — no external CLI tools SHALL be invoked

### Requirement 22: Terminal-Adaptive Output

**User Story:** As a DevOps engineer, I want the tabular output to adapt to my terminal width so that the table remains readable on narrow screens and does not break formatting.

#### Acceptance Criteria

1. THE Table Renderer SHALL detect the current terminal width at runtime before rendering the table
2. ON Linux and macOS, THE Plugin SHALL detect terminal width using `golang.org/x/term` package or `os.Stdout.Fd()` with POSIX ioctl
3. ON Windows, THE Plugin SHALL detect terminal width using the Windows Console API via `golang.org/x/term` (which abstracts platform differences)
4. WHEN the terminal width cannot be detected (e.g., output is piped to a file), THE Plugin SHALL default to 200 characters width
5. WHEN the detected terminal width is less than the full table width, THE Table Renderer SHALL truncate long column values (node name, nodepool, nodeclaim, taints) to fit within the available width
6. WHEN truncating column values, THE Table Renderer SHALL append an ellipsis character ("…") to indicate truncation
7. THE Table Renderer SHALL NOT truncate numeric columns (pods, CPU, memory, load percentages) — only text metadata columns SHALL be subject to truncation
8. WHEN the `--output json` or `--output yaml` flag is provided, THE Plugin SHALL NOT apply terminal width detection or truncation — structured output SHALL always contain full untruncated data

### Requirement 23: Retry Logic for API Requests

**User Story:** As a DevOps engineer, I want the plugin to automatically retry failed API requests with exponential backoff so that the plugin works reliably under unstable network conditions or Kubernetes API throttling.

#### Acceptance Criteria

1. THE Retry_Wrapper SHALL retry API calls on transient errors: network timeouts, connection refused, 429 Too Many Requests, and 5xx server errors
2. THE Retry_Wrapper SHALL NOT retry API calls on permanent errors: 401 Unauthorized, 403 Forbidden, 404 Not Found (for non-nodeclaim resources)
3. THE Retry_Wrapper SHALL use exponential backoff with jitter, starting at 100ms and doubling each attempt (100ms, 200ms, 400ms, 800ms, 1600ms)
4. THE Retry_Wrapper SHALL perform a maximum of 5 retry attempts per API call
5. WHEN all retry attempts are exhausted, THE Retry_Wrapper SHALL return the last error with context indicating the number of retries
6. WHEN debug mode is enabled, THE Retry_Wrapper SHALL log each retry attempt to stderr including the attempt number, backoff duration, and error reason
7. THE Retry_Wrapper SHALL use client-go built-in retry mechanisms where available, or implement a custom retry wrapper for API calls not covered by client-go
8. THE Retry_Wrapper SHALL add random jitter (up to 50% of the backoff interval) to prevent thundering herd effects

### Requirement 24: Autoscaler Type Detection

**User Story:** As a DevOps engineer, I want to see which autoscaler manages each node (Karpenter, Cluster Autoscaler, Spot.io) to understand the cluster's autoscaling topology.

#### Acceptance Criteria

1. THE Label_Detector SHALL detect Karpenter autoscaler when the node has label `karpenter.sh/nodepool` OR `karpenter.k8s.aws/nodepool`, and set autoscaler type to "karpenter"
2. THE Label_Detector SHALL detect Spot.io (Ocean) autoscaler when the node has label `spotinst.io/ocean-vng-id` OR `spotinst.io/node-lifecycle` (without Karpenter labels), and set autoscaler type to "spotio"
3. THE Label_Detector SHALL detect Cluster Autoscaler (CAS) when the node has label `eks.amazonaws.com/nodegroup` (without Karpenter or Spot.io labels), and set autoscaler type to "cas"
4. WHEN none of the autoscaler labels are found, THE Label_Detector SHALL set autoscaler type to "x"
5. THE Label_Detector SHALL apply autoscaler detection priority: karpenter > spotio > cas > x (if multiple labels present, highest priority wins)
6. THE Filter_Engine SHALL support filtering by `autoscaler` attribute (e.g., `--filter autoscaler=karpenter`)
7. THE Sort_Engine SHALL support sorting by `autoscaler` column (e.g., `--sort autoscaler=asc`) using lexicographic comparison
8. THE Output_Formatter SHALL include the autoscaler type in JSON and YAML output
9. THE Node_Table SHALL display the autoscaler type in the node info column