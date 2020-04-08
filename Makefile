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
GOLANGCI_VERSION=v1.23.6
export DOCKER_CLI_EXPERIMENTAL=enabled

# Computed variables
# ------------------
HAS_GOLANGCI:=$(shell which golangci-lint)
GOPATH:=$(shell go env GOPATH)
REPO_DIR:=$(shell pwd)
LDFLAGS=-w $(VERSION_LDFLAGS)

.PHONY: all
all: _output/$(ARCH)/metrics-server

# Build Rules
# -----------

src_deps=$(shell find pkg cmd -type f -name "*.go")
LDFLAGS:=-X sigs.k8s.io/metrics-server/pkg/version.gitVersion=$(GIT_TAG) -X sigs.k8s.io/metrics-server/pkg/version.gitCommit=$(GIT_COMMIT) -X sigs.k8s.io/metrics-server/pkg/version.buildDate=$(BUILD_DATE)
_output/%/metrics-server: $(src_deps)
	GO111MODULE=on GOARCH=$* CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o _output/$*/metrics-server sigs.k8s.io/metrics-server/cmd/metrics-server

yaml_deps=$(shell find deploy/kubernetes -type f -name "*.yaml")
_output/components.yaml: $(yaml_deps)
	cat deploy/kubernetes/*.yaml > _output/components.yaml

# Image Rules
# -----------

.PHONY: container
container: container-$(ARCH)

.PHONY: container-*
container-%:
	docker build --pull -t $(REGISTRY)/metrics-server-$*:$(GIT_COMMIT) -f deploy/docker/Dockerfile --build-arg GOARCH=$* --build-arg LDFLAGS='$(LDFLAGS)' .

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
	@for arch in $(ALL_ARCHITECTURES); do docker manifest annotate --arch $${arch} $(REGISTRY)/metrics-server:$(GIT_TAG) $(REGISTRY)/metrics-server-$${arch}:${VERSION}; done
	docker manifest push --purge $(REGISTRY)/metrics-server:$(GIT_TAG)


# Release rules
# -------------

.PHONY: release-tag
release-tag:
	git tag $(GIT_TAG)
	git push $(GIT_TAG)

.PHONY: release-manifests
release-manifests: _output/components.yaml
	echo "Please upload file _output/components.yaml to GitHub release"

# Unit tests
# ----------

.PHONY: test-unit
test-unit:
ifeq ($(ARCH),amd64)
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race ./pkg/...
else
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short ./pkg/...
endif

# Binary tests
# ------------

.PHONY: test-version
test-version: container
	IMAGE=$(REGISTRY)/metrics-server-$(ARCH):$(GIT_COMMIT) EXPECTED_VERSION=$(GIT_TAG) ./test/version.sh

# E2e tests
# -----------

.PHONY: test-e2e
test-e2e: test-e2e-1.17

.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.17 test-e2e-1.16 test-e2e-1.15

.PHONY: test-e2e-1.17
test-e2e-1.17: container-amd64 _output/components.yaml
	KUBERNETES_VERSION=v1.17.0 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

.PHONY: test-e2e-1.16
test-e2e-1.16: container-amd64 _output/components.yaml
	KUBERNETES_VERSION=v1.16.1 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

.PHONY: test-e2e-1.15
test-e2e-1.15: container-amd64 _output/components.yaml
	KUBERNETES_VERSION=v1.15.0 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

# Static analysis
# ---------------

.PHONY: lint
lint:
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin ${GOLANGCI_VERSION}
endif
	GO111MODULE=on golangci-lint run

.PHONY: fmt
fmt:
	find pkg cmd -type f -name "*.go" | xargs gofmt -s -w

# Clean
# -----

.PHONY: clean
clean:
	rm -rf _output
