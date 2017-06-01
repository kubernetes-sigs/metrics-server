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

VERSION?=v0.1.0
GIT_COMMIT:=$(shell git rev-parse --short HEAD)

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

LDFLAGS=-w -X github.com/kubernetes-incubator/metrics-server/version.MetricsServerVersion=$(VERSION) -X github.com/kubernetes-incubator/metrics-server/version.GitCommit=$(GIT_COMMIT)

fmt:
	find . -type f -name "*.go" | grep -v "./vendor*" | xargs gofmt -s -w

build: clean fmt
	GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o metrics-server github.com/kubernetes-incubator/metrics-server/metrics

test-unit: clean build
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
		&& GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags \"$(LDFLAGS)\" -o /build/metrics-server github.com/kubernetes-incubator/metrics-server/metrics"

	cp deploy/docker/Dockerfile $(TEMP_DIR)
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
	rm -f metrics-server

.PHONY: all build test-unit container clean
