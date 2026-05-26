.PHONY: all build test test-race cover cover-html vet tidy fmt lint clean run install help

GO ?= go
BIN := bin/exec-ecs
PKG := ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X ecs-tool/installer.Version=$(VERSION)

all: build

help:
	@echo "Targets:"
	@echo "  build       Build $(BIN) for the host platform"
	@echo "  test        Run tests"
	@echo "  test-race   Run tests with the race detector"
	@echo "  cover       Run tests and print per-function coverage"
	@echo "  cover-html  Generate and open an HTML coverage report"
	@echo "  vet         Run go vet"
	@echo "  fmt         Run gofmt -w on the tree"
	@echo "  tidy        Run go mod tidy"
	@echo "  lint        Run go vet + gofmt -l (fails if anything needs formatting)"
	@echo "  run         Build and run the binary"
	@echo "  install     Install the binary into $$GOBIN / GOPATH/bin"
	@echo "  clean       Remove build artefacts"

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	$(GO) test $(PKG)

test-race:
	$(GO) test -race $(PKG)

cover:
	$(GO) test -covermode=atomic -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out

cover-html: cover
	$(GO) tool cover -html=coverage.out

vet:
	$(GO) vet $(PKG)

fmt:
	gofmt -w .

tidy:
	$(GO) mod tidy

lint: vet
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt issues in:"; echo "$$out"; exit 1; \
	fi

run: build
	./$(BIN)

install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" .

clean:
	rm -rf bin coverage.out
