#!/bin/bash

# AI Route Monitor Plugin 构建脚本
# 支持 Go 1.24 原生编译

set -e

echo "========================================="
echo "Building AI Route Monitor Plugin"
echo "========================================="

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 检查 Go 版本
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Please install Go 1.24 or later:"
    echo "  Visit: https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Go version: $GO_VERSION"

if [ "$(printf '%s\n' "1.24" "$GO_VERSION" | sort -V | head -n1)" != "1.24" ]; then
    echo "Error: Go version must be 1.24 or later"
    echo "Current version: $GO_VERSION"
    exit 1
fi

echo ""
echo "Building wasm file with Go 1.24 native compiler..."

# 使用 Go 1.24 原生编译
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm ./

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================="
    echo "Build successful!"
    echo "========================================="
    echo "Wasm file: $SCRIPT_DIR/main.wasm"
    ls -lh main.wasm
    echo ""
    echo "Next steps:"
    echo "1. Build Docker image:"
    echo "   docker build -t ai-route-monitor:1.0.0 ."
    echo ""
    echo "2. Push to registry:"
    echo "   docker tag ai-route-monitor:1.0.0 your-registry/ai-route-monitor:1.0.0"
    echo "   docker push your-registry/ai-route-monitor:1.0.0"
else
    echo ""
    echo "Build failed!"
    exit 1
fi
