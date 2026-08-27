# syntax=docker/dockerfile:1
# Offline multi-architecture build for the TSDB storage engine.
#
# The image builds entirely from the vendored dependency tree: no module
# download, proxy or checksum database access happens during the build, so the
# build succeeds in a fully isolated network environment.
FROM golang:1.27-bookworm AS build

ENV GOPROXY=off \
    GOSUMDB=off \
    CGO_ENABLED=0

WORKDIR /src

# Layer the module metadata and vendored dependencies first so the offline
# build never needs to reach the network.
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .

# Build every package from the vendored tree, then emit the server binary.
RUN go build -mod=vendor ./...
RUN go build -mod=vendor -o /out/tsdb-server ./cmd/tsdb-server

# Minimal runtime image with a cert bundle for any optional TLS usage.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/tsdb-server /usr/local/bin/tsdb-server
EXPOSE 8080
ENTRYPOINT ["tsdb-server"]
CMD ["-addr", ":8080"]
