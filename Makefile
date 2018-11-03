BINARY_NAME = ssm-sh
DOCKER_REPO = itsdalmo/ssm-sh
TARGET     ?= darwin
ARCH       ?= amd64
EXT        ?= ""

GIT_REF = $(shell git rev-parse --short HEAD)
GIT_TAG = $(if $(TRAVIS_TAG),$(TRAVIS_TAG),ref-$(GIT_REF))

LDFLAGS = -ldflags "-X=main.version=$(GIT_TAG)"
SRC     = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

export GO111MODULE=on

default: test

run: test
	@echo "== Run =="
	go run $(LDFLAGS) main.go

build: test
	@echo "== Build =="
	go build -o $(BINARY_NAME) -v $(LDFLAGS)

test:
	@echo "== Test =="
	gofmt -s -l -w $(SRC)
	go vet -v ./...
	go test -race -v ./...

clean:
	@echo "== Cleaning =="
	@rm -f ssm-sh* || true

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

build-release: test
	@echo "== Release build =="
	CGO_ENABLED=0 GOOS=$(TARGET) GOARCH=$(ARCH) go build $(LDFLAGS) -o $(BINARY_NAME)-$(TARGET)-$(ARCH)$(EXT) -v

.PHONY: default build test build-docker run-docker build-release
