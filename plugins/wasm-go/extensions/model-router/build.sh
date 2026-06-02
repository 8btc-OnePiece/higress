#!/bin/bash

set -e

PLUGIN_NAME="model-router"
VERSION=$(cat VERSION 2>/dev/null || echo "1.0.0")

echo "========================================"
echo "Building Model Router Plugin"
echo "========================================"
echo "Plugin: $PLUGIN_NAME"
echo "Version: $VERSION"
echo ""

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check if VERSION file exists, if not create it
if [ ! -f "VERSION" ]; then
    echo "1.0.0" > VERSION
    echo "Created VERSION file with default version"
    VERSION="1.0.0"
fi

# Detect build mode
if [ "$1" = "tinygo" ]; then
    BUILD_MODE="Go 1.24 TinyGo Compilation"
else
    BUILD_MODE="Go 1.24 Native WASM Compilation"
fi

echo "Build mode: $BUILD_MODE"
echo "Go version: $(go version)"
echo ""

# Check Go version
GO_VERSION=$(go version | awk '{print $3}')
REQUIRED_VERSION="1.24"

if [ "$GO_VERSION" != "$REQUIRED_VERSION" ]; then
    echo "Warning: Go version $GO_VERSION detected, but $REQUIRED_VERSION is recommended"
    echo "Build may fail if version is incompatible"
    echo ""
fi

# Build wasm file
echo "Building wasm file with $BUILD_MODE..."

if [ "$1" = "tinygo" ]; then
    # Build with TinyGo
    tinygo build -o main.wasm -target=wasi \
        -scheduler=none \
        -no-stack-overflow-check=true \
        -panic=trap \
        -gc=leaking \
        -deadcode \
        -map \
        -heap-size-bytes=131072 \
        -optimize=-z \
        -optimise-size \
        -no-strip \
        -trim-path \
        -stack-size-bytes=8192 \
        -dump=source \
        main.go
    if [ $? -eq 0 ]; then
        echo "✅ Build successful with TinyGo"
    else
        echo "❌ Build failed with TinyGo"
        exit 1
    fi
else
    # Build with Go 1.24 native compiler
    GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm ./
    if [ $? -eq 0 ]; then
        echo "✅ Build successful with Go 1.24"
    else
        echo "❌ Build failed with Go 1.24"
        exit 1
    fi
fi

# Check if wasm file was created
if [ ! -f "main.wasm" ]; then
    echo "❌ Error: main.wasm not found"
    exit 1
fi

# Get file size
FILE_SIZE=$(ls -lh main.wasm | awk '{print $5}')

echo ""
echo "========================================"
echo "Build successful!"
echo "========================================"
echo "Wasm file: main.wasm"
echo "File size: $FILE_SIZE"
echo ""

# Build Docker image
echo "Building Docker image..."
IMAGE_NAME="higress-$PLUGIN_NAME"
IMAGE_TAG="$IMAGE_NAME:$VERSION"

docker build -t $IMAGE_TAG -f Dockerfile .
if [ $? -eq 0 ]; then
    echo "✅ Docker image built: $IMAGE_TAG"
else
    echo "❌ Docker image build failed"
    exit 1
fi

echo ""
echo "========================================"
echo "Build complete!"
echo "========================================"
echo "Wasm file: main.wasm"
echo "Docker image: $IMAGE_TAG"
echo "File size: $FILE_SIZE"
echo ""
echo "Next steps:"
echo "1. Push to registry:"
echo "   docker push $IMAGE_TAG"
echo ""
echo "2. Configure in Higress:"
echo "   Use image: docker.swr.cn-east-3.myhuaweicloud.com/btc8_public/$IMAGE_TAG"
