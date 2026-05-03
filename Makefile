.DEFAULT_GOAL := build

VERSION ?= dev
LDFLAGS := -X github.com/kubectl-k8i/pkg/version.Version=$(VERSION)
BINARY := kubectl-k8i
INSTALL_DIR := $(if $(GOPATH),$(GOPATH)/bin,/usr/local/bin)
DOCKER_CI_IMAGE := kubectl-k8i-ci
export PATH := /usr/local/go/bin:$(HOME)/go/bin:$(PATH)

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build install test lint vet security vulncheck test-integration test-e2e test-bench test-all check build-all release-local clean \
        docker-lint docker-vet docker-vulncheck docker-security docker-test docker-build docker-build-all docker-check

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/kubectl-k8i/

install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	./$(BINARY) completion > $(INSTALL_DIR)/kubectl_complete-k8i
	chmod +x $(INSTALL_DIR)/kubectl_complete-k8i

test:
	go test -race ./pkg/...

lint:
	golangci-lint run

vet:
	go vet ./...

security:
	@echo "==> Running govulncheck (vulnerable dependencies)..."
	govulncheck ./...
	@echo "==> Running gosec (security static analysis)..."
	gosec -quiet ./...

vulncheck:
	govulncheck ./...

test-integration:
	go test -tags=integration ./test/integration/...

test-e2e:
	go test ./test/e2e/...

test-bench:
	go test -bench=. ./test/benchmark/...

test-all: lint vet security test test-integration test-e2e

check: lint vet security
	@echo "All static analysis checks passed."

build-all:
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)_$${os}_$${arch}$${ext} ./cmd/kubectl-k8i/; \
	done

release-local:
	goreleaser release --snapshot --clean

clean:
	rm -f $(BINARY)
	rm -rf dist/

## ---- Docker targets (run all checks in a container with correct Go version) ----

docker-lint:
	docker build -f Dockerfile.ci --target lint -t $(DOCKER_CI_IMAGE):lint .

docker-vet:
	docker build -f Dockerfile.ci --target vet -t $(DOCKER_CI_IMAGE):vet .

docker-vulncheck:
	docker build -f Dockerfile.ci --target vulncheck -t $(DOCKER_CI_IMAGE):vulncheck .

docker-security:
	docker build -f Dockerfile.ci --target security -t $(DOCKER_CI_IMAGE):security .

docker-test:
	docker build -f Dockerfile.ci --target test -t $(DOCKER_CI_IMAGE):test .

docker-build:
	docker build -f Dockerfile.ci --target build -t $(DOCKER_CI_IMAGE):build .

docker-build-all:
	docker build -f Dockerfile.ci --target build-all -t $(DOCKER_CI_IMAGE):build-all .

docker-check:
	docker build -f Dockerfile.ci --target check-all -t $(DOCKER_CI_IMAGE):check-all .

docker-ci:
	docker build -f Dockerfile.ci --target ci -t $(DOCKER_CI_IMAGE):ci .
