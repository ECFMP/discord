# ECFMP Discord

ECFMP Discord Service

# Developing Locally

A docker-compose file is provided for development convenience.

## Before You Start

Before you start the containers, make sure you run `make protobuf`. This will generate the protobuf files that this
service requires.

## Running Tests

Tests can be run using `make test`, which will run the tests in the `go_testing` container, allowing it to access
MongoDB.
