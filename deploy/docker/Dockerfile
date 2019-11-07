FROM golang:1.12.12 as build
ENV CGO_ENABLED=0
ENV GOPATH=/go

WORKDIR /go/src/github.com/kubernetes-incubator/metrics-server
COPY pkg pkg
COPY cmd cmd
COPY vendor vendor

ARG GOARCH
ARG LDFLAGS
RUN go build -ldflags "$LDFLAGS" -o /metrics-server $PWD/cmd/metrics-server

FROM gcr.io/distroless/static:latest

COPY --from=build metrics-server /

ENTRYPOINT ["/metrics-server"]
