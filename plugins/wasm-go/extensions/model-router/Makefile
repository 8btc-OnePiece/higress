PLUGIN_NAME := model-router
VERSION := $(shell cat VERSION)
IMAGE_NAME := higress-$(PLUGIN_NAME)
IMAGE_TAG := $(IMAGE_NAME):$(VERSION)
REGISTRY ?= docker.swr.cn-east-3.myhuaweicloud.com/btc8_public
REGISTRY_IMAGE ?= $(REGISTRY)/$(IMAGE_TAG)

.PHONY: all build-go build-image push-image clean help

all: build-go build-image

build-go:
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm main.go
	@echo "Build completed: main.wasm"

build-image:
	docker build -t $(REGISTRY_IMAGE) -f Dockerfile .
	@echo "Docker image built: $(REGISTRY_IMAGE)"

push-image:
	docker push $(REGISTRY_IMAGE)
	@echo "Docker image pushed: $(REGISTRY_IMAGE)"

clean:
	rm -f main.wasm
	@echo "Cleaned build artifacts"

help:
	@echo "Available targets:"
	@echo "  make all          - Build wasm and Docker image (default)"
	@echo "  make build-go     - Build Go WASM file"
	@echo "  make build-image   - Build Docker image"
	@echo "  make push-image    - Push Docker image to registry"
	@echo "  make clean         - Clean build artifacts"
	@echo ""
	@echo "Environment variables:"
	@echo "  REGISTRY        - Docker registry (default: $(REGISTRY))"