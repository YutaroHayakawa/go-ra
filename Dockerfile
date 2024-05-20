FROM debian AS builder

RUN echo "interfaces: []" > /config.yaml

FROM scratch

COPY gorad /gorad
COPY gora /gora
COPY --from=builder /config.yaml /config.yaml

ENTRYPOINT ["/gorad", "-f", "/config.yaml"]
