FROM BASEIMAGE

COPY metrics-server /
COPY ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/metrics-server"]
