# This stage copies all the files we need into
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Building profobuf
COPY proto ./proto
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
RUN apk add --no-cache protobuf
RUN protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./proto/discord.proto
RUN mkdir -p ./ecfmp.vatsim.net/grpc/discord

# Go dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

# Do the build
RUN go build -o /cmd/ecfmp-discord ./cmd/ecfmp-discord

#######################################################

# This container runs the actual binary in production
FROM alpine:3.14 as production

COPY --from=builder /cmd/ecfmp-discord /ecfmp-discord

EXPOSE 80

CMD [ "/ecfmp-discord" ]
