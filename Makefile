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

src_deps=$(shell find pkg cmd -type f -name "*.go")
PKG:=sigs.k8s.io/metrics-server/pkg
LDFLAGS:=-X $(PKG)/version.gitVersion=$(GIT_TAG) -X $(PKG)/version.gitCommit=$(GIT_COMMIT) -X $(PKG)/version.buildDate=$(BUILD_DATE)
metrics-server: $(src_deps)
	GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o metrics-server sigs.k8s.io/metrics-server/cmd/metrics-server

pkg/scraper/types_easyjson.go: pkg/scraper/types.go
	go get github.com/mailru/easyjson/...
	$(GOPATH)/bin/easyjson -all pkg/scraper/types.go

# Image Rules
# -----------

.PHONY: container
container: container-$(ARCH)

container-%: $(src_deps)
	docker build --pull -t $(REGISTRY)/metrics-server-$*:$(GIT_COMMIT) --build-arg ARCH=$* --build-arg GIT_TAG=$(GIT_TAG) --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg BUILD_DATE .

# Official Container Push Rules
# -----------------------------

.PHONY: push
push: $(addprefix sub-push-,$(ALL_ARCHITECTURES)) push-multi-arch;

.PHONY: sub-push-*
sub-push-%: container-% do-push-% ;

.PHONY: do-push-*
do-push-%:
	docker tag $(REGISTRY)/metrics-server-$*:$(GIT_COMMIT) $(REGISTRY)/metrics-server-$*:$(GIT_TAG)
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

# Binary tests
# ------------

.PHONY: test-version
test-version: container
	IMAGE=$(REGISTRY)/metrics-server-$(ARCH):$(GIT_COMMIT) EXPECTED_VERSION=$(GIT_TAG) ./test/version.sh

# E2e tests
# -----------

.PHONY: test-e2e
test-e2e: test-e2e-1.20


.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.20 test-e2e-1.19 test-e2e-1.18

.PHONY: test-e2e-1.20
test-e2e-1.20: container-amd64
	KUBERNETES_VERSION=v1.20.0@sha256:b40ecf8bcb188f6a0d0f5d406089c48588b75edc112c6f635d26be5de1c89040 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

.PHONY: test-e2e-1.19
test-e2e-1.19: container-amd64
	KUBERNETES_VERSION=v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

.PHONY: test-e2e-1.18
test-e2e-1.18: container-amd64
	KUBERNETES_VERSION=v1.18.8@sha256:f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

# Static analysis
# ---------------

.PHONY: verify
verify: verify-licenses verify-lint verify-toc

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
	go get github.com/google/addlicense
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
	go get sigs.k8s.io/mdtoc
endif

# Deprecated
# TODO remove when CI will migrate to "make verify"
.PHONY: lint
lint: verify

# Clean
# -----

.PHONY: clean
clean:
	rm -rf _output
