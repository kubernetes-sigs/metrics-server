# Common User-Settable Flags
# --------------------------
REGISTRY?=gcr.io/k8s-staging-metrics-server
ARCH?=amd64

# Consts
# ------
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x
GOLANGCI_VERSION=v1.23.6

# Computed variables
# ------------------
HAS_GOLANGCI:=$(shell which golangci-lint)
GOPATH:=$(shell go env GOPATH)
REPO_DIR:=$(shell pwd)
LDFLAGS=-w $(VERSION_LDFLAGS)

include hack/Makefile.buildinfo

.PHONY: all
all: _output/$(ARCH)/metrics-server

# Build Rules
# -----------

src_deps=$(shell find pkg cmd -type f -name "*.go")
_output/%/metrics-server: $(src_deps)
	GO111MODULE=on GOARCH=$* CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o _output/$*/metrics-server sigs.k8s.io/metrics-server/cmd/metrics-server

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
	docker tag $(REGISTRY)/metrics-server-$*:$(GIT_COMMIT) $(REGISTRY)/metrics-server-$*:$(VERSION)
	docker push $(REGISTRY)/metrics-server-$*:$(VERSION)

.PHONY: push-multi-arch
push-multi-arch:
	docker manifest create --amend $(REGISTRY)/metrics-server:$(VERSION) $(shell echo $(ALL_ARCHITECTURES) | sed -e "s~[^ ]*~$(REGISTRY)/metrics-server\-&:$(VERSION)~g")
	@for arch in $(ALL_ARCHITECTURES); do docker manifest annotate --arch $${arch} $(REGISTRY)/metrics-server:$(VERSION) $(REGISTRY)/metrics-server-$${arch}:${VERSION}; done
	docker manifest push --purge $(REGISTRY)/metrics-server:$(VERSION)

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
	IMAGE=$(REGISTRY)/metrics-server-$(ARCH):$(GIT_COMMIT) EXPECTED_VERSION=$(GIT_VERSION) ./test/version.sh

# E2e tests
# -----------

.PHONY: test-e2e
test-e2e: test-e2e-1.17

.PHONY: test-e2e-all
test-e2e-all: test-e2e-1.17 test-e2e-1.16 test-e2e-1.15

.PHONY: test-e2e-1.17
test-e2e-1.17: container-amd64
	KUBERNETES_VERSION=v1.17.0 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

.PHONY: test-e2e-1.16
test-e2e-1.16: container-amd64
	KUBERNETES_VERSION=v1.16.1 IMAGE=$(REGISTRY)/metrics-server-amd64:$(GIT_COMMIT) ./test/e2e.sh

.PHONY: test-e2e-1.15
test-e2e-1.15: container-amd64
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
