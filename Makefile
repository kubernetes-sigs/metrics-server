# Common User-Settable Flags
# ==========================
# Push to staging registry.
PREFIX?=127.0.0.1:5000
ARCH?=amd64
GOLANGCI_VERSION := v1.15.0
BAZEL_BIN?=bazel
HAS_GOLANGCI := $(shell which golangci-lint)
GOPATH := $(shell go env GOPATH)
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x
REPO_DIR:=$(shell pwd)
LDFLAGS=-w $(VERSION_LDFLAGS)


# $(call TEST_KUBERNETES, image_tag, prefix, git_commit)
define TEST_KUBERNETES
	KUBERNETES_VERSION=$(1) IMAGE=$(2)/metrics-server-amd64:$(3) ./test/e2e.sh; \
		if [ $$? != 0 ]; then \
			exit 1; \
		fi;
endef

build:
	$(BAZEL_BIN) build //cmd/metrics-server:metrics-server

include hack/Makefile.buildinfo

.PHONY: build test-unit container container-* clean push do-push-* sub-push-* lint

container:
	$(BAZEL_BIN) build //cmd/metrics-server:image

container-%:
	$(BAZEL_BIN) build //cmd/metrics-server:image --platforms=@io_bazel_rules_go//go/toolchain:linux_$*

container-all: $(addprefix container-,$(ALL_ARCHITECTURES)) ;

push:
	$(BAZEL_BIN) run //cmd/metrics-server:push_image

push-%:
	$(BAZEL_BIN) run //cmd/metrics-server:push_image --platforms=@io_bazel_rules_go//go/toolchain:linux_$*

push-all: $(addprefix push-,$(ALL_ARCHITECTURES)) ;

fmt:
	find pkg cmd -type f -name "*.go" | xargs gofmt -s -w

test-unit:
	$(BAZEL_BIN) test //pkg/... --test_output=streamed

test-version: container
	IMAGE=$(PREFIX)/metrics-server-$(ARCH):$(GIT_COMMIT) EXPECTED_VERSION=$(GIT_VERSION) ./test/version.sh

load-image-docker: container
	docker run -d -p 5000:5000 --name registry registry:2 | true
	$(BAZEL_BIN) run //cmd/metrics-server:push_image
	docker pull 127.0.0.1:5000/metrics-server

test-e2e: test-e2e-1.17
test-e2e-all: test-e2e-1.17 test-e2e-1.16 test-e2e-1.15

test-e2e-1.17: load-image-docker
	$(call TEST_KUBERNETES,v1.17.0,$(PREFIX),latest)

test-e2e-1.16: load-image-docker
	$(call TEST_KUBERNETES,v1.16.1,$(PREFIX),latest)

test-e2e-1.15: load-image-docker
	$(call TEST_KUBERNETES,v1.15.0,$(PREFIX),latest)

lint:
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin ${GOLANGCI_VERSION}
endif
	GO111MODULE=on golangci-lint run
