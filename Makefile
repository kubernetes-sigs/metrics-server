# Common User-Settable Flags
# ==========================
# Push to staging registry.
PREFIX?=staging-k8s.gcr.io
FLAGS=
ARCH?=amd64
GOLANG_VERSION?=1.13.8
GOLANGCI_VERSION := v1.23.6
HAS_GOLANGCI := $(shell which golangci-lint)
GOPATH := $(shell go env GOPATH)

# $(call TEST_KUBERNETES, image_tag, prefix, git_commit)
define TEST_KUBERNETES
	KUBERNETES_VERSION=$(1) IMAGE=$(2)/metrics-server-amd64:$(3) ./test/e2e.sh; \
		if [ $$? != 0 ]; then \
			exit 1; \
		fi;
endef


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
include hack/Makefile.buildinfo
# Rules
# =====

.PHONY: all test-unit container container-* clean container-only container-only-* tmpdir push do-push-* sub-push-* lint

# Build Rules
# -----------

# building depends on all go files (this is mostly redundant in the face of go 1.10's incremental builds,
# but it allows us to safely write actual dependency rules in our makefile)
src_deps=$(shell find pkg cmd -type f -name "*.go")
_output/%/metrics-server: $(src_deps)	
	GO111MODULE=on GOARCH=$* CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o _output/$*/metrics-server sigs.k8s.io/metrics-server/cmd/metrics-server

# Image Rules
# -----------

# build a container using containerized build (the current arch by default)
container: container-$(ARCH)

container-%:
	docker build --pull -t $(PREFIX)/metrics-server-$*:$(GIT_COMMIT) -f deploy/docker/Dockerfile --build-arg GOARCH=$* --build-arg LDFLAGS='$(LDFLAGS)' .

# Official Container Push Rules
# -----------------------------

# do the actual push for official images
do-push-%:
	docker tag $(PREFIX)/metrics-server-$*:$(GIT_COMMIT) $(PREFIX)/metrics-server-$*:$(VERSION)
	docker push $(PREFIX)/metrics-server-$*:$(VERSION)

	# push alternate tags
ifeq ($*,amd64)
	# TODO: Remove this and push the manifest list as soon as it's working
	docker tag $(PREFIX)/metrics-server-$*:$(VERSION) $(PREFIX)/metrics-server:$(VERSION)
	docker push $(PREFIX)/metrics-server:$(VERSION)
endif

# do build and then push a given official image
sub-push-%: container-% do-push-% ;

# do build and then push all official images
push: gcr-login $(addprefix sub-push-,$(ALL_ARCHITECTURES)) ;
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

test-unit:
ifeq ($(ARCH),amd64)
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short -race ./pkg/... $(FLAGS)
else
	GO111MODULE=on GOARCH=$(ARCH) go test --test.short ./pkg/... $(FLAGS)
endif

test-version: container
	IMAGE=$(PREFIX)/metrics-server-$(ARCH):$(GIT_COMMIT) EXPECTED_VERSION=$(GIT_VERSION) ./test/version.sh

# e2e Test Rules
test-e2e-latest: container-amd64
	$(call TEST_KUBERNETES,v1.17.0,$(PREFIX),$(GIT_COMMIT))

test-e2e-1.17: container-amd64
	$(call TEST_KUBERNETES,v1.17.0,$(PREFIX),$(GIT_COMMIT))

test-e2e-1.16: container-amd64
	$(call TEST_KUBERNETES,v1.16.1,$(PREFIX),$(GIT_COMMIT))

test-e2e-1.15: container-amd64
	$(call TEST_KUBERNETES,v1.15.0,$(PREFIX),$(GIT_COMMIT))

test-e2e-all: container-amd64 test-e2e-latest test-e2e-1.17 test-e2e-1.16 test-e2e-1.15

# set up a temporary director when we need it
# it's the caller's responsibility to clean it up
tmpdir-%:
	$(eval TEMP_DIR:=$(shell mktemp -d /tmp/metrics-server.XXXXXX))
lint:
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin ${GOLANGCI_VERSION}
endif
	GO111MODULE=on golangci-lint run
