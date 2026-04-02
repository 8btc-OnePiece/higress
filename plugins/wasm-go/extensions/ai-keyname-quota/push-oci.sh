#!/bin/bash

# AI KeyName Quota Plugin - OCI 镜像推送脚本
# 使用 ORAS 将插件推送到 OCI 镜像仓库

set -e

echo "========================================="
echo "Pushing AI KeyName Quota Plugin"
echo "to OCI Registry with ORAS"
echo "========================================="

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 配置变量（可通过环境变量覆盖）
PLUGIN_NAME="${PLUGIN_NAME:-ai-keyname-quota}"
VERSION="${VERSION:-1.0.0}"
IMAGE_REGISTRY_SERVICE="${IMAGE_REGISTRY_SERVICE:-registry.cn-hangzhou.aliyuncs.com}"
IMAGE_REPOSITORY="${IMAGE_REGISTRY_SERVICE}/2456868764/${PLUGIN_NAME}"

echo "Configuration:"
echo "  Plugin: ${PLUGIN_NAME}"
echo "  Version: ${VERSION}"
echo "  Registry: ${IMAGE_REGISTRY_SERVICE}"
echo "  Repository: ${IMAGE_REPOSITORY}:${VERSION}"
echo ""

# 检查必要文件
if [ ! -f "main.wasm" ]; then
    echo "Error: main.wasm not found"
    echo "Please build the plugin first:"
    echo "  ./build.sh"
    exit 1
fi

if [ ! -f "spec.yaml" ]; then
    echo "Error: spec.yaml not found"
    exit 1
fi

if [ ! -f "README.md" ]; then
    echo "Error: README.md not found"
    exit 1
fi

# 检查 oras 是否安装
if ! command -v oras &> /dev/null; then
    echo "Error: oras is not installed"
    echo ""
    echo "Please install ORAS:"
    echo "  macOS:   brew install oras"
    echo "  Linux:   curl -LO https://oras.land/install.sh | bash"
    echo "  Windows: scoop install oras"
    echo ""
    echo "Or visit: https://oras.land/cli/"
    exit 1
fi

echo "ORAS version:"
oras version
echo ""

# 打包 wasm 文件
echo "Packing wasm file..."
tar czvf plugin.tar.gz main.wasm

if [ $? -ne 0 ]; then
    echo "Error: Failed to pack wasm file"
    exit 1
fi

echo "Wasm file packed successfully"
echo ""

# 登录镜像仓库
echo "Logging in to ${IMAGE_REGISTRY_SERVICE}..."
oras login "${IMAGE_REGISTRY_SERVICE}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to login to registry"
    echo "Please login manually:"
    echo "  oras login ${IMAGE_REGISTRY_SERVICE}"
    exit 1
fi

echo ""

# 推送 OCI 镜像
echo "Pushing OCI image to ${IMAGE_REPOSITORY}:${VERSION}..."
echo ""

oras push "${IMAGE_REPOSITORY}:${VERSION}" \
    ./spec.yaml:application/vnd.module.wasm.spec.v1+yaml \
    ./README.md:application/vnd.module.wasm.doc.v1+markdown \
    ./plugin.tar.gz:application/vnd.oci.image.layer.v1.tar+gzip

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================="
    echo "Push successful!"
    echo "========================================="
    echo ""
    echo "OCI Image: ${IMAGE_REPOSITORY}:${VERSION}"
    echo ""
    echo "You can now use this plugin in Higress:"
    echo ""
    echo "  Via Console:"
    echo "    Image URL: oci://${IMAGE_REPOSITORY}:${VERSION}"
    echo ""
    echo "  Via CRD:"
    echo "    url: oci://${IMAGE_REPOSITORY}:${VERSION}"
    echo ""
    echo "  Example plugin.yaml:"
    echo "    apiVersion: extensions.higress.io/v1alpha1"
    echo "    kind: WasmPlugin"
    echo "    metadata:"
    echo "      name: ai-keyname-quota"
    echo "      namespace: higress-system"
    echo "    spec:"
    echo "      url: oci://${IMAGE_REPOSITORY}:${VERSION}"
    echo "      # ... your config ..."
    echo ""
else
    echo ""
    echo "Push failed!"
    exit 1
fi

# 清理临时文件
rm -f plugin.tar.gz