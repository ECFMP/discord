# Base stage, sets up the GRPC Health Probe, we use a ubuntu image because no devcontainer features work on alpine
ARG GO_VERSION
ARG TARGETOS
ARG TARGETARCH
FROM golang:${GO_VERSION} AS builder_base

WORKDIR /app

# Install gRPC Health Probe
RUN set -ex \
    && curl -fSL https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc_health_probe-$TARGETOS-$TARGETARCH -o /usr/local/bin/grpc_health_probe \
    && chmod +x /usr/local/bin/grpc_health_probe

#######################################################
# Builds the development container
FROM builder_base AS development

# We listen on port 80 in development
EXPOSE 80

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 CMD [ "grpc_health_probe", "-addr", "localhost:80", "-connect-timeout", "100ms", "-rpc-timeout", "250ms" ]

# Create the user
RUN adduser --uid 1000 appuser

USER appuser

# Install Air for live reloading
RUN go install github.com/cosmtrek/air@latest

ENTRYPOINT ["./docker/dev-container.sh"]

#######################################################
# Builds the test container for CI
FROM golang:${GO_VERSION}-alpine AS testing

WORKDIR /app

# Create the user
RUN adduser -u1000 -D appuser

USER appuser

# We check if a file exists in /tmp/health.txt to see if the container is healthy
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=10 CMD [ "sh", "-c", "[ -f /tmp/health.txt ]"]

ENTRYPOINT [ "./docker/test-container.sh" ]

#######################################################
# Builds the production binary
FROM golang:${GO_VERSION}-alpine AS builder_production

WORKDIR /app 

# Go dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY proto ./proto

# Do the build
RUN go build -buildvcs=false -o ecfmp-discord ./cmd/ecfmp-discord

# Make sure the binary is executable
RUN chmod +x ecfmp-discord

#######################################################
# This container runs the actual binary in production
FROM alpine:3.18 as production

# Create the user
RUN adduser -u1000 -D appuser

COPY --from=builder_production --chown=appuser:appuser ./app/ecfmp-discord /ecfmp-discord
COPY --from=builder_base /usr/local/bin/grpc_health_probe /usr/local/bin/grpc_health_probe

EXPOSE 80

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 CMD [ "grpc_health_probe", "-addr", "localhost:80", "-connect-timeout", "100ms", "-rpc-timeout", "250ms" ]

USER appuser

ENTRYPOINT [ "/ecfmp-discord" ]
