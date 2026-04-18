.PHONY: build clean run test deps

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_NAME=filetransmitter
BINARY_LINUX=$(BINARY_NAME)_linux
BINARY_WINDOWS=$(BINARY_NAME).exe

# Main paths
MAIN_PATH=./cmd/server
WEB_PATH=./web

# Build flags
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

all: deps build

deps:
	$(GOMOD) download
	$(GOMOD) tidy

build:
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PATH)

build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_LINUX) $(MAIN_PATH)

build-arm:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_LINUX)_arm64 $(MAIN_PATH)

build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_WINDOWS) $(MAIN_PATH)

build-all: build-linux build-arm build-windows

run:
	$(GOCMD) run $(MAIN_PATH)/main.go

clean:
	$(GOCLEAN)
	rm -rf bin/
	rm -f $(BINARY_NAME)

test:
	$(GOTEST) -v ./...

# Frontend
web-install:
	cd $(WEB_PATH) && npm install

web-dev:
	cd $(WEB_PATH) && npm run dev

web-build:
	cd $(WEB_PATH) && npm run build

# Docker
docker-build:
	docker build -t $(BINARY_NAME):latest -f deployments/docker/Dockerfile .

# Package (embed frontend)
package: web-build build
	cp -r $(WEB_PATH)/dist bin/web

.DEFAULT_GOAL := all