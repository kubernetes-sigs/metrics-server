FROM BASEIMAGE

COPY metrics-server /

RUN adduser -D metrics-server

USER metrics-server

ENTRYPOINT ["/metrics-server"]
