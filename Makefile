# Common User-Settable Flags
# ==========================
# Push to staging registry.
PREFIX?=staging-k8s.gcr.io
FLAGS=
ARCH?=amd64
GOLANG_VERSION?=1.10
GOLANGCI_VERSION := v1.15.0
HAS_GOLANGCI := $(shell which golangci-lint)

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
BASEIMAGE?=gcr.io/distroless/static:latest
# Rules
# =====

.PHONY: all test-unit container container-* clean container-only container-only-* tmpdir push do-push-* sub-push-* lint

# Build Rules
# -----------

pkg/generated/openapi/zz_generated.openapi.go:
	go run vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go --logtostderr -i k8s.io/metrics/pkg/apis/metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version -p github.com/kubernetes-incubator/metrics-server/pkg/generated/openapi/ -O zz_generated.openapi -h $(REPO_DIR)/hack/boilerplate.go.txt -r /dev/null

# building depends on all go files (this is mostly redundant in the face of go 1.10's incremental builds,
# but it allows us to safely write actual dependency rules in our makefile)
src_deps=$(shell find pkg cmd -type f -name "*.go" -and ! -name "zz_generated.*.go")
_output/%/metrics-server: $(src_deps) pkg/generated/openapi/zz_generated.openapi.go
	GOARCH=$* CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o _output/$*/metrics-server github.com/kubernetes-incubator/metrics-server/cmd/metrics-server

# Image Rules
# -----------

# build a container using containerized build (the current arch by default)
container: container-$(ARCH)

container-%: pkg/generated/openapi/zz_generated.openapi.go tmpdir-%
	# Run the build in a container in order to have reproducible builds
	docker run --rm -v $(TEMP_DIR):/build -v $(REPO_DIR):/go/src/github.com/kubernetes-incubator/metrics-server -w /go/src/github.com/kubernetes-incubator/metrics-server golang:$(GOLANG_VERSION) /bin/bash -c "\
		GOARCH=$* CGO_ENABLED=0 go build -ldflags \"$(LDFLAGS)\" -o /build/metrics-server github.com/kubernetes-incubator/metrics-server/cmd/metrics-server"


	# copy the base Dockerfile into the temp dir, and set the base image
	cp deploy/docker/Dockerfile $(TEMP_DIR)
	sed -i -e "s|BASEIMAGE|$(BASEIMAGE)|g" $(TEMP_DIR)/Dockerfile

	# run the actual build
	docker build --pull -t $(PREFIX)/metrics-server-$*:$(VERSION) $(TEMP_DIR)

	# remove our TEMP_DIR, as needed
	rm -rf $(TEMP_DIR)

# build a container using a locally-built binary (the current arch by default)
container-only: container-only-$(ARCH)

container-only-%: _output/$(ARCH)/metrics-server tmpdir-%
	# copy the base Dockerfile and binary into the temp dir, and set the base image
	cp deploy/docker/Dockerfile $(TEMP_DIR)
	cp _output/$*/metrics-server $(TEMP_DIR)
	sed -i -e "s|BASEIMAGE|$(BASEIMAGE)|g" $(TEMP_DIR)/Dockerfile

	# run the actual build
	docker build --pull -t $(PREFIX)/metrics-server-$*:$(VERSION) $(TEMP_DIR)

	# remove our TEMP_DIR, as needed
	rm -rf $(TEMP_DIR)

# Official Container Push Rules
# -----------------------------

# do the actual push for official images
do-push-%:
	# push with main tag
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
	rm pkg/generated/openapi/zz_generated.openapi.go

fmt:
	find pkg cmd -type f -name "*.go" | xargs gofmt -s -w

test-unit: pkg/generated/openapi/zz_generated.openapi.go
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
