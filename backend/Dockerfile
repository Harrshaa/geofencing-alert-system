# Multi-stage build: compile in a full Go image, ship only the binary.
# The final image is a few megabytes instead of several hundred.

# ---- Stage 1: build ----
#
# --platform=$BUILDPLATFORM runs this stage on the machine doing the build
# (e.g. an Apple Silicon Mac), and Go cross-compiles for the target below.
# That is much faster than emulating another CPU when building for servers.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src

# Copy the dependency list first. Docker caches this layer, so dependencies
# are only downloaded again when go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary that needs no system libraries.
# -ldflags "-s -w" strips debug information to make it smaller.
# TARGETOS/TARGETARCH are filled in by Docker from --platform
# (e.g. linux/amd64 for most servers, linux/arm64 for Apple Silicon).
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/server .

# ---- Stage 2: run ----
FROM alpine:3.22

# ca-certificates: needed for TLS connections to a managed database.
# Run as a normal user, not root.
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app

COPY --from=build /out/server /usr/local/bin/server

EXPOSE 8080
ENTRYPOINT ["server"]
