.DEFAULT_GOAL = all

version  := $(shell git rev-list --count HEAD).$(shell git rev-parse --short HEAD)

name     := gce-available-disks
package  := github.com/k-oguma/$(name)
packages := $(shell go list ./... | grep -v /vendor/)

.PHONY: all
all:: dep
all:: build

.PHONY: build
build::
	go build -ldflags "-s -w" .

.PHONY: dep
dep::
	GO111MODULE="on" go mod tidy

.PHONY: lint
lint::
	go vet -v $(packages)

.PHONY: clean
clean::
	git clean -xddff
