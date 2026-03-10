# Multi-stage build for Foreman and API server (Block 6 in-cluster deploy).
# Builds both binaries; Deployments override CMD to run foreman or apiserver.
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG BUILD_TIME
RUN CGO_ENABLED=0 go build -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" -o /foreman ./cmd/foreman && \
    CGO_ENABLED=0 go build -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" -o /apiserver ./cmd/apiserver

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /foreman /app/foreman
COPY --from=builder /apiserver /app/apiserver
# Default entrypoint; override in Deployment with command.
ENTRYPOINT ["/app/foreman"]
