.PHONY: run

ifneq (,$(wildcard .env))
    include .env
    export
endif

run:
	go run ./cmd/bot/main.go
