# Common User-Settable Flags
# ==========================
# Push to staging registry.
PREFIX?=gcr.io/k8s-staging-metrics-server
FLAGS=
ARCH?=amd64
GOLANG_VERSION?=1.10
GOLANGCI_VERSION := v1.15.0
HAS_GOLANGCI := $(shell which golangci-lint)

# Release variables
# ------------------
GIT_COMMIT?=$(shell git rev-parse "HEAD^{commit}" 2>/dev/null)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

export DOCKER_CLI_EXPERIMENTAL=enabled

# by default, build the current arch's binary
# (this needs to be pre-include, for some reason)
all: _output/$(ARCH)/metrics-server

# Constants
# =========
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x

# Calculated Variables
# ====================
REPO_DIR:=$(shell pwd)
LDFLAGS=-w $(VERSION_LDFLAGS)
# get the appropriate version information
BASEIMAGE?=gcr.io/distroless/static:latest
# Rules
# =====

.PHONY: all test-unit container container-* clean container-only container-only-* tmpdir push do-push-* sub-push-* lint push-all

# Build Rules
# -----------


# building depends on all go files (this is mostly redundant in the face of go 1.10's incremental builds,
# but it allows us to safely write actual dependency rules in our makefile)
src_deps=$(shell find pkg cmd -type f -name "*.go" -and ! -name "zz_generated.*.go")
LDFLAGS:=-X sigs.k8s.io/metrics-server/pkg/version.gitVersion=$(GIT_TAG) -X sigs.k8s.io/metrics-server/pkg/version.gitCommit=$(GIT_COMMIT) -X sigs.k8s.io/metrics-server/pkg/version.buildDate=$(BUILD_DATE)
_output/%/metrics-server: $(src_deps)
	GOARCH=$* CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o _output/$*/metrics-server github.com/kubernetes-incubator/metrics-server/cmd/metrics-server

yaml_deps=$(shell find deploy/kubernetes -type f -name "*.yaml")
_output/components.yaml: $(yaml_deps) _output
	cat deploy/kubernetes/*.yaml > _output/components.yaml

_output:
	mkdir -p _output

# Image Rules
# -----------

# build a container using containerized build (the current arch by default)
container: container-$(ARCH)

container-%:
	docker build --pull -t $(PREFIX)/metrics-server-$*:$(GIT_TAG) -f deploy/docker/Dockerfile --build-arg GOARCH=$* --build-arg LDFLAGS='$(LDFLAGS)' .

# Official Container Push Rules
# -----------------------------

# do the actual push for official images
do-push-%:
	# push with main tag
	docker push $(PREFIX)/metrics-server-$*:$(GIT_TAG)

# do build and then push a given official image
sub-push-%: container-% do-push-%;

# push the fat manifest
push-all:
	docker manifest create --amend $(PREFIX)/metrics-server:$(GIT_TAG) $(shell echo $(ALL_ARCHITECTURES) | sed -e "s~[^ ]*~$(PREFIX)/metrics-server\-&:$(GIT_TAG)~g")
	@for arch in $(ALL_ARCHITECTURES); do docker manifest annotate --arch $${arch} $(PREFIX)/metrics-server:$(GIT_TAG) $(PREFIX)/metrics-server-$${arch}:${GIT_TAG}; done
	docker manifest push --purge $(PREFIX)/metrics-server:$(GIT_TAG)

# do build and then push all official images
push: gcr-login $(addprefix sub-push-,$(ALL_ARCHITECTURES)) push-all;
	# TODO: push with manifest-tool?
	# Should depend on target: ./manifest-tool

# log in to the official container registry
gcr-login:
ifeq ($(findstring gcr.io,$(PREFIX)),gcr.io)
	@echo "If you are pushing to a gcr.io registry, you have to be logged in via 'docker login'; 'gcloud docker push' can't push manifest lists yet."
	@echo "This script is automatically logging you in now with 'gcloud docker -a'"
	gcloud docker -a
endif

# Utility Rules
# -------------

clean:
	rm -rf _output

fmt:
	find pkg cmd -type f -name "*.go" | xargs gofmt -s -w

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

test-unit:
ifeq ($(ARCH),amd64)
	GOARCH=$(ARCH) go test --test.short -race ./pkg/... $(FLAGS)
else
	GOARCH=$(ARCH) go test --test.short ./pkg/... $(FLAGS)
endif

# set up a temporary director when we need it
# it's the caller's responsibility to clean it up
tmpdir-%:
	$(eval TEMP_DIR:=$(shell mktemp -d /tmp/metrics-server.XXXXXX))
lint:
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin ${GOLANGCI_VERSION}
endif
	golangci-lint run
