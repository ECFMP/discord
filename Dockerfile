# This stage copies all the files we need into
ARG GO_VERSION
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app

# Install gRPC Health Probe
RUN set -ex \
    && apk add --no-cache curl \
    && curl -fSL https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.19/grpc_health_probe-linux-amd64 -o /usr/local/bin/grpc_health_probe \
    && chmod +x /usr/local/bin/grpc_health_probe

# Go dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY proto ./proto

# Do the build
RUN go build -o /cmd/ecfmp-discord ./cmd/ecfmp-discord

#######################################################

# This container runs the actual binary in production
FROM alpine:3.14 as production

COPY --from=builder /cmd/ecfmp-discord /ecfmp-discord

EXPOSE 80

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 CMD [ "grpc_health_probe", "-addr", "localhost:80", "-connect-timeout", "250ms", "-rpc-timeout", "100ms" ]

# Create the user
RUN adduser -u1000 -D appuser
USER appuser

CMD [ "/ecfmp-discord" ]
