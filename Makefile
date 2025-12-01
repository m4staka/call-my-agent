SHELL := /bin/bash
APP=bin/cma

.PHONY: build test fmt

build:
	go build -o $(APP) ./cmd/cma

start: build
	@source .env && $(APP) start -config $(HOME)/.cma/config.json

test:
	go test ./...

fmt:
	gofmt -w $(shell git ls-files '*.go')
