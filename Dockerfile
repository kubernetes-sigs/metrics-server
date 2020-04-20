FROM golang:1.14.2 as build
ENV CGO_ENABLED=0
ENV GOPATH=/go

WORKDIR /go/src/sigs.k8s.io/metrics-server
COPY go.mod .
COPY go.sum .
COPY vendor vendor
RUN go mod download

COPY pkg pkg
COPY cmd cmd

ARG GOARCH
ARG LDFLAGS
RUN go build -mod=readonly -ldflags "$LDFLAGS" -o /metrics-server $PWD/cmd/metrics-server

FROM gcr.io/distroless/static:latest

COPY --from=build metrics-server /

USER 65534

ENTRYPOINT ["/metrics-server"]
