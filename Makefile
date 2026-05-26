.PHONY: build test test-go test-helm lint docker

build:
	go build -o bin/image-optimize-proxy ./cmd/server/

test: test-go test-helm

test-go:
	go test ./... -v -cover -race

test-helm:
	@if ! helm plugin list | grep -q "unittest"; then \
		echo "Installing helm-unittest plugin..."; \
		helm plugin install https://github.com/helm-unittest/helm-unittest.git; \
	fi
	helm unittest charts/image-optimize-proxy

lint:
	go vet ./...

docker:
	docker build -t image-optimize-proxy:dev .
