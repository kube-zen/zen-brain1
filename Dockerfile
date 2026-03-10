# Multi-stage Dockerfile for zen-brain1
# Build from repo root: docker build -t zen-brain:dev .
# For k3d local registry: docker build -t localhost:5000/zen-brain:dev .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies (git needed for go mod download)
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go.mod, go.sum, and vendor directory for offline builds
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source code
COPY . .

# Build all in-cluster binaries (Block 6: foreman, apiserver, controller)
# Use -mod=vendor for offline builds (avoids git authentication issues)
ARG BUILD_SHA=""
ARG VERSION="dev"
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w -X main.version=${VERSION} -X main.buildCommit=${BUILD_SHA}" -o zen-brain ./cmd/zen-brain && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w" -o foreman ./cmd/foreman && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w" -o apiserver ./cmd/apiserver && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w" -o controller ./cmd/controller

# Runtime stage (minimal Alpine image)
FROM alpine:3.19

RUN apk --no-cache add ca-certificates
RUN adduser -D -s /bin/sh zenuser

# In-cluster binaries under /app (Block 6 bootstrap)
WORKDIR /app
COPY --from=builder /build/zen-brain /build/foreman /build/apiserver /build/controller .
RUN chown -R zenuser:zenuser /app

USER zenuser
ENTRYPOINT ["./zen-brain"]
