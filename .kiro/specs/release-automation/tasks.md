# Implementation Plan: Release Automation

## Overview

Implement CI/CD infrastructure for kubectl-k8i: a version package for ldflags, GoReleaser and golangci-lint configs, a Makefile for local development, a CI workflow for linting/testing/building on push and PR, and a release workflow triggered by semantic version tags that cross-compiles, publishes a GitHub Release, and auto-updates the krew manifest.

## Tasks

- [x] 1. Create version package
  - [x] 1.1 Create `pkg/version/version.go`
    - Create the `pkg/version` directory and `version.go` file
    - Define `var Version = "dev"` with a comment explaining it is set at build time via ldflags
    - The default value `"dev"` is used for local builds without ldflags
    - _Requirements: 12.1, 12.2_

- [x] 2. Create GoReleaser and golangci-lint configurations
  - [x] 2.1 Create `.goreleaser.yaml`
    - Use GoReleaser v2 format (`version: 2`)
    - Define a single build with id `kubectl-k8i`, main `./cmd/kubectl-k8i`, binary `kubectl-k8i`
    - Set `CGO_ENABLED=0` for static linking
    - Set ldflags: `-s -w -X github.com/kubectl-k8i/pkg/version.Version={{.Version}}`
    - Define 6 target platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64
    - Configure archives: tar.gz default, zip override for windows, include LICENSE, name template `kubectl-k8i_{{ .Os }}_{{ .Arch }}`
    - Configure checksum with `checksums.txt` and sha256 algorithm
    - Configure changelog with ascending sort and exclusion filters for docs/chore/ci/test prefixes
    - Configure release section with GitHub owner and repo name
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3, 6.4, 7.1, 7.2, 8.1, 8.2, 8.3, 10.1, 10.2, 10.3, 10.4, 10.5, 12.1_

  - [x] 2.2 Create `.golangci.yaml`
    - Set run timeout to 5 minutes
    - Enable linters: errcheck, govet, staticcheck, unused, ineffassign, gosimple
    - Set `exclude-use-default: false` in issues section
    - _Requirements: 1.1_

- [x] 3. Create Makefile for local development
  - [x] 3.1 Create `Makefile`
    - Set `.DEFAULT_GOAL := build`
    - Define `VERSION ?= dev` variable and `LDFLAGS` using the version package path
    - Implement `build` target: `go build -ldflags "$(LDFLAGS)" -o kubectl-k8i ./cmd/kubectl-k8i/`
    - Implement `test` target: `go test -race ./pkg/...`
    - Implement `lint` target: `golangci-lint run`
    - Implement `test-integration` target: `go test -tags=integration ./test/integration/...`
    - Implement `test-e2e` target: `go test ./test/e2e/...`
    - Implement `test-bench` target: `go test -bench=. ./test/benchmark/...`
    - Implement `release-local` target: `goreleaser release --snapshot --clean`
    - Implement `clean` target: remove `kubectl-k8i` binary and `dist/` directory
    - Mark all targets as `.PHONY`
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8, 11.9, 12.2_

- [x] 4. Checkpoint — Verify local tooling
  - Run `make build` to verify compilation with ldflags
  - Run `make lint` to verify golangci-lint configuration
  - Run `make test` to verify test execution
  - Run `goreleaser check` to validate `.goreleaser.yaml`
  - Ensure all commands pass, ask the user if questions arise.

- [x] 5. Create CI workflow
  - [x] 5.1 Create `.github/workflows/ci.yaml`
    - Set workflow name to a descriptive CI name
    - Configure triggers: `push` to `main` branch, `pull_request` to `main` branch
    - Set `permissions: contents: read` (minimal permissions)
    - Define a single job running on `ubuntu-latest`
    - Step 1: Checkout using `actions/checkout` pinned by SHA commit (v4), with version comment
    - Step 2: Setup Go using `actions/setup-go` pinned by SHA commit (v5), with `go-version-file: go.mod`
    - Step 3: Lint using `golangci/golangci-lint-action` pinned by SHA commit (v6)
    - Step 4: Test with `run: go test -race ./pkg/...`
    - Step 5: Build with `run: go build ./cmd/kubectl-k8i/`
    - All third-party actions must be pinned by full SHA with a version comment after the SHA
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 13.1, 13.4_

- [x] 6. Create Release workflow
  - [x] 6.1 Create `.github/workflows/release.yaml`
    - Set workflow name to a descriptive release name
    - Configure trigger: `push` tags matching `v*.*.*`
    - Set `permissions: contents: write` (for creating release and pushing commit)
    - Define a single job running on `ubuntu-latest`
    - Step 1: Checkout using `actions/checkout` pinned by SHA (v4), with `fetch-depth: 0` and `fetch-tags: true`
    - Step 2: Setup Go using `actions/setup-go` pinned by SHA (v5), with `go-version-file: go.mod`
    - Step 3: Run GoReleaser using `goreleaser/goreleaser-action` pinned by SHA (v6), with `args: release --clean`
    - Step 4: Update krew manifest — inline bash script that:
      - Extracts version from `GITHUB_REF_NAME`
      - Parses `dist/checksums.txt` to get SHA256 hashes for all 6 platform archives
      - Updates `sha256` fields in `krew-manifest.yaml` for each platform using `sed`
      - Updates `uri` fields with the correct version in download URLs
      - Updates `spec.version` to the tag value
    - Step 5: Commit and push updated `krew-manifest.yaml` to main
      - Configure git user for the commit
      - Create commit with descriptive message including the version
      - Push to main branch
    - All third-party actions must be pinned by full SHA with a version comment after the SHA
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 7.3, 8.1, 8.2, 8.3, 9.1, 9.2, 9.3, 9.4, 13.2, 13.3_

- [x] 7. Final checkpoint — Verify all configurations
  - Run `make build` to verify build still works
  - Run `make lint` to verify linting passes
  - Run `make test` to verify tests pass
  - Run `goreleaser check` to validate GoReleaser config
  - Review all YAML files for correct syntax and pinned SHA references
  - Ensure all checks pass, ask the user if questions arise.

## Notes

- No property-based tests are included — this feature consists entirely of configuration files and a single Go variable, which are best validated by static checks (`goreleaser check`, `make lint`) and integration testing (first real release).
- All GitHub Actions are pinned by SHA commit hash for supply-chain security (Requirement 13).
- The version package uses `"dev"` as default so local builds without ldflags still produce a usable binary.
- The krew manifest update uses inline `sed` to avoid external tool dependencies.
- Each task references specific requirements for traceability.
