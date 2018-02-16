BINARY_NAME=ssm-sh
TARGET ?= darwin
ARCH ?= amd64
EXT ?= ""
DOCKER_REPO=itsdalmo/ssm-sh
TRAVIS_TAG ?= ref-$(shell git rev-parse --short HEAD)
SRC=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

default: test

run: test
	@echo "== Run =="
	go run main.go

build: test
	@echo "== Build =="
	go build -o $(BINARY_NAME) -v

test:
	@echo "== Test =="
	gofmt -s -l -w $(SRC)
	go vet -v ./...
	go test -race -v ./...

clean:
	@echo "== Cleaning =="
	rm ssm-sh*

lint:
	@echo "== Lint =="
	golint manager
	golint command

run-docker:
	@echo "== Docker run =="
	docker run --rm $(DOCKER_REPO):latest

build-docker:
	@echo "== Docker build =="
	docker build -t $(DOCKER_REPO):latest .

build-release:
	@echo "== Release build =="
	CGO_ENABLED=0 GOOS=$(TARGET) GOARCH=$(ARCH) go build -ldflags "-X main.version=$(TRAVIS_TAG)" -o $(BINARY_NAME)-$(TARGET)-$(ARCH)$(EXT) -v

.PHONY: default build test build-docker run-docker build-release
