#!/bin/bash

# Build and Push Script for AI Success Rate Monitor Plugin

set -e

PLUGIN_NAME="ai-success-rate-monitor"
VERSION=$(cat VERSION 2>/dev/null || echo "1.0.0")
REGISTRY="swr.cn-east-3.myhuaweicloud.com/btc8_public"
IMAGE_NAME="${REGISTRY}/wujieai-${PLUGIN_NAME}:${VERSION}"

echo "========================================"
echo "Building and Pushing ${PLUGIN_NAME}"
echo "========================================"
echo "Version: ${VERSION}"
echo "Image: ${IMAGE_NAME}"
echo ""

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Step 1: Build wasm file
echo "Step 1: Building wasm file..."
./build.sh

if [ ! -f "main.wasm" ]; then
    echo "❌ Error: main.wasm not found after build"
    exit 1
fi

echo "✅ Wasm file built successfully"
echo ""

# Step 2: Build Docker image
echo "Step 2: Building Docker image..."
docker build -t ${IMAGE_NAME} .

if [ $? -eq 0 ]; then
    echo "✅ Docker image built successfully"
else
    echo "❌ Docker image build failed"
    exit 1
fi

echo ""

# Step 3: Push to registry
echo "Step 3: Pushing image to registry..."
docker push ${IMAGE_NAME}

if [ $? -eq 0 ]; then
    echo "✅ Image pushed successfully"
else
    echo "❌ Image push failed"
    exit 1
fi

echo ""
echo "========================================"
echo "Build and Push Complete!"
echo "========================================"
echo "Image: ${IMAGE_NAME}"
echo ""
echo "To use this plugin in Higress:"
echo "1. Add the plugin to your Higress configuration"
echo "2. Set the plugin image to: ${IMAGE_NAME}"
echo "3. Configure the plugin with your DingTalk webhook"
echo ""
