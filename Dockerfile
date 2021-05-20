# Update the base image in Makefile when updating golang version. This has to
# be pre-pulled in order to work on GCB.
FROM golang:1.16.4 as build

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
RUN setcap cap_net_bind_service=+ep metrics-server

FROM gcr.io/distroless/static:latest
COPY --from=build /go/src/sigs.k8s.io/metrics-server/metrics-server /
USER 65534
ENTRYPOINT ["/metrics-server"]
