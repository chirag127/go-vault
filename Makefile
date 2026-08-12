.PHONY: run build test lint proto clean

## run: start all services with docker compose
run:
	docker compose up --build

## build: compile the server binary
build:
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

## test: run all tests with race detector
test:
	go test -race -count=1 ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## vet: run go vet
vet:
	go vet ./...

## proto: regenerate protobuf stubs (requires protoc + plugins in PATH)
proto:
	protoc \
		--proto_path=api/proto \
		--proto_path=$(GOOGLEAPIS_PATH) \
		--go_out=api/gen \
		--go_opt=paths=source_relative \
		--go-grpc_out=api/gen \
		--go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=api/gen \
		--grpc-gateway_opt=paths=source_relative \
		api/proto/shortener.proto

## clean: remove build artifacts
clean:
	rm -rf bin/
