# ECFMP Discord

ECFMP Discord Service. This service is responsible for interacting with ECFMP's Discord server in order to be notify flow managers of flow measure updates.

# Local Development

A docker-compose file is provided for development convenience.

## Before You Start

Before you start the containers, make sure you run `make protobuf`. This will generate the protobuf files that this
service requires for other services to communicate with this.

Copy the `.env.dev.example` file to `.env` and set the bot token. This bot should be allowed to access the channel that you intend to publish
messages onto.

## Devcontainer

A devcontainer setup is provided for development.

## Running Tests

Tests can be run using `make test`, which will run the tests in the main container. If you're in the dev container, you can run the tests
by running `go test -v ./...`.

# Integrating

For how to integrate with this service, check out [ECFMP's protobuf repo](https://github.com/ECFMP/ecfmp-protobuf), which contains the protocol.
