#!/bin/bash

# 快速版本的 Higress 插件构建和推送脚本
# 简洁版本，适合快速使用

set -e

# 参数检查
if [ $# -lt 1 ]; then
    echo "用法: $0 <插件名> [Dockerfile路径]"
    echo "示例: $0 my-plugin"
    exit 1
fi

PLUGIN_NAME=$1
DOCKERFILE=${2:-"./Dockerfile"}
TIMESTAMP=$(date +"%Y%m%d%H%M%S")

echo "🚀 开始构建 Higress 插件: ${PLUGIN_NAME}"
echo "📅 版本号: ${TIMESTAMP}"

# 1. 清理旧的 WASM 文件
echo "🧹 清理旧的 WASM 文件..."
rm -f main.wasm

# 2. 执行构建
echo "🔨 执行构建脚本..."
./build.sh

# 3. 构建镜像
echo "🐳 构建 Docker 镜像..."
docker build -t ${PLUGIN_NAME}:${TIMESTAMP} -f ${DOCKERFILE} .

# 4. 标记镜像
echo "🏷️  标记镜像..."
SWR_IMAGE="swr.cn-east-3.myhuaweicloud.com/tob/higress-${PLUGIN_NAME}:${TIMESTAMP}"
docker tag ${PLUGIN_NAME}:${TIMESTAMP} ${SWR_IMAGE}

# 5. 推送到 SWR
echo "📤 推送到华为云 SWR..."
docker push ${SWR_IMAGE}

echo "✅ 完成！"
echo "📦 本地镜像: ${PLUGIN_NAME}:${TIMESTAMP}"
echo "🌐 SWR 镜像: ${SWR_IMAGE}"
echo ""
echo "💡 在 Higress 中使用: ${SWR_IMAGE}"
