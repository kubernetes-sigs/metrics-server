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

SRC_DEPS=$(shell find pkg cmd -type f -name "*.go") go.mod go.sum
CHECKSUM=$(shell md5sum $(SRC_DEPS) | md5sum | awk '{print $$1}')
PKG:=k8s.io/client-go/pkg
LDFLAGS:=-X $(PKG)/version.gitVersion=$(GIT_TAG) -X $(PKG)/version.gitCommit=$(GIT_COMMIT) -X $(PKG)/version.buildDate=$(BUILD_DATE)

metrics-server: $(SRC_DEPS)
	GOARCH=$(ARCH) CGO_ENABLED=0 go build -mod=readonly -ldflags "$(LDFLAGS)" -o metrics-server sigs.k8s.io/metrics-server/cmd/metrics-server

# Image Rules
# -----------

CONTAINER_ARCH_TARGETS=$(addprefix container-,$(ALL_ARCHITECTURES))

.PHONY: container
container:
	# Pull base image explicitly. Keep in sync with Dockerfile, otherwise
	# GCB builds will start failing.
	docker pull golang:1.19.0
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
	mkdir -p _output
	kubectl kustomize manifests/overlays/release > _output/components.yaml
	kubectl kustomize manifests/overlays/release-ha > _output/high-availability.yaml


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
test-e2e: test-e2e-1.25

.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.25 test-e2e-1.24 test-e2e-1.23

.PHONY: test-e2e-1.25
test-e2e-1.25:
	NODE_IMAGE=kindest/node:v1.25.0@sha256:428aaa17ec82ccde0131cb2d1ca6547d13cf5fdabcc0bbecf749baa935387cbf ./test/test-e2e.sh

.PHONY: test-e2e-1.24
test-e2e-1.24:
	NODE_IMAGE=kindest/node:v1.24.4@sha256:adfaebada924a26c2c9308edd53c6e33b3d4e453782c0063dc0028bdebaddf98 ./test/test-e2e.sh

.PHONY: test-e2e-1.23
test-e2e-1.23:
	NODE_IMAGE=kindest/node:v1.23.10@sha256:f047448af6a656fae7bc909e2fab360c18c487ef3edc93f06d78cdfd864b2d12 ./test/test-e2e.sh

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
verify: verify-licenses verify-lint verify-toc verify-deps verify-generated verify-structured-logging

.PHONY: update
update: update-licenses update-lint update-toc update-deps update-generated

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
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.48.0
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
	go install -mod=readonly sigs.k8s.io/logtools/logcheck
endif

# Dependencies
# ------------

.PHONY: update-deps
update-deps:
	go mod tidy

.PHONY: verify-deps
verify-deps:
	go mod verify
	go mod tidy
	@git diff --exit-code -- go.mod go.sum

# Generated
# ---------

generated_files=pkg/api/generated/openapi/zz_generated.openapi.go

.PHONY: verify-generated
verify-generated: update-generated
	@git diff --exit-code -- $(generated_files)

.PHONY: update-generated
update-generated:
	# pkg/api/generated/openapi/zz_generated.openapi.go
	go install -mod=readonly k8s.io/kube-openapi/cmd/openapi-gen
	$(GOPATH)/bin/openapi-gen --logtostderr -i k8s.io/metrics/pkg/apis/metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version -p pkg/api/generated/openapi/ -O zz_generated.openapi -o $(REPO_DIR) -h $(REPO_DIR)/scripts/boilerplate.go.txt -r /dev/null

# Deprecated
# ----------

# Remove when CI is migrated
lint: verify
test-version: test-image-all

# Clean
# -----

.PHONY: clean
clean:
	rm -rf _output
