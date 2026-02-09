FROM golang:1.23 AS builder

COPY . /build
WORKDIR /build
RUN CGO_ENABLED=0 go build -o /toposcope-extract ./cmd/toposcope

FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git \
        default-jdk \
        curl \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN curl -Lo /usr/local/bin/bazelisk \
    https://github.com/bazelbuild/bazelisk/releases/latest/download/bazelisk-linux-amd64 && \
    chmod +x /usr/local/bin/bazelisk && \
    ln -s /usr/local/bin/bazelisk /usr/local/bin/bazel

COPY --from=builder /toposcope-extract /usr/local/bin/toposcope

ENTRYPOINT ["toposcope"]
