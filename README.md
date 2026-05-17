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
