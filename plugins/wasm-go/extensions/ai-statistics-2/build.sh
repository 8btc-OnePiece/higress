#!/bin/bash

# AI Statistics Plugin 构建脚本
# 支持 Go 1.24 原生编译和 TinyGo 编译两种方式

set -e

echo "========================================="
echo "Building AI Statistics Plugin"
echo "========================================="

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 检测编译方式
BUILD_MODE="${BUILD_MODE:-go124}"  # 默认使用 Go 1.24

if [ "$BUILD_MODE" = "go124" ]; then
    echo "Build mode: Go 1.24 Native WASM Compilation"

    # 检查 Go 版本
    if ! command -v go &> /dev/null; then
        echo "Error: Go is not installed"
        echo "Please install Go 1.24 or later:"
        echo "  Visit: https://go.dev/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Go version: $GO_VERSION"

    # 检查版本是否 >= 1.24
    if [ "$(printf '%s\n' "1.24" "$GO_VERSION" | sort -V | head -n1)" != "1.24" ]; then
        echo "Error: Go version must be 1.24 or later"
        echo "Current version: $GO_VERSION"
        echo ""
        echo "Alternatively, you can use TinyGo by setting:"
        echo "  BUILD_MODE=tinygo ./build.sh"
        exit 1
    fi

    echo ""
    echo "Building wasm file with Go 1.24 native compiler..."

    # 使用 Go 1.24 原生编译
    GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm ./

elif [ "$BUILD_MODE" = "tinygo" ]; then
    echo "Build mode: TinyGO Compiler"

    # 检查 tinygo 是否安装
    if ! command -v tinygo &> /dev/null; then
        echo "Error: tinygo is not installed"
        echo "Please install tinygo first:"
        echo "  brew install tinygo"
        echo "Or visit: https://tinygo.org/getting-started/install/"
        exit 1
    fi

    echo "TinyGo version:"
    tinygo version

    echo ""
    echo "Building wasm file with TinyGo..."

    # 使用 TinyGo 编译
    tinygo build \
        -o main.wasm \
        -scheduler=none \
        -target=wasi \
        -gc=custom \
        -heap-size=32MB \
        main.go
else
    echo "Error: Invalid BUILD_MODE '$BUILD_MODE'"
    echo "Supported modes: go124 (default), tinygo"
    exit 1
fi

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
    echo "   docker build -t ai-statistics:2.0.0 ."
    echo ""
    echo "2. Push to registry:"
    echo "   docker tag ai-statistics:2.0.0 your-registry/ai-statistics:2.0.0"
    echo "   docker push your-registry/ai-statistics:2.0.0"
    echo ""
    echo "3. Configure in Higress:"
    echo "   Use image: your-registry/ai-statistics:2.0.0"
else
    echo ""
    echo "Build failed!"
    exit 1
fi

