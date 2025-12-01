APP=cma

.PHONY: build test fmt

build:
	go build ./...

test:
	go test ./...

fmt:
	gofmt -w $(shell git ls-files '*.go')
