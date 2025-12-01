SHELL := /bin/bash
APP=cma

.PHONY: build test fmt

build:
	go build -o bin/$(APP) ./cmd/cma

start: build
	@source .env && bin/$(APP) start -config config.json

test:
	go test ./...

fmt:
	gofmt -w $(shell git ls-files '*.go')
