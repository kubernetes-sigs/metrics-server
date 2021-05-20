# Common User-Settable Flags
# --------------------------
REGISTRY?=gcr.io/k8s-staging-metrics-server
ARCH?=amd64

# Release variables
# ------------------
GIT_COMMIT?=$(shell git rev-parse "HEAD^{commit}" 2>/dev/null)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Consts
# ------
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x
export DOCKER_CLI_EXPERIMENTAL=enabled

# Computed variables
# ------------------
HAS_GOLANGCI:=$(shell which golangci-lint)
GOPATH:=$(shell go env GOPATH)
REPO_DIR:=$(shell pwd)
LDFLAGS=-w $(VERSION_LDFLAGS)

.PHONY: all
all: metrics-server

# Build Rules
# -----------

SRC_DEPS=$(shell find pkg cmd -type f -name "*.go")
CHECKSUM=$(shell md5sum $(SRC_DEPS) | md5sum | awk '{print $$1}')
PKG:=k8s.io/client-go/pkg
LDFLAGS:=-X $(PKG)/version.gitVersion=$(GIT_TAG) -X $(PKG)/version.gitCommit=$(GIT_COMMIT) -X $(PKG)/version.buildDate=$(BUILD_DATE)

metrics-server: $(SRC_DEPS)
	GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o metrics-server sigs.k8s.io/metrics-server/cmd/metrics-server

# Image Rules
# -----------

CONTAINER_ARCH_TARGETS=$(addprefix container-,$(ALL_ARCHITECTURES))

.PHONY: container
container:
	# Pull base image explicitly. Keep in sync with Dockerfile, otherwise
	# GCB builds will start failing.
	docker pull golang:1.16.4
	docker buildx build -t $(REGISTRY)/metrics-server-$(ARCH):$(CHECKSUM) --build-arg ARCH=$(ARCH) --build-arg GIT_TAG=$(GIT_TAG) --build-arg GIT_COMMIT=$(GIT_COMMIT) .

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
	mkdir -p _output
	kubectl kustomize manifests/release > _output/components.yaml
	echo "Please upload file _output/components.yaml to GitHub release"

# Unit tests
# ----------

.PHONY: test-unit
test-unit:
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race ./pkg/... ./cmd/...

# Benchmarks
# ----------

HAS_BENCH_STORAGE=$(wildcard ./_output/bench_storage.txt)

.PHONY: bench-storage
bench-storage: benchstat
	@mkdir -p _output
ifneq ("$(HAS_BENCH_STORAGE)","")
	@mv _output/bench_storage.txt _output/bench_storage.old.txt
endif
	@go test ./pkg/storage/ -bench=. -run=^$ -benchmem -count 5 -timeout 1h | tee _output/bench_storage.txt
ifeq ("$(HAS_BENCH_STORAGE)","")
	@cp _output/bench_storage.txt _output/bench_storage.old.txt
endif
	@echo
	@echo 'Comparing versus previous run. When optimizing copy everything below this line and include in PR description.'
	@echo
	@benchstat _output/bench_storage.old.txt _output/bench_storage.txt

HAS_BENCHSTAT:=$(shell which benchstat)
.PHONY: benchstat
benchstat:
ifndef HAS_BENCHSTAT
	@go install -mod=readonly golang.org/x/perf/cmd/benchstat
endif

# CLI flags tests
# ------------

.PHONY: test-cli
test-cli: container
	IMAGE=$(REGISTRY)/metrics-server-$(ARCH):$(CHECKSUM) EXPECTED_VERSION=$(GIT_TAG) ./test/test-cli.sh

# E2e tests
# -----------

.PHONY: test-e2e
test-e2e: test-e2e-1.21


.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.21 test-e2e-1.20 test-e2e-1.19

.PHONY: test-e2e-1.21
test-e2e-1.21:
	NODE_IMAGE=kindest/node:v1.21.1@sha256:fae9a58f17f18f06aeac9772ca8b5ac680ebbed985e266f711d936e91d113bad ./test/test-e2e.sh

.PHONY: test-e2e-1.20
test-e2e-1.20:
	NODE_IMAGE=kindest/node:v1.20.7@sha256:e645428988191fc824529fd0bb5c94244c12401cf5f5ea3bd875eb0a787f0fe9 ./test/test-e2e.sh

.PHONY: test-e2e-1.19
test-e2e-1.19:
	NODE_IMAGE=kindest/node:v1.19.11@sha256:7664f21f9cb6ba2264437de0eb3fe99f201db7a3ac72329547ec4373ba5f5911 ./test/test-e2e.sh

# Static analysis
# ---------------

.PHONY: verify
verify: verify-licenses verify-lint verify-toc verify-deps verify-generated verify-structured-logging

.PHONY: update
update: update-licenses update-lint update-toc update-generated

# License
# -------

HAS_ADDLICENSE:=$(shell which addlicense)
.PHONY: verify-licenses
verify-licenses:addlicense
	find -type f -name "*.go" ! -path "*/vendor/*" | xargs $(GOPATH)/bin/addlicense -check || (echo 'Run "make update"' && exit 1)

.PHONY: update-licenses
update-licenses: addlicense
	find -type f -name "*.go" ! -path "*/vendor/*" | xargs $(GOPATH)/bin/addlicense -c "The Kubernetes Authors."

.PHONY: addlicense
addlicense:
ifndef HAS_ADDLICENSE
	go install -mod=readonly github.com/google/addlicense
endif

# Lint
# ----

.PHONY: verify-lint
verify-lint: golangci
	golangci-lint run --timeout 10m --modules-download-mode=readonly || (echo 'Run "make update"' && exit 1)

.PHONY: update-lint
update-lint: golangci
	golangci-lint run --fix --modules-download-mode=readonly

HAS_GOLANGCI:=$(shell which golangci-lint)
.PHONY: golangci
golangci:
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin latest
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

HAS_MDTOC:=$(shell which mdtoc)
.PHONY: mdtoc
mdtoc:
ifndef HAS_MDTOC
	go install -mod=readonly sigs.k8s.io/mdtoc
endif

# Structured Logging
# -----------------

.PHONY: verify-structured-logging
verify-structured-logging: logcheck
	$(GOPATH)/bin/logcheck ./... || (echo 'Fix structured logging' && exit 1)

HAS_LOGCHECK:=$(shell which logcheck)
.PHONY: logcheck
logcheck:
ifndef HAS_LOGCHECK
	go install -mod=readonly k8s.io/klog/hack/tools/logcheck
endif

# Dependencies
# ------------

.PHONY: verify-deps
verify-deps:
	go mod verify
	go mod tidy
	@git diff --exit-code -- go.mod go.sum

# Generated
# ---------

generated_files=pkg/scraper/client/summary/types_easyjson.go pkg/api/generated/openapi/zz_generated.openapi.go

.PHONY: verify-generated
verify-generated: update-generated
	@git diff --exit-code -- $(generated_files)

.PHONY: update-generated
update-generated:
	# pkg/scraper/client/summary/types_easyjson.go:
	go install -mod=readonly github.com/mailru/easyjson/easyjson
	$(GOPATH)/bin/easyjson -all pkg/scraper/client/summary/types.go
	# pkg/api/generated/openapi/zz_generated.openapi.go
	go install -mod=readonly k8s.io/kube-openapi/cmd/openapi-gen
	$(GOPATH)/bin/openapi-gen --logtostderr -i k8s.io/metrics/pkg/apis/metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version -p pkg/api/generated/openapi/ -O zz_generated.openapi -o $(REPO_DIR) -h $(REPO_DIR)/scripts/boilerplate.go.txt -r /dev/null

# Deprecated
# ----------

# Remove when CI is migrated
lint: verify
test-version: test-cli

# Clean
# -----

.PHONY: clean
clean:
	rm -rf _output
