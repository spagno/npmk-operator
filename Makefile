IMG ?= npmk-operator:latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Tool versions - keep in sync with CI
GOLANGCI_LINT_VERSION ?= v2.12.2

# Local bin directory for tools
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Tool binaries
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: test
test: ## Run unit tests
	go test ./... -cover

.PHONY: build
build: ## Build the operator binary
	go build -o bin/manager ./main.go

.PHONY: lint
lint: golangci-lint ## Run linters
	$(GOLANGCI_LINT) run
	go vet ./...

##@ Build

.PHONY: docker-build
docker-build: ## Build container image
	docker build --build-arg VERSION=$(VERSION) -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push container image
	docker push ${IMG}

##@ Deployment

.PHONY: deploy
deploy: ## Deploy operator to cluster (set IMG=your-registry/npmk-operator:tag)
	@cd config/default && \
		cp kustomization.yaml kustomization.yaml.bak && \
		sed -i 's|newName: .*|newName: $(shell echo ${IMG} | cut -d: -f1)|' kustomization.yaml && \
		sed -i 's|newTag: .*|newTag: $(shell echo ${IMG} | cut -d: -f2)|' kustomization.yaml && \
		kubectl kustomize . | kubectl apply -f - && \
		mv kustomization.yaml.bak kustomization.yaml

.PHONY: undeploy
undeploy: ## Remove operator from cluster
	kubectl kustomize config/default | kubectl delete -f -

.PHONY: deploy-prometheus
deploy-prometheus: ## Deploy ServiceMonitor and metrics Service for Prometheus
	kubectl kustomize config/prometheus | kubectl apply -f -

.PHONY: undeploy-prometheus
undeploy-prometheus: ## Remove ServiceMonitor and metrics Service
	kubectl kustomize config/prometheus | kubectl delete -f -

##@ Manifests

.PHONY: manifests
manifests: ## Generate CRD manifests
	@echo "Generating manifests..."

##@ CI

.PHONY: ci
ci: lint test build ## Run all CI checks locally

.PHONY: coverage
coverage: ## Run tests with coverage report
	go test ./... -v -coverprofile=coverage.txt
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: vulncheck
vulncheck: ## Run Go vulnerability check
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

##@ Tools

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary
$(GOLANGCI_LINT): $(LOCALBIN)
	@test -s $(LOCALBIN)/golangci-lint && $(LOCALBIN)/golangci-lint version --format short | grep -q $(subst v,,$(GOLANGCI_LINT_VERSION)) || \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)
