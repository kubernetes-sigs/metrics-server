all: build

PREFIX?=gcr.io/google_containers
FLAGS=
ARCH?=amd64
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x
ML_PLATFORMS=linux/amd64,linux/arm,linux/arm64,linux/ppc64le,linux/s390x
GOLANG_VERSION?=1.8

ifndef TEMP_DIR
TEMP_DIR:=$(shell mktemp -d /tmp/metrics-server.XXXXXX)
endif

GIT_COMMIT:=$(shell git rev-parse "HEAD^{commit}" 2>/dev/null)
GIT_VERSION_RAW:=$(shell git describe --tags --abbrev=14 "$(GIT_COMMIT)^{commit}" 2>/dev/null)
DASHES_IN_VERSION:=$(shell echo "$(GIT_VERSION_RAW)" | sed "s/[^-]//g")
GIT_VERSION:=$(GIT_VERSION_RAW)
ifeq ($(DASHES_IN_VERSION), ---)
GIT_VERSION:=$(shell echo "$(GIT_VERSION_RAW)" | sed "s/-\([0-9]\{1,\}\)-g\([0-9a-f]\{14\}\)$$/.\1\+\2/")
endif
ifeq ($(DASHES_IN_VERSION), --)
GIT_VERSION:=$(shell echo "$(GIT_VERSION_RAW)" | sed "s/-g\([0-9a-f]\{14\}\)$$/+\1/")
endif

ifeq ($(shell status --porcelain 2>/dev/null), "")
GIT_TREE_STATE:=clean
else
GIT_TREE_STATE:=dirty
GIT_VERSION:=$(GIT_VERSION)-dirty
endif
ifdef SOURCE_DATE_EPOCH
BUILD_DATE:=$(shell date --date=@${SOURCE_DATE_EPOCH} -u +'%Y-%m-%dT%H:%M:%SZ')
else
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
endif

# Set default base image dynamically for each arch
ifeq ($(ARCH),amd64)
	BASEIMAGE?=busybox
endif
ifeq ($(ARCH),arm)
	BASEIMAGE?=arm32v7/busybox
endif
ifeq ($(ARCH),arm64)
	BASEIMAGE?=arm64v8/busybox
endif
ifeq ($(ARCH),ppc64le)
	BASEIMAGE?=ppc64le/busybox
endif
ifeq ($(ARCH),s390x)
	BASEIMAGE?=s390x/busybox
endif
VERSION:=$(shell echo "$(GIT_VERSION)" | grep -E -o '^v[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+(-(alpha|beta)\.[[:digit:]]+)?')

ifdef REPO_DIR
DOCKER_IN_DOCKER=1
else
REPO_DIR:=$(shell pwd)
endif

# You can set this variable for testing and the built image will also be tagged with this name
OVERRIDE_IMAGE_NAME?=

# If this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
TTY=
ifeq ($(INTERACTIVE), 1)
	TTY=-t
endif

LDFLAGS=-w -X github.com/kubernetes-incubator/metrics-server/pkg/version.gitVersion=$(GIT_VERSION) -X github.com/kubernetes-incubator/metrics-server/pkg/version.gitCommit=$(GIT_COMMIT) -X github.com/kubernetes-incubator/metrics-server/pkg/version.gitTreeState=$(GIT_TREE_STATE) -X github.com/kubernetes-incubator/metrics-server/pkg/version.buildDate=$(BUILD_DATE)

version-info:
	@echo "Version: $(GIT_VERSION) ($(VERSION))"
	@echo "    built from $(GIT_COMMIT) ($(GIT_TREE_STATE))"
	@echo "    built on $(BUILD_DATE)"

fmt:
	find . -type f -name "*.go" | grep -v "./vendor*" | xargs gofmt -s -w

build: fmt
	GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o _output/$(ARCH)/metrics-server github.com/kubernetes-incubator/metrics-server/cmd/metrics-server

test-unit:
ifeq ($(ARCH),amd64)
	GOARCH=$(ARCH) go test --test.short -race ./... $(FLAGS)
else
	GOARCH=$(ARCH) go test --test.short ./... $(FLAGS)
endif

container:
	# Run the build in a container in order to have reproducible builds
	# Also, fetch the latest ca certificates
	docker run --rm -i $(TTY) -v $(TEMP_DIR):/build -v $(REPO_DIR):/go/src/github.com/kubernetes-incubator/metrics-server -w /go/src/github.com/kubernetes-incubator/metrics-server golang:$(GOLANG_VERSION) /bin/bash -c "\
		cp /etc/ssl/certs/ca-certificates.crt /build \
		&& GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags \"$(LDFLAGS)\" -o /build/metrics-server github.com/kubernetes-incubator/metrics-server/cmd/metrics-server"

	cp deploy/docker/Dockerfile $(TEMP_DIR)
	sed -i -e "s|BASEIMAGE|$(BASEIMAGE)|g" $(TEMP_DIR)/Dockerfile
	docker build --pull -t $(PREFIX)/metrics-server-$(ARCH):$(VERSION) $(TEMP_DIR)
ifneq ($(OVERRIDE_IMAGE_NAME),)
	docker tag -f $(PREFIX)/metrics-server-$(ARCH):$(VERSION) $(OVERRIDE_IMAGE_NAME)
endif

ifndef DOCKER_IN_DOCKER
	rm -rf $(TEMP_DIR)
endif

do-push:
	docker push $(PREFIX)/metrics-server-$(ARCH):$(VERSION)
ifeq ($(ARCH),amd64)
# TODO: Remove this and push the manifest list as soon as it's working
	docker tag $(PREFIX)/metrics-server-$(ARCH):$(VERSION) $(PREFIX)/metrics-server:$(VERSION)
	docker push $(PREFIX)/metrics-server:$(VERSION)
endif

# Should depend on target: ./manifest-tool
push: gcr-login $(addprefix sub-push-,$(ALL_ARCHITECTURES))
#	./manifest-tool push from-args --platforms $(ML_PLATFORMS) --template $(PREFIX)/metrics-server-ARCH:$(VERSION) --target $(PREFIX)/metrics-server:$(VERSION)

sub-push-%:
	$(MAKE) ARCH=$* PREFIX=$(PREFIX) VERSION=$(VERSION) container
	$(MAKE) ARCH=$* PREFIX=$(PREFIX) VERSION=$(VERSION) do-push

gcr-login:
ifeq ($(findstring gcr.io,$(PREFIX)),gcr.io)
	@echo "If you are pushing to a gcr.io registry, you have to be logged in via 'docker login'; 'gcloud docker push' can't push manifest lists yet."
	@echo "This script is automatically logging you in now with 'gcloud docker -a'"
	gcloud docker -a
endif

# TODO(luxas): As soon as it's working to push fat manifests to gcr.io, reenable this code
#./manifest-tool:
#	curl -sSL https://github.com/luxas/manifest-tool/releases/download/v0.3.0/manifest-tool > manifest-tool
#	chmod +x manifest-tool

clean:
	rm -rf _output

.PHONY: all build test-unit container clean version-info
