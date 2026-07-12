BINARY := memory

.PHONY: build test lint install

build:
	go build -o $(BINARY) .

test:
	go test ./...

lint:
	go vet ./...

install:
	CGO_ENABLED=0 go install .
