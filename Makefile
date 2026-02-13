BINARY_NAME := repoinjector
BUILD_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X github.com/ezer/repoinjector/internal/cmd.version=$(VERSION)"

PREFIX ?= /usr/local

.PHONY: build run test clean install uninstall

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/repoinjector

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(PREFIX)/bin/$(BINARY_NAME)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY_NAME)
