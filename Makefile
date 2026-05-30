.PHONY: run build clean

BINARY_NAME=programmable-assistant
BUILD_DIR=build

ifneq (,$(wildcard .env))
    include .env
    export
endif

run:
	go run ./cmd/bot/main.go

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/bot/main.go

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME).exe
