BINARY_NAME := repoinjector
BUILD_DIR := bin

.PHONY: build run test clean

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/repoinjector

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
