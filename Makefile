.PHONY: build test fmt lint run-generator run-lint

GO ?= go

build:
	$(GO) build ./cmd/generator ./cmd/lint

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint: fmt
	$(GO) vet ./...

run-generator:
	$(GO) run ./cmd/generator

run-lint:
	$(GO) run ./cmd/lint -dir ./content

