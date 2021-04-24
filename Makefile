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

pkg/scraper/types_easyjson.go: pkg/scraper/types.go
	go install -mod=readonly github.com/mailru/easyjson
	$(GOPATH)/bin/easyjson -all pkg/scraper/types.go

pkg/api/generated/openapi/zz_generated.openapi.go: go.mod
	go install -mod=readonly k8s.io/kube-openapi/cmd/openapi-gen
	$(GOPATH)/bin/openapi-gen --logtostderr -i k8s.io/metrics/pkg/apis/metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version -p pkg/api/generated/openapi/ -O zz_generated.openapi -o $(REPO_DIR) -h $(REPO_DIR)/scripts/boilerplate.go.txt -r /dev/null

# Image Rules
# -----------

.PHONY: container
container: container-$(ARCH)

container-%: $(SRC_DEPS)
	docker build --pull -t $(REGISTRY)/metrics-server-$*:$(CHECKSUM) --build-arg ARCH=$* --build-arg GIT_TAG=$(GIT_TAG) --build-arg GIT_COMMIT=$(GIT_COMMIT) .

# Official Container Push Rules
# -----------------------------

.PHONY: push
push: $(addprefix sub-push-,$(ALL_ARCHITECTURES)) push-multi-arch;

.PHONY: sub-push-*
sub-push-%: container-% do-push-% ;

.PHONY: do-push-*
do-push-%:
	docker tag $(REGISTRY)/metrics-server-$*:$(CHECKSUM) $(REGISTRY)/metrics-server-$*:$(GIT_TAG)
	docker push $(REGISTRY)/metrics-server-$*:$(GIT_TAG)

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
	kubectl kustomize manifests/release > _output/components.yaml
	echo "Please upload file _output/components.yaml to GitHub release"

# Unit tests
# ----------

.PHONY: test-unit
test-unit:
ifeq ($(ARCH),amd64)
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race ./pkg/... ./cmd/...
else
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short ./pkg/... ./cmd/...
endif

# CLI flags tests
# ------------

.PHONY: test-cli
test-cli: container
	IMAGE=$(REGISTRY)/metrics-server-$(ARCH):$(CHECKSUM) EXPECTED_VERSION=$(GIT_TAG) ./test/test-cli.sh

# E2e tests
# -----------

.PHONY: test-e2e
test-e2e: test-e2e-1.20


.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.20 test-e2e-1.19 test-e2e-1.18

.PHONY: test-e2e-1.20
test-e2e-1.20: container
	KUBERNETES_VERSION=v1.20.2@sha256:8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab ./test/test-e2e.sh

.PHONY: test-e2e-1.19
test-e2e-1.19: container
	KUBERNETES_VERSION=v1.19.7@sha256:a70639454e97a4b733f9d9b67e12c01f6b0297449d5b9cbbef87473458e26dca ./test/test-e2e.sh

.PHONY: test-e2e-1.18
test-e2e-1.18: container
	KUBERNETES_VERSION=v1.18.15@sha256:5c1b980c4d0e0e8e7eb9f36f7df525d079a96169c8a8f20d8bd108c0d0889cc4 ./test/test-e2e.sh

# Static analysis
# ---------------

.PHONY: verify
verify: verify-licenses verify-lint verify-toc verify-deps

.PHONY: update
update: update-licenses update-lint update-toc

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

# Dependencies
# ------------

.PHONY: verify-deps
verify-deps:
	go mod verify
	go mod tidy
	@git diff --exit-code -- go.mod go.sum

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
