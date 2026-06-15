#!/bin/bash

set -e

PLUGIN_NAME="ai-request-id-response-header"
VERSION=$(cat VERSION 2>/dev/null || echo "1.0.0")

echo "========================================"
echo "Building AI Request ID Response Header Plugin"
echo "========================================"
echo "Plugin: $PLUGIN_NAME"
echo "Version: $VERSION"
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "VERSION" ]; then
    echo "1.0.0" > VERSION
    echo "Created VERSION file with default version"
    VERSION="1.0.0"
fi

echo "Build mode: Go 1.24 Native WASM Compilation"
echo "Go version: $(go version)"
echo ""

GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm ./

FILE_SIZE=$(ls -lh main.wasm | awk '{print $5}')
echo ""
echo "========================================"
echo "Build successful!"
echo "========================================"
echo "Wasm file: main.wasm"
echo "File size: $FILE_SIZE"
