.PHONY: build test lint docker

build:
	go build -o bin/image-optimize-proxy ./cmd/server/

test:
	go test ./... -v -cover -race

lint:
	go vet ./...

docker:
	docker build -t image-optimize-proxy:dev .
