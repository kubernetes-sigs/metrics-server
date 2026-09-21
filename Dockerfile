# GOLANG_VERSION comes from the go directive in go.mod, via the Makefile.
# make container pre-pulls that image so GCB builds can use the local cache.
ARG GOLANG_VERSION
ARG ARCH
FROM golang:${GOLANG_VERSION} as build

WORKDIR /go/src/sigs.k8s.io/metrics-server
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY pkg pkg
COPY cmd cmd
COPY Makefile Makefile

ARG ARCH
ARG GIT_COMMIT
ARG GIT_TAG
RUN make metrics-server

FROM gcr.io/distroless/static:latest-$ARCH
COPY --from=build /go/src/sigs.k8s.io/metrics-server/metrics-server /
USER 65534
ENTRYPOINT ["/metrics-server"]
