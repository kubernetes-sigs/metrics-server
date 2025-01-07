# Common User-Settable Flags
# --------------------------
REGISTRY?=gcr.io/k8s-staging-metrics-server
ARCH?=amd64
OS?=linux
BINARY_NAME?=metrics-server-$(OS)-$(ARCH)

ifeq ($(OS),windows)
BINARY_NAME:=$(BINARY_NAME).exe
endif

OUTPUT_DIR?=_output

# Release variables
# ------------------
GIT_COMMIT?=$(shell git rev-parse "HEAD^{commit}" 2>/dev/null)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Consts
# ------
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x
export DOCKER_CLI_EXPERIMENTAL=enabled

ALL_BINARIES_PLATFORMS= $(addprefix linux/,$(ALL_ARCHITECTURES)) \
						darwin/amd64 \
						darwin/arm64 \
						windows/amd64 \
						windows/arm64

# Tools versions
# --------------
GOLANGCI_VERSION:=1.61.0

# Computed variables
# ------------------
GOPATH:=$(shell go env GOPATH)
REPO_DIR:=$(shell pwd)

.PHONY: all
all: metrics-server

# Build Rules
# -----------

SRC_DEPS=$(shell find pkg cmd -type f -name "*.go") go.mod go.sum
CHECKSUM=$(shell md5sum $(SRC_DEPS) | md5sum | awk '{print $$1}')
PKG:=k8s.io/client-go/pkg
VERSION_LDFLAGS:=-X $(PKG)/version.gitVersion=$(GIT_TAG) -X $(PKG)/version.gitCommit=$(GIT_COMMIT) -X $(PKG)/version.buildDate=$(BUILD_DATE)
LDFLAGS:=-w $(VERSION_LDFLAGS)

metrics-server:
	OUTPUT_DIR=. BINARY_NAME=$@ $(MAKE) build

.PHONY: build
build: $(SRC_DEPS)
	@mkdir -p $(OUTPUT_DIR)
	GOARCH=$(ARCH) GOOS=$(OS) CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags "$(LDFLAGS)" -o "$(OUTPUT_DIR)/$(BINARY_NAME)" sigs.k8s.io/metrics-server/cmd/metrics-server

.PHONY: build-all
build-all:
	@for platform in $(ALL_BINARIES_PLATFORMS); do \
		OS="$${platform%/*}" ARCH="$${platform#*/}" $(MAKE) build; \
	done

# Image Rules
# -----------

CONTAINER_ARCH_TARGETS=$(addprefix container-,$(ALL_ARCHITECTURES))

.PHONY: container
container:
	# Pull base image explicitly. Keep in sync with Dockerfile, otherwise
	# GCB builds will start failing.
	docker pull golang:1.22.5
	docker build -t $(REGISTRY)/metrics-server-$(ARCH):$(CHECKSUM) --build-arg ARCH=$(ARCH) --build-arg GIT_TAG=$(GIT_TAG) --build-arg GIT_COMMIT=$(GIT_COMMIT) .

.PHONY: container-all
container-all: $(CONTAINER_ARCH_TARGETS);

.PHONY: $(CONTAINER_ARCH_TARGETS)
$(CONTAINER_ARCH_TARGETS): container-%:
	ARCH=$* $(MAKE) container

# Official Container Push Rules
# -----------------------------

PUSH_ARCH_TARGETS=$(addprefix push-,$(ALL_ARCHITECTURES))

.PHONY: push
push: container
	docker tag $(REGISTRY)/metrics-server-$(ARCH):$(CHECKSUM) $(REGISTRY)/metrics-server-$(ARCH):$(GIT_TAG)
	docker push $(REGISTRY)/metrics-server-$(ARCH):$(GIT_TAG)

.PHONY: push-all
push-all: $(PUSH_ARCH_TARGETS) push-multi-arch;

.PHONY: $(PUSH_ARCH_TARGETS)
$(PUSH_ARCH_TARGETS): push-%:
	ARCH=$* $(MAKE) push

.PHONY: push-multi-arch
push-multi-arch:
	docker manifest create --amend $(REGISTRY)/metrics-server:$(GIT_TAG) $(shell echo $(ALL_ARCHITECTURES) | sed -e "s~[^ ]*~$(REGISTRY)/metrics-server\-&:$(GIT_TAG)~g")
	@for arch in $(ALL_ARCHITECTURES); do docker manifest annotate --arch $${arch} $(REGISTRY)/metrics-server:$(GIT_TAG) $(REGISTRY)/metrics-server-$${arch}:${GIT_TAG}; done
	docker manifest push --purge $(REGISTRY)/metrics-server:$(GIT_TAG)

# Release rules
# -------------

.PHONY: release-tag
release-tag:
	git tag $(GIT_TAG)
	git push origin $(GIT_TAG)

.PHONY: release-manifests
release-manifests:
	mkdir -p $(OUTPUT_DIR)
	kubectl kustomize manifests/overlays/release > $(OUTPUT_DIR)/components.yaml
	kubectl kustomize manifests/overlays/release-ha > $(OUTPUT_DIR)/high-availability.yaml
	kubectl kustomize manifests/overlays/release-ha-1.21+ > $(OUTPUT_DIR)/high-availability-1.21+.yaml


# fuzz tests
# ----------

.PHONY: test-fuzz
test-fuzz:
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race -fuzz=Fuzz_decodeBatchPrometheusFormat -fuzztime 900s -timeout 10s ./pkg/scraper/client/resource/
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race -fuzz=Fuzz_decodeBatchRandom -fuzztime 900s -timeout 10s ./pkg/scraper/client/resource/
# Unit tests
# ----------
.PHONY: test-unit
test-unit:
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race ./pkg/... ./cmd/...

# Benchmarks
# ----------

HAS_BENCH_STORAGE=$(wildcard ./$(OUTPUT_DIR)/bench_storage.txt)

.PHONY: bench-storage
bench-storage: benchstat
	@mkdir -p $(OUTPUT_DIR)
ifneq ("$(HAS_BENCH_STORAGE)","")
	@mv $(OUTPUT_DIR)/bench_storage.txt $(OUTPUT_DIR)/bench_storage.old.txt
endif
	@go test ./pkg/storage/ -bench=. -run=^$ -benchmem -count 5 -timeout 1h | tee $(OUTPUT_DIR)/bench_storage.txt
ifeq ("$(HAS_BENCH_STORAGE)","")
	@cp $(OUTPUT_DIR)/bench_storage.txt $(OUTPUT_DIR)/bench_storage.old.txt
endif
	@echo
	@echo 'Comparing versus previous run. When optimizing copy everything below this line and include in PR description.'
	@echo
	@benchstat $(OUTPUT_DIR)/bench_storage.old.txt $(OUTPUT_DIR)/bench_storage.txt

HAS_BENCHSTAT:=$(shell command -v benchstat)
.PHONY: benchstat
benchstat:
ifndef HAS_BENCHSTAT
	@go install -mod=readonly -modfile=scripts/go.mod golang.org/x/perf/cmd/benchstat
endif

# Image tests
# ------------

.PHONY: test-image
test-image: container
	IMAGE=$(REGISTRY)/metrics-server-$(ARCH):$(CHECKSUM) EXPECTED_ARCH=$(ARCH) EXPECTED_VERSION=$(GIT_TAG) ./test/test-image.sh

.PHONY: test-image-all
test-image-all:
	@for arch in $(ALL_ARCHITECTURES); do ARCH=$${arch} $(MAKE) test-image; done

# E2e tests
# -----------

.PHONY: test-e2e
test-e2e: test-e2e-1.29

.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.29 test-e2e-1.28 test-e2e-1.27

.PHONY: test-e2e-1.29
test-e2e-1.29:
	NODE_IMAGE=kindest/node:v1.29.0@sha256:eaa1450915475849a73a9227b8f201df25e55e268e5d619312131292e324d570 KIND_CONFIG="${PWD}/test/kind-config-with-sidecar-containers.yaml" ./test/test-e2e.sh

.PHONY: test-e2e-1.28
test-e2e-1.28:
	NODE_IMAGE=kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31 KIND_CONFIG="${PWD}/test/kind-config-with-sidecar-containers.yaml" ./test/test-e2e.sh

.PHONY: test-e2e-1.27
test-e2e-1.27:
	NODE_IMAGE=kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72 ./test/test-e2e.sh


.PHONY: test-e2e-ha
test-e2e-ha:
	SKAFFOLD_PROFILE="test-ha" $(MAKE) test-e2e

.PHONY: test-e2e-ha-all
test-e2e-ha-all:
	SKAFFOLD_PROFILE="test-ha" $(MAKE) test-e2e-all

.PHONY: test-e2e-helm
test-e2e-helm:
	SKAFFOLD_PROFILE="helm" $(MAKE) test-e2e

.PHONY: test-e2e-helm-all
test-e2e-helm-all:
	SKAFFOLD_PROFILE="helm" $(MAKE) test-e2e-all

# Static analysis
# ---------------

.PHONY: verify
verify: verify-licenses verify-lint verify-toc verify-deps verify-scripts-deps verify-generated verify-structured-logging

.PHONY: update
update: update-licenses update-lint update-toc update-deps update-generated

# License
# -------

HAS_ADDLICENSE:=$(shell command -v addlicense)
.PHONY: verify-licenses
verify-licenses:addlicense
	find -type f -name "*.go" ! -path "*/vendor/*" | xargs $(GOPATH)/bin/addlicense -check || (echo 'Run "make update"' && exit 1)

.PHONY: update-licenses
update-licenses: addlicense
	find -type f -name "*.go" ! -path "*/vendor/*" | xargs $(GOPATH)/bin/addlicense -c "The Kubernetes Authors."

.PHONY: addlicense
addlicense:
ifndef HAS_ADDLICENSE
	go install -mod=readonly -modfile=scripts/go.mod github.com/google/addlicense
endif

# Lint
# ----

.PHONY: verify-lint
verify-lint: golangci
	$(GOPATH)/bin/golangci-lint run --timeout 10m || (echo 'Run "make update"' && exit 1)

.PHONY: update-lint
update-lint: golangci
	$(GOPATH)/bin/golangci-lint run --fix

HAS_GOLANGCI_VERSION:=$(shell $(GOPATH)/bin/golangci-lint version --format=short > /dev/null 2>&1)
.PHONY: golangci
golangci:
ifneq ($(HAS_GOLANGCI_VERSION), $(GOLANGCI_VERSION))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v$(GOLANGCI_VERSION)
endif

# Table of Contents
# -----------------

docs_with_toc=FAQ.md KNOWN_ISSUES.md

.PHONY: verify-toc
verify-toc: mdtoc $(docs_with_toc)
	$(GOPATH)/bin/mdtoc --inplace --dryrun $(docs_with_toc)

.PHONY: update-toc
update-toc: mdtoc $(docs_with_toc)
	$(GOPATH)/bin/mdtoc --inplace $(docs_with_toc)

HAS_MDTOC:=$(shell command -v mdtoc)
.PHONY: mdtoc
mdtoc:
ifndef HAS_MDTOC
	go install -mod=readonly -modfile=scripts/go.mod sigs.k8s.io/mdtoc
endif

# Structured Logging
# -----------------

.PHONY: verify-structured-logging
verify-structured-logging: logcheck
	$(GOPATH)/bin/logcheck ./... || (echo 'Fix structured logging' && exit 1)

HAS_LOGCHECK:=$(shell command -v logcheck)
.PHONY: logcheck
logcheck:
ifndef HAS_LOGCHECK
	go install -mod=readonly -modfile=scripts/go.mod sigs.k8s.io/logtools/logcheck
endif

# Dependencies
# ------------

.PHONY: update-deps
update-deps:
	go mod tidy
	cd scripts && go mod tidy

.PHONY: verify-deps
verify-deps:
	go mod verify
	go mod tidy
	@git diff --exit-code -- go.mod go.sum

.PHONY: verify-scripts-deps
verify-scripts-deps:
	make -C scripts -f ../Makefile verify-deps

# Generated
# ---------

generated_files=pkg/api/generated/openapi/zz_generated.openapi.go

.PHONY: verify-generated
verify-generated: update-generated
	@git diff --exit-code -- $(generated_files)

.PHONY: update-generated
update-generated:
	# pkg/api/generated/openapi/zz_generated.openapi.go
	go install -mod=readonly -modfile=scripts/go.mod k8s.io/kube-openapi/cmd/openapi-gen
	$(GOPATH)/bin/openapi-gen -i k8s.io/metrics/pkg/apis/metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version -p pkg/api/generated/openapi/ -O zz_generated.openapi -o $(REPO_DIR) -h $(REPO_DIR)/scripts/boilerplate.go.txt -r /dev/null

# Deprecated
# ----------

# Remove when CI is migrated
lint: verify
test-version: test-image-all

# Clean
# -----

.PHONY: clean
clean:
	rm -rf $(OUTPUT_DIR)
