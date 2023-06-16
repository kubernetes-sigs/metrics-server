# Update the base image in Makefile when updating golang version. This has to
# be pre-pulled in order to work on GCB.
ARG ARCH
FROM golang:1.19.7 as build

# FIPS
ARG CRYPTO_LIB
ENV GOEXPERIMENT=${CRYPTO_LIB:+boringcrypto}

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
