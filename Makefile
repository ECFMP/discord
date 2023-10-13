GO_VERSION = 1.21

test:
	docker compose exec discord go test -v ./...

protobuf:
	cd proto && make discord_proto && make health_proto

build_production: protobuf build_production_container

build_production_container:
	docker build . --target production --build-arg GO_VERSION=$(GO_VERSION)
