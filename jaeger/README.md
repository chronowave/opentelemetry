## Jaeger ChronoWave gRPC Storage Plugin

#### build docker image
```BASH
docker build -t chronowave/jaeger-all-in-one .
```

#### local build

1. build plugin binary
```shell script
go build
```

2. start `jaeger-all-in-one`, and save chronowave data files under `/data`
```shell script
SPAN_STORAGE_TYPE=grpc-plugin jager-all-in-one\
    --grpc-storage-plugin.binary chronowave-jaeger \
    --grpc-storage-plugin.configuration-file plugin.yaml
```
