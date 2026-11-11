.PHONY: run build

run:
	go run ./cmd/server

build:
	mkdir -p bin
	go build -o bin/tienda-gorras ./cmd/server
