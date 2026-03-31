# Multi-stage Dockerfile for zen-brain1
# Build from repo root: docker build -t zen-brain:dev .
# For k3d local registry: docker build -t localhost:5000/zen-brain:dev .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates

# Set working directory
WORKDIR /build

# Copy everything (including vendor/)
COPY . .

# Build all in-cluster binaries using vendor directory (no network needed)
ARG BUILD_SHA=""
ARG VERSION="dev"
ENV GONOSUMCHECK=github.com/kube-zen/*
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w -X main.version=${VERSION} -X main.buildCommit=${BUILD_SHA}" -o zen-brain ./cmd/zen-brain && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w" -o foreman ./cmd/foreman && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w" -o apiserver ./cmd/apiserver && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w" -o controller ./cmd/controller

# Runtime stage (minimal Alpine image)
FROM alpine:3.19

# Install ca-certificates and create user in single layer (reduces layers)
RUN apk --no-cache add ca-certificates && \
    adduser -D -s /bin/sh -u 1000 zenuser

# In-cluster binaries under /app (Block 6 bootstrap)
# Use --chown to set ownership during copy (eliminates separate chown layer)
WORKDIR /app
COPY --from=builder --chown=zenuser:zenuser /build/zen-brain /build/foreman /build/apiserver /build/controller .

USER zenuser
ENTRYPOINT ["./zen-brain"]
