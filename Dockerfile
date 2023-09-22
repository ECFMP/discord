FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./

RUN go mod download
COPY *.go ./
RUN go build -o /ecfmp-discord

EXPOSE 80

FROM alpine:3.14 as base

COPY --from=builder /ecfmp-discord /ecfmp-discord

CMD [ "/ecfmp-discord" ]
