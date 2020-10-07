FROM golang:1.15.2-alpine3.12 AS builder

RUN apk update && apk add --no-cache alpine-sdk cmake

COPY . /src
WORKDIR /src

RUN go build -o /usr/local/bin/chronowave-linux

FROM jaegertracing/opentelemetry-all-in-one

COPY --from=builder /usr/local/bin/chronowave-linux /go/bin/chronowave-linux
COPY plugin.yaml /etc/jaeger/plugin.yaml

ENV SPAN_STORAGE_TYPE=grpc-plugin

VOLUME ["/tmp"]
ENTRYPOINT ["/go/bin/all-in-one-linux"]
CMD ["--sampling.strategies-file=/etc/jaeger/sampling_strategies.json", "--grpc-storage-plugin.binary=/go/bin/chronowave-linux", "--grpc-storage-plugin.configuration-file=/etc/jaeger/plugin.yaml"]
