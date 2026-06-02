# kubectl-k8i

> 🇷🇺 [Документация на русском](README_RU.md)

A kubectl plugin that displays detailed Kubernetes node resource information with color-coded load percentages.

## Features

- **Rich tabular view** of node resources: pods, CPU, memory, load percentages, and node metadata
- **Color-coded load percentages**: green (≤60%), yellow (61–80%), red (>80%)
- **Node metadata**: EC2 instance ID, instance type, capacity type, architecture, zone, nodepool, nodeclaim, autoscaler type, age, taints
- **Autoscaler detection**: identifies Karpenter, Cluster Autoscaler (CAS), and Spot.io managed nodes
- **Filter and sort** by any attribute or column
- **Group by taints** to identify logical node groups
- **Workload analysis**: show all workloads (Deployments, StatefulSets, DaemonSets, Pods) running on selected nodes with aggregated CPU/memory data
- **Multiple output formats**: table (default), JSON, YAML
- **Terminal-adaptive rendering**: table columns adjust to terminal width
- **Zero external dependencies**: standalone binary, no jq/awk/sed/grep required
- **Cross-platform**: Linux, macOS, and Windows (amd64 and arm64)

## Installation

### Via krew

```bash
kubectl krew install k8i
```

### Manual download

Download the binary for your platform from the [releases page](https://github.com/ViktorUJ/kubectl-k8i/releases):

| Platform       | Binary                            |
|----------------|-----------------------------------|
| Linux amd64    | `kubectl-k8i_linux_amd64.tar.gz`  |
| Linux arm64    | `kubectl-k8i_linux_arm64.tar.gz`  |
| macOS amd64    | `kubectl-k8i_darwin_amd64.tar.gz` |
| macOS arm64    | `kubectl-k8i_darwin_arm64.tar.gz` |
| Windows amd64  | `kubectl-k8i_windows_amd64.zip`   |
| Windows arm64  | `kubectl-k8i_windows_arm64.zip`   |

Extract the binary and place it in your `$PATH`:

```bash
# Linux / macOS
tar xzf kubectl-k8i_<platform>.tar.gz
chmod +x kubectl-k8i
mv kubectl-k8i /usr/local/bin/
```

Verify the installation:

```bash
kubectl k8i --help
```

## Shell Completion

kubectl-k8i supports shell completion (tab-completion) for all flags and options via the standard kubectl plugin completion mechanism (kubectl 1.26+).

### Setup

After installing the plugin, generate and install the completion script:

```bash
# Generate and install the completion script
kubectl k8i completion > kubectl_complete-k8i
chmod +x kubectl_complete-k8i
sudo mv kubectl_complete-k8i /usr/local/bin/
```

### Verify

Open a new shell session and test:

```bash
kubectl k8i --<TAB><TAB>
```

You should see all available flags (`--context`, `--labels`, `--taints`, `--filter`, `--sort`, etc.).

### How it works

When you press TAB after `kubectl k8i`, kubectl looks for an executable named `kubectl_complete-k8i` in your `$PATH`. This script calls `kubectl-k8i __complete` with the current arguments, and Cobra returns the list of completions.

## Usage

```bash
kubectl k8i [flags]
kubectl k8i analyze [flags]
```

### Flags

| Flag                    | Description                                                    | Default    |
|-------------------------|----------------------------------------------------------------|------------|
| `--context CONTEXT`     | Kubernetes context to use                                      |            |
| `--labels SELECTOR`     | Label selector to filter nodes at the API level                |            |
| `--taints KEY[=VALUE]`  | Filter nodes by taint key or key=value                         |            |
| `--filter ATTR=VALUE`   | Filter output by node attribute                                |            |
| `--sort COLUMN=DIR`     | Sort output by column and direction                            | `pool=asc` |
| `--deployment NS/NAME`  | Show only nodes running pods of this deployment                |            |
| `--statefulset NS/NAME` | Show only nodes running pods of this statefulset               |            |
| `--daemonset NS/NAME`   | Show only nodes running pods of this daemonset                 |            |
| `--namespace NAME`      | Show only nodes running pods from this namespace               |            |
| `--autoscaler VALUE`    | Show only nodes managed by this autoscaler (`karpenter`, `cluster-autoscaler`, `spotio`, `x`) | |
| `--fargate`             | Include Fargate nodes in the output                            | `false`    |
| `--color true\|false`   | Force enable or disable ANSI colors                            | `auto`     |
| `--debug`               | Enable debug output to stderr                                  | `false`    |
| `--group-by taint`      | Group nodes by common taint sets                               |            |
| `--output, -o FORMAT`   | Output format: `table`, `json`, `yaml`                         | `table`    |
| `--no-headers`          | Suppress table header, separator, timestamp, and annotations   | `false`    |
| `--help`                | Display usage information                                      |            |

### Filter attributes

`ec2_type`, `instance_type`, `arch`, `zone`, `pool`, `nodeclaim`, `taint`, `autoscaler`

### Sort columns

`name`, `pods`, `cpu_req`, `cpu_lim`, `cpu_use`, `cpu_cap`, `cpu_load`, `mem_req`, `mem_lim`, `mem_use`, `mem_cap`, `mem_load`, `ec2_type`, `instance_type`, `arch`, `zone`, `pool`, `age`, `taint`, `autoscaler`

## Examples

### Default table output

```bash
kubectl k8i
```

### Filter by instance type

```bash
kubectl k8i --filter instance_type=m5.xlarge
```

### Sort by CPU load descending

```bash
kubectl k8i --sort cpu_load=desc
```

### Show only Karpenter-managed nodes

```bash
kubectl k8i --filter autoscaler=karpenter
```

### Group nodes by taints

```bash
kubectl k8i --group-by taint
```

### Use a specific context

```bash
kubectl k8i --context production
```

### Filter by label selector at the API level

```bash
kubectl k8i --labels "topology.kubernetes.io/zone=us-east-1a"
```

### Filter by taint

```bash
# Show only nodes with taint key "dedicated"
kubectl k8i --taints dedicated

# Show only nodes with taint key=value "dedicated=gpu"
kubectl k8i --taints 'dedicated=gpu'
```

### Include Fargate nodes

```bash
kubectl k8i --fargate
```

### Disable colors (useful for piping)

```bash
kubectl k8i --color false
```

### Suppress headers for scripting

```bash
kubectl k8i --no-headers
```

### Show nodes running a specific deployment

```bash
# Show only nodes that have pods of the "api-server" deployment in the "production" namespace
kubectl k8i --deployment production/api-server
```

Tab completion works for this flag — press TAB after `--deployment` to get a list of namespaces, then TAB again after `namespace/` to get deployments in that namespace.

### Show nodes running a specific statefulset

```bash
# Show only nodes that have pods of the "postgres" statefulset in the "production" namespace
kubectl k8i --statefulset production/postgres
```

Tab completion works the same way as `--deployment`.

### Show nodes running a specific daemonset

```bash
# Show only nodes that have pods of the "fluentd" daemonset in the "logging" namespace
kubectl k8i --daemonset logging/fluentd
```

Tab completion works the same way as `--deployment`.

### Show nodes running pods from a specific namespace

```bash
# Show only nodes that have at least one running pod from the "monitoring" namespace
kubectl k8i --namespace monitoring
```

Tab completion returns all available namespaces.

### Filter by autoscaler type

```bash
# Show only Karpenter-managed nodes
kubectl k8i --autoscaler karpenter

# Show only Cluster Autoscaler (EKS nodegroup) nodes
kubectl k8i --autoscaler cluster-autoscaler

# Show only Spot.io-managed nodes
kubectl k8i --autoscaler spotio

# Show only nodes with no recognized autoscaler
kubectl k8i --autoscaler x
```

Tab completion returns all valid values: `karpenter`, `cluster-autoscaler`, `spotio`, `x`.

### Combine workload filters with other flags

```bash
# Nodes of a deployment, sorted by memory load
kubectl k8i --deployment production/api-server --sort mem_load=desc

# Nodes of a statefulset, JSON output
kubectl k8i --statefulset production/postgres -o json

# Nodes of a daemonset, grouped by taints
kubectl k8i --daemonset logging/fluentd --group-by taint

# Nodes of a namespace, grouped by taints
kubectl k8i --namespace monitoring --group-by taint
```

## Analyze subcommand

`kubectl k8i analyze` shows all workloads running on a selected set of nodes. For each workload it displays the namespace, kind, name, pod count, and aggregated CPU/memory requests, limits, and usage.

### Analyze flags

| Flag                          | Description                                              | Default  |
|-------------------------------|----------------------------------------------------------|----------|
| `--node NAME`                 | Analyze workloads on this specific node                  |          |
| `--labels SELECTOR`           | Analyze workloads on nodes matching this label selector  |          |
| `--taints KEY[=VALUE]`        | Analyze workloads on nodes with this taint               |          |
| `--autoscaler VALUE`          | Analyze workloads on nodes managed by this autoscaler (`karpenter`, `cluster-autoscaler`, `spotio`, `x`) | |
| `--namespace NAME`            | Show only workloads from this namespace (combines with any node selector) | |
| `--exclude-namespace NAME`    | Exclude namespace from output (repeatable)               |          |
| `--cpu-overcommit PERCENT`    | Show only workloads whose CPU limit exceeds request by more than this percent | |
| `--mem-overcommit PERCENT`    | Show only workloads whose memory limit exceeds request by more than this percent | |
| `--output, -o FORMAT`         | Output format: `table`, `json`, `yaml`                   | `table`  |
| `--color true\|false`         | Force enable or disable ANSI colors                      | `auto`   |
| `--context CONTEXT`           | Kubernetes context to use                                |          |
| `--debug`                     | Enable debug output to stderr                            | `false`  |

Exactly one of `--node`, `--labels`, `--taints`, or `--autoscaler` must be provided.

The `--namespace` flag is optional and combines with any node selector to show only workloads from that namespace.

The `--cpu-overcommit` and `--mem-overcommit` flags filter the output to workloads whose limit exceeds the request by more than the given percentage. The overcommit percentage is computed as `(limit - request) / request * 100`. When both flags are set, a workload must exceed both thresholds to be shown. Workloads with a zero request (overcommit not applicable) are excluded when the corresponding threshold is set.

Results are sorted by namespace → kind → name.

### Analyze examples

```bash
# Analyze workloads on a specific node
kubectl k8i analyze --node ip-10-0-1-100

# Analyze workloads on nodes with a label selector
kubectl k8i analyze --labels 'worker-type=spot'

# Analyze workloads on nodes with a specific taint
kubectl k8i analyze --taints 'dedicated=gpu'

# Analyze workloads on all Karpenter-managed nodes
kubectl k8i analyze --autoscaler karpenter

# Analyze only the "default" namespace workloads on Karpenter nodes
kubectl k8i analyze --autoscaler karpenter --namespace default

# Show only workloads whose CPU limit exceeds request by more than 100%
kubectl k8i analyze --autoscaler karpenter --cpu-overcommit 100

# Show only workloads with memory overcommit above 50%
kubectl k8i analyze --node ip-10-0-1-100 --mem-overcommit 50

# Show workloads that overcommit BOTH cpu (>100%) and memory (>50%)
kubectl k8i analyze --autoscaler karpenter --cpu-overcommit 100 --mem-overcommit 50

# Memory overcommit above 50%, excluding the kube-system namespace
kubectl k8i analyze --autoscaler karpenter --mem-overcommit 50 --exclude-namespace kube-system

# Analyze workloads on EKS nodegroup (Cluster Autoscaler) nodes
kubectl k8i analyze --autoscaler cluster-autoscaler

# Analyze workloads on Spot.io nodes, exclude system namespaces
kubectl k8i analyze --autoscaler spotio --exclude-namespace kube-system

# Exclude system namespaces to reduce noise
kubectl k8i analyze --labels 'worker-type=spot' \
  --exclude-namespace kube-system \
  --exclude-namespace monitoring

# Output as JSON
kubectl k8i analyze --node ip-10-0-1-100 -o json

# Output as YAML
kubectl k8i analyze --autoscaler karpenter -o yaml
```

### Combining a node selector with `--namespace`

The `--namespace` flag narrows the output to a single namespace while keeping the node
selection intact. This is useful when you want to see what a specific application namespace
is consuming on a particular group of nodes.

```bash
# Show all workloads from the "default" namespace that run on Karpenter-managed nodes,
# with their requests, limits, and current usage
kubectl k8i analyze --autoscaler karpenter --namespace default
```

How it works:

1. `--autoscaler karpenter` selects every Ready node managed by Karpenter.
2. `--namespace default` keeps only the pods from the `default` namespace running on those nodes.
3. The pods are grouped by their top-level owner (Deployment, StatefulSet, DaemonSet, or standalone Pod)
   and their CPU/memory requests, limits, and usage are aggregated per workload.

Tab completion works for both flags — press TAB after `--autoscaler` to get the valid values,
and after `--namespace` to get the list of cluster namespaces.

### Analyze table output format

```
NAMESPACE            KIND         NAME                                PODS  CPU req/lim/use    MEM req/lim/use GB  CPU OC% MEM OC%
                                                                            (cores)                               (lim/req) (lim/req)
====================================================================================================================================
production           Deployment   api-server                             3  0.75/1.50/0.42     1.00/2.00/0.65     100%    100%
production           StatefulSet  postgres                               2  0.50/1.00/0.31     2.00/4.00/1.80     100%    100%
kube-system          DaemonSet    aws-node                               1  0.02/0.00/0.01     0.03/0.00/0.02     0%      0%
```

The `CPU OC%` and `MEM OC%` columns show the overcommit percentage — how much the limit
exceeds the request, computed as `(limit - request) / request * 100`. A value of `n/a`
means the request is zero (overcommit is undefined).

### Analyze JSON output

```bash
kubectl k8i analyze --node ip-10-0-1-100 -o json
```

```json
[
  {
    "namespace": "production",
    "kind": "Deployment",
    "name": "api-server",
    "pod_count": 3,
    "cpu_request_cores": 0.75,
    "cpu_limit_cores": 1.5,
    "cpu_usage_cores": 0.42,
    "cpu_overcommit_pct": 100,
    "mem_request_gb": 1.0,
    "mem_limit_gb": 2.0,
    "mem_usage_gb": 0.65,
    "mem_overcommit_pct": 100
  }
]
```

## Output Formats

### Table (default)

```
2024-01-15 10:30:00 UTC
Filter: instance_type=m5.xlarge | Sort: pool=asc
NODE                PODS   CPU(req/lim/use/cap)  CPU%  MEM(req/lim/use/cap)  MEM%  EC2               TYPE        CAP  ARCH   AZ  POOL            NODECLAIM            AS        AGE   TAINTS
----                ----   --------------------  ----  --------------------  ----  ---               ----        ---  ----   --  ----            ---------            --        ---   ------
ip-10-0-1-100       12/58  3.2/6.0/2.8/4.0       70    8.5/12.0/7.2/16.0     45    i-0abc123def456  m5.xlarge   od   amd64  1a  my-pool         my-nodeclaim         karpenter 5d12h none
```

### JSON

```bash
kubectl k8i -o json
```

```json
[
  {
    "name": "ip-10-0-1-100",
    "pods_used": 12,
    "pods_max": 58,
    "cpu_request_cores": 3.2,
    "cpu_limit_cores": 6.0,
    "cpu_usage_cores": 2.8,
    "cpu_capacity_cores": 4.0,
    "cpu_load_percent": 70,
    "mem_request_gb": 8.5,
    "mem_limit_gb": 12.0,
    "mem_usage_gb": 7.2,
    "mem_capacity_gb": 16.0,
    "mem_load_percent": 45,
    "ec2_instance_id": "i-0abc123def456",
    "instance_type": "m5.xlarge",
    "capacity_type": "od",
    "architecture": "amd64",
    "zone": "1a",
    "nodepool": "my-pool",
    "nodeclaim": "my-nodeclaim",
    "autoscaler": "karpenter",
    "age": "5d12h",
    "taints": "none"
  }
]
```

### YAML

```bash
kubectl k8i -o yaml
```

```yaml
- name: ip-10-0-1-100
  pods_used: 12
  pods_max: 58
  cpu_request_cores: 3.2
  cpu_limit_cores: 6.0
  cpu_usage_cores: 2.8
  cpu_capacity_cores: 4.0
  cpu_load_percent: 70
  mem_request_gb: 8.5
  mem_limit_gb: 12.0
  mem_usage_gb: 7.2
  mem_capacity_gb: 16.0
  mem_load_percent: 45
  ec2_instance_id: i-0abc123def456
  instance_type: m5.xlarge
  capacity_type: od
  architecture: amd64
  zone: "1a"
  nodepool: my-pool
  nodeclaim: my-nodeclaim
  autoscaler: karpenter
  age: 5d12h
  taints: none
```

## Development

### Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- [golangci-lint](https://golangci-lint.run/welcome/install/)
- [goreleaser](https://goreleaser.com/install/) (optional, for local release builds)

### Build from source

```bash
git clone https://github.com/ViktorUJ/kubectl-k8i.git
cd kubectl-k8i
make build
```

### Available Make targets

| Command              | Description                                      |
|----------------------|--------------------------------------------------|
| `make build`         | Build the binary for your current platform       |
| `make install`       | Build and install to `/usr/local/bin` (or `$GOPATH/bin`) without krew |
| `make test`          | Run unit tests with race detector                |
| `make lint`          | Run golangci-lint (static analysis + security linters) |
| `make vet`           | Run `go vet` (built-in static analysis)          |
| `make security`      | Run govulncheck + gosec (vulnerability and security scan) |
| `make vulncheck`     | Check dependencies for known vulnerabilities (govulncheck) |
| `make check`         | Run all static analysis: lint + vet + security   |
| `make test-all`      | Run all checks + all test suites                 |
| `make build-all`     | Cross-compile for all 6 platforms to `dist/`     |
| `make release-local` | Local release build via GoReleaser (snapshot)     |
| `make clean`         | Remove build artifacts                           |

### Security scanning

```bash
# Check dependencies for known vulnerabilities
make vulncheck

# Full security scan (govulncheck + gosec)
make security

# All static analysis (lint + vet + security)
make check
```

Requires [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) and [gosec](https://github.com/securego/gosec):

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

### Creating a release

Releases are fully automated. Push to `main` and the CI pipeline will:

1. Run lint, tests, and build for all 6 platforms
2. Auto-tag with a patch version bump (e.g., `v0.1.0` → `v0.1.1`)
3. The new tag triggers the release workflow, which uses GoReleaser to create a GitHub Release with cross-compiled binaries and updated krew manifest

To create a release manually, push a semantic version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Requirements

- A running Kubernetes cluster accessible via `kubectl`
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server) installed for CPU/memory usage data (optional — without it, usage values show as zero)

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
