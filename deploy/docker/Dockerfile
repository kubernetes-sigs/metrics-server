FROM gcr.io/google_containers/ubuntu-slim:0.1

COPY metrics-server /
COPY ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/metrics-server"]
