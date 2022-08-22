
# Using PWD is not guaranteed to be the directory of the Makefile. Use these instead:
MAKE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MAKE_DIR := $(shell dirname $(MAKE_PATH))

# Image URL to use all building/pushing image targets
REGISTRY ?= docker.io
IMAGE_NAME ?= erwinvaneyk/cluster-api-external-controller
IMG ?= $(REGISTRY)/$(IMAGE_NAME)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
KUSTOMIZE_VERSION ?= v4.5.2

LOCALBIN ?= $(MAKE_DIR)/bin
KUSTOMIZE = $(LOCALBIN)/kustomize
CONTROLLER_GEN = $(LOCALBIN)/controller-gen

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

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

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: generate controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: verify
verify: ## Run all static analysis checks.
	# Check if codebase is formatted.
	@bash -c "[ -z \"$$(gofmt -l . | grep -v '^vendor')\" ] && echo 'OK' || (echo 'ERROR: files are not formatted:' && gofmt -l . | grep -v '^vendor' && echo -e \"\nRun 'make format' or manually fix the formatting issues.\n\" && false)"

	# Run static checks on codebase.
	go vet .

.PHONY: format
format: ## Run all formatters.
	# Format the Go codebase.
	gofmt -w -s .

	# Format the go.mod file.
	go mod tidy

.PHONY: verify-generate
verify-generate: ## Verify that all code generation is up to date
	hack/verify-codegen.sh

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate manifests verify ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	# Note: explicitly setting the GOARCH to fix the bugged setup-envtest.sh for Apple M1.
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; export GOARCH=amd64; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

.PHONY: clean
clean: docker-clean ## Clean up build-generated artifacts.
	rm -rf bin/
	rm -rf build/

##@ Build

build: generate verify ## Build manager binary.
	go build -o bin/manager main.go

run: manifests generate ## Run a controller from your host.
	go run ./main.go

##@ Docker

docker-build:  ## Build docker image with the manager.
	DOCKER_BUILDKIT=1 docker build -t ${IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docker-clean
docker-clean:
	docker rmi ${IMG} || true

##@ Deployment

install: manifests $(KUSTOMIZE)  ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests $(KUSTOMIZE)  ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests $(KUSTOMIZE)  ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -
	git restore config/default/manager_image_patch.yaml || true # Clean up changes made by kustomize edit.

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

.PHONY: manifest-build
manifest-build: kustomize
	mkdir -p build
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMG)
	$(KUSTOMIZE) build config/default > $(MANIFEST_BUILD_PATH)
	git restore config/default/manager_image_patch.yaml || true # Clean up changes made by kustomize edit.
	
.PHONY: tools
tools: controller-gen kustomize

controller-gen: $(LOCALBIN) ## Download controller-gen locally if necessary.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1

$(LOCALBIN): ## Ensure that the directory exists
	mkdir -p $(LOCALBIN)


.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	rm -f $(KUSTOMIZE)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)


##@ Release commands

RELEASE_TAG := $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR := build/releases

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMG):$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests
	#$(MAKE) release-templates

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMG):$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	kustomize build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

.PHONY: release-manifests-clusterctl
release-manifests-clusterctl: ## Create the releases directory to conform with clusterctl provider contract for a local provider
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	$(MAKE) release-manifests RELEASE_DIR=$(RELEASE_DIR)/infrastructure-external/latest RELEASE_TAG=latest
	$(MAKE) release-manifests RELEASE_DIR=$(RELEASE_DIR)/infrastructure-external/$(RELEASE_TAG)

.PHONY: release-templates
release-templates: $(RELEASE_DIR)
	cp templates/cluster-template* $(RELEASE_DIR)/

.PHONY: release-staging
release-staging: ## Builds and push container images to the staging registry.
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-alias-tag

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"$(MANIFEST_IMG)"'@' ./config/default/manager_image_patch.yaml
	rm -f ./config/default/manager_image_patch.yaml-e # Needed for MacOS sed

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml
	rm -f ./config/default/manager_pull_policy.yaml-e # Needed for MacOS sed

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

##@ Kind commands

KIND_CLUSTER_NAME=kind
KIND_IMG=cluster-api-external-controller:kind
KIND_CONTROLLER_NAMESPACE=capc-system
KIND_KUBECONFIG_PATH=build/kind-$(KIND_CLUSTER_NAME).kubeconfig

.PHONY: kind-kubeconfig
kind-kubeconfig:
	mkdir -p build
	kind get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KIND_KUBECONFIG_PATH)

.PHONY: kind-deploy
kind-deploy: docker-build kind-kubeconfig ## Build and deploy an image into a local KinD cluster
	# Verify that the "kind" cluster exists
	kind get clusters | grep -q $(KIND_CLUSTER_NAME)
	# Rename the image to avoid conflicts
	docker tag $(IMG) $(KIND_IMG)
	# Alternative 'kind load', because it is broken for Apple M1 in some cases.
	docker save $(KIND_IMG) | docker exec --privileged -i $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images import --all-platforms -
	# Update the deployment
	$(MAKE) deploy KUBECONFIG=$(KIND_KUBECONFIG_PATH) IMG=$(KIND_IMG)
	# Recreate the pods to ensure that the newer image is used
	kubectl --kubeconfig=$(KIND_KUBECONFIG_PATH) -n $(KIND_CONTROLLER_NAMESPACE) delete pod --all || true
