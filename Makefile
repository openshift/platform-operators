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
GO_INSTALL_OPTS ?= "-mod=readonly"
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
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths=./... output:rbac:artifacts:config=config/rbac

RBAC_LIST = rbac.authorization.k8s.io_v1_clusterrole_platform-operators-manager-role.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-metrics-reader.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-proxy-role.yaml \
	rbac.authorization.k8s.io_v1_clusterrolebinding_platform-operators-manager-rolebinding.yaml \
	rbac.authorization.k8s.io_v1_clusterrolebinding_platform-operators-proxy-rolebinding.yaml \
	rbac.authorization.k8s.io_v1_role_platform-operators-leader-election-role.yaml \
	rbac.authorization.k8s.io_v1_rolebinding_platform-operators-leader-election-rolebinding.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-rukpak-bundle-reader.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-rukpak-bundle-uploader.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_platform-operators-rukpak-core-admin.yaml \
	rbac.authorization.k8s.io_v1_clusterrolebinding_platform-operators-rukpak-core-admin.yaml

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: generate yq kustomize
	$(KUSTOMIZE) build config/default -o $(TMP_DIR)/
	ls $(TMP_DIR)

	@# Cleanup the existing manifests so no removed ones linger post generation
	rm manifests/* || true

	@# Move the vendored PlatformOperator CRD from o/api to the manifests folder
	cp $(ROOT_DIR)/vendor/github.com/openshift/api/platform/v1alpha1/platformoperators.crd.yaml manifests/0000_50_cluster-platform-operator-manager_00-platformoperator.crd.yaml
	@# TODO(tflannag): Remove this hack when https://github.com/openshift/api/pull/1282 merges.
	${YQ} '.metadata.annotations["include.release.openshift.io/self-managed-high-availability"] = "true"' -i manifests/0000_50_cluster-platform-operator-manager_00-platformoperator.crd.yaml
	${YQ} '.metadata.annotations["include.release.openshift.io/single-node-developer"] = "true"' -i manifests/0000_50_cluster-platform-operator-manager_00-platformoperator.crd.yaml
	${YQ} '.metadata.annotations["exclude.release.openshift.io/internal-openshift-hosted"] = "true"' -i manifests/0000_50_cluster-platform-operator-manager_00-platformoperator.crd.yaml

	@# Move all of the rukpak manifests into the manifests folder
	$(MV_TMP_DIR)/apiextensions.k8s.io_v1_customresourcedefinition_bundledeployments.core.rukpak.io.yaml manifests/0000_50_cluster-platform-operator-manager_00-rukpak-bundledeployments.crd.yaml
	$(MV_TMP_DIR)/apiextensions.k8s.io_v1_customresourcedefinition_bundles.core.rukpak.io.yaml manifests/0000_50_cluster-platform-operator-manager_00-rukpak-bundles.crd.yaml
	$(MV_TMP_DIR)/v1_configmap_platform-operators-rukpak-core-tls.yaml manifests/0000_50_cluster-platform-operator-manager_01-core-ca.cm.yaml
	$(MV_TMP_DIR)/v1_configmap_platform-operators-rukpak-webhook-tls.yaml manifests/0000_50_cluster-platform-operator-manager_01-webhook-ca.cm.yaml
	$(MV_TMP_DIR)/v1_serviceaccount_platform-operators-rukpak-core-admin.yaml manifests/0000_50_cluster-platform-operator-manager_01-rukpak-core-admin.sa.yaml
	$(MV_TMP_DIR)/v1_serviceaccount_platform-operators-rukpak-webhooks-admin.yaml manifests/0000_50_cluster-platform-operator-manager_01-rukpak-webhooks-admin.sa.yaml
	$(MV_TMP_DIR)/v1_service_platform-operators-rukpak-core.yaml manifests/0000_50_cluster-platform-operator-manager_02-rukpak-core.service.yaml
	$(MV_TMP_DIR)/v1_service_platform-operators-rukpak-webhook-service.yaml manifests/0000_50_cluster-platform-operator-manager_02-rukpak-webhook.service.yaml
	$(MV_TMP_DIR)/apps_v1_deployment_platform-operators-rukpak-webhooks.yaml manifests/0000_50_cluster-platform-operator-manager_04-rukpak-webhooks.deployment.yaml
	$(MV_TMP_DIR)/apps_v1_deployment_platform-operators-rukpak-core.yaml manifests/0000_50_cluster-platform-operator-manager_04-rukpak-core.deployment.yaml
	$(MV_TMP_DIR)/admissionregistration.k8s.io_v1_validatingwebhookconfiguration_platform-operators-rukpak-validating-webhook-configuration.yaml manifests/0000_50_cluster-platform-operator-manager_05-rukpak.validating-webhook-configuration.yaml

	@# Move all of the platform operators manifests into the manifests folder
	$(MV_TMP_DIR)/v1_namespace_openshift-platform-operators.yaml manifests/0000_50_cluster-platform-operator-manager_00-namespace.yaml
	$(MV_TMP_DIR)/v1_serviceaccount_platform-operators-controller-manager.yaml manifests/0000_50_cluster-platform-operator-manager_01-serviceaccount.yaml
	$(MV_TMP_DIR)/v1_service_platform-operators-controller-manager-metrics-service.yaml manifests/0000_50_cluster-platform-operator-manager_02-metricsservice.yaml
	$(MV_TMP_DIR)/apps_v1_deployment_platform-operators-controller-manager.yaml manifests/0000_50_cluster-platform-operator-manager_06-deployment.yaml
	$(MV_TMP_DIR)/config.openshift.io_v1_clusteroperator_platform-operators-aggregated.yaml manifests/0000_50_cluster-platform-operator-manager_07-aggregated-clusteroperator.yaml

	@# cluster-platform-operator-manager rbacs
	rm -f manifests/0000_50_cluster-platform-operator-manager_03_rbac.yaml
	for rbac in $(RBAC_LIST); do \
		cat $(TMP_DIR)/$${rbac} >> manifests/0000_50_cluster-platform-operator-manager_03_rbac.yaml ;\
		echo '---' >> manifests/0000_50_cluster-platform-operator-manager_03_rbac.yaml ;\
	done

.PHONY: lint
lint: ## Run golangci-lint linter checks.
lint: golangci-lint
	@# Set the golangci-lint cache directory to a directory that's
	@# writable in downstream CI.
	GOLANGCI_LINT_CACHE=/tmp/golangci-cache $(GOLANGCI_LINT) run

UNIT_TEST_DIRS=$(shell go list ./... | grep -v /test/)
.PHONY: unit
unit: generate envtest ## Run unit tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -count=1 -short $(UNIT_TEST_DIRS)

.PHONY: e2e
e2e: deploy test-e2e

.PHONY: test-e2e
FOCUS := $(if $(TEST),-v -focus "$(TEST)")
JUNIT_REPORT := $(if $(ARTIFACT_DIR), -output-dir $(ARTIFACT_DIR) -junit-report junit_e2e.xml)
test-e2e: ginkgo ## Run e2e tests
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
kind-load: build-container kind
	$(KIND) load docker-image $(IMG)

.PHONY: kind-cluster
kind-cluster: kind
	$(KIND) get clusters | grep $(KIND_CLUSTER_NAME) || $(KIND) create cluster --name $(KIND_CLUSTER_NAME)

.PHONY: run
run: build-container
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GINKGO ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
KIND ?= $(LOCALBIN)/kind
YQ ?= $(LOCALBIN)/yq

## Tool Versions
KUSTOMIZE_VERSION ?= v4.2.0
CONTROLLER_TOOLS_VERSION ?= v0.9.0
ENVTEST_VERSION ?= latest
GINKGO_VERSION ?= v2.1.4
GOLANGCI_LINT_VERSION ?= v1.47.3
KIND_VERSION ?= v0.14.0
YQ_VERSION ?= latest

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	rm -f $(KUSTOMIZE)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.PHONY: ginkgo
ginkgo: $(GINKGO)
$(GINKGO): $(LOCALBIN) ## Download ginkgo locally if necessary.
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN) ## Download golangci-lint locally if necessary.
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) github.com/mikefarah/yq/v4@$(YQ_VERSION)
