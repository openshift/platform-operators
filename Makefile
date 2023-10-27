SHELL := /bin/bash

ROOT_DIR := $(patsubst %/,%,$(dir $(realpath $(lastword $(MAKEFILE_LIST)))))

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.22
BIN_DIR := bin
CONTAINER_RUNTIME ?= docker
KUBECTL ?= kubectl
KIND_CLUSTER_NAME ?= kind
TMP_DIR := $(shell mktemp -d -t manifests-$(date +%Y-%m-%d-%H-%M-%S)-XXXXXXXXXX)
MV_TMP_DIR := mv $(TMP_DIR)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the unit target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# bingo manages consistent tooling versions for things like kind, kustomize, etc.
include .bingo/Variables.mk

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths=./... output:rbac:artifacts:config=config/rbac

RBAC_LIST = rbac.authorization.k8s.io_v1_clusterrole_platform-operators-manager-role.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-metrics-reader.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-proxy-role.yaml \
	rbac.authorization.k8s.io_v1_clusterrolebinding_platform-operators-manager-rolebinding.yaml \
	rbac.authorization.k8s.io_v1_clusterrolebinding_platform-operators-proxy-rolebinding.yaml \
	rbac.authorization.k8s.io_v1_role_platform-operators-leader-election-role.yaml \
	rbac.authorization.k8s.io_v1_rolebinding_platform-operators-leader-election-rolebinding.yaml

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: generate $(YQ) $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default -o $(TMP_DIR)/
	ls $(TMP_DIR)

	@# Cleanup the existing manifests so no removed ones linger post generation
	rm manifests/*.yaml || true

	@# Move the vendored PlatformOperator CRD from o/api to the manifests folder
	cp $(ROOT_DIR)/vendor/github.com/openshift/api/platform/v1alpha1/platformoperators.crd.yaml manifests/0000_50_cluster-platform-operator-manager_00-platformoperator.crd.yaml

	@# Move all of the platform operators manifests into the manifests folder
	$(MV_TMP_DIR)/v1_namespace_openshift-platform-operators.yaml manifests/0000_50_cluster-platform-operator-manager_00-namespace.yaml
	$(MV_TMP_DIR)/v1_serviceaccount_platform-operators-controller-manager.yaml manifests/0000_50_cluster-platform-operator-manager_01-serviceaccount.yaml
	$(MV_TMP_DIR)/v1_service_platform-operators-controller-manager-metrics-service.yaml manifests/0000_50_cluster-platform-operator-manager_02-metricsservice.yaml
	$(MV_TMP_DIR)/apps_v1_deployment_platform-operators-controller-manager.yaml manifests/0000_50_cluster-platform-operator-manager_06-deployment.yaml
	$(MV_TMP_DIR)/config.openshift.io_v1_clusteroperator_platform-operators-aggregated.yaml manifests/0000_50_cluster-platform-operator-manager_07-aggregated-clusteroperator.yaml
	sed -i '/^  namespace:/d' manifests/0000_50_cluster-platform-operator-manager_07-aggregated-clusteroperator.yaml

	@# cluster-platform-operator-manager rbacs
	rm -f manifests/0000_50_cluster-platform-operator-manager_03_rbac.yaml
	for rbac in $(RBAC_LIST); do \
		cat $(TMP_DIR)/$${rbac} >> manifests/0000_50_cluster-platform-operator-manager_03_rbac.yaml ;\
		echo '---' >> manifests/0000_50_cluster-platform-operator-manager_03_rbac.yaml ;\
	done

.PHONY: lint
lint: ## Run golangci-lint linter checks.
lint: $(GOLANGCI_LINT)
	@# Set the golangci-lint cache directory to a directory that's
	@# writable in downstream CI.
	GOLANGCI_LINT_CACHE=/tmp/golangci-cache $(GOLANGCI_LINT) run

UNIT_TEST_DIRS=$(shell go list ./... | grep -v /test/)
ENVTEST_OPTS ?= $(if $(OPENSHIFT_CI),--bin-dir=/tmp)
.PHONY: unit
unit: generate $(SETUP_ENVTEST) ## Run unit tests.
	KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) $(ENVTEST_OPTS) -p path)" go test -count=1 -short $(UNIT_TEST_DIRS)

.PHONY: e2e
e2e: deploy test-e2e

.PHONY: test-e2e
FOCUS := $(if $(TEST),-v -focus "$(TEST)")
JUNIT_REPORT := $(if $(ARTIFACT_DIR), -output-dir $(ARTIFACT_DIR) -junit-report junit_e2e.xml)
test-e2e: $(GINKGO) ## Run e2e tests
	$(GINKGO) -trace -progress $(JUNIT_REPORT) $(FOCUS) test/e2e

.PHONY: verify
verify: vendor manifests
	git diff --exit-code

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

##@ Build

.PHONY: build
build: ## Build manager binary.
	CGO_ENABLED=0 go build -o bin/manager ./cmd/...

.PHONY: build-container
build-container: build ## Builds provisioner container image locally
	$(CONTAINER_RUNTIME) build -f Dockerfile -t $(IMG) $(BIN_DIR)

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

deploy: export KUBECTL=oc
deploy:
	$(ROOT_DIR)/hack/apply-feature-gate.sh

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: kind-load
kind-load: build-container $(KIND)
	$(KIND) load docker-image $(IMG)

.PHONY: kind-cluster
kind-cluster: $(KIND)
	$(KIND) get clusters | grep $(KIND_CLUSTER_NAME) || $(KIND) create cluster --name $(KIND_CLUSTER_NAME)

.PHONY: run
run: build-container $(KUSTOMIZE)
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -
