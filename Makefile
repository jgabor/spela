BINARY := spela
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint install clean

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/spela

test:
	go test -v ./...

lint:
	golangci-lint run

install:
	go install $(LDFLAGS) ./cmd/spela

clean:
	rm -f $(BINARY)
