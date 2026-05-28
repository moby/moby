FROM alpine
RUN apk add --no-cache curl
COPY diagnosticClient /usr/local/bin/diagnosticClient
ENTRYPOINT ["/usr/local/bin/diagnosticClient"]
