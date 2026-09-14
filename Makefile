API_PORT ?= 8020
WEB_PORT ?= 5174
API_HOST ?= 127.0.0.1
BIN ?= bin/watcher

.PHONY: install vendor-web dev-api dev-web test build web fmt deploy

install:
	python3 web/scripts/vendor.py
	go mod tidy

vendor-web:
	python3 web/scripts/vendor.py

web:
	bash web/scripts/build.sh production

fmt:
	gofmt -w cmd internal

test:
	go test ./...

build: web
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/watcher

dev-api:
	go run ./cmd/watcher -listen $(API_HOST):$(API_PORT) -data ./data

dev-web:
	python3 web/scripts/dev.py --host $(API_HOST) --port $(WEB_PORT) --backend http://127.0.0.1:$(API_PORT)

deploy:
	bash web/scripts/build.sh production
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/watcher
	bash deploy/install.sh
