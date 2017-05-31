FROM scratch

COPY metrics-server /
COPY ca-certificates.crt /etc/ssl/certs/

#   nobody:nobody
USER 65534:65534
ENTRYPOINT ["/metrics-server"]
