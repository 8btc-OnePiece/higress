#!/bin/bash

# 快速版本的 Higress 插件构建和推送脚本
# 支持从任何目录执行

set -e

# 检查参数
if [ $# -lt 1 ]; then
    echo "用法: $0 <插件名> [插件目录] [Dockerfile路径]"
    echo "示例: $0 consumer-group-mapping"
    echo "示例: $0 consumer-group-mapping ./consumer-group-mapping"
    exit 1
fi

PLUGIN_NAME=$1
PLUGIN_DIR=${2:-"./${PLUGIN_NAME}"}
DOCKERFILE=${3:-"${PLUGIN_DIR}/Dockerfile"}

# 自动查找插件目录
if [ ! -d "${PLUGIN_DIR}" ] && [ -d "${PWD}/${PLUGIN_NAME}" ]; then
    PLUGIN_DIR="${PWD}/${PLUGIN_NAME}"
fi

# 获取绝对路径
PLUGIN_DIR=$(cd "${PLUGIN_DIR}" && pwd)

# 如果 Dockerfile 不存在，尝试使用默认路径
if [ ! -f "${DOCKERFILE}" ]; then
    DOCKERFILE="${PLUGIN_DIR}/Dockerfile"
fi

TIMESTAMP=$(date +"%Y%m%d%H%M%S")

echo "🚀 开始构建 Higress 插件: ${PLUGIN_NAME}"
echo "📁 插件目录: ${PLUGIN_DIR}"
echo "📅 版本号: ${TIMESTAMP}"

# 切换到插件目录
cd "${PLUGIN_DIR}"

# 1. 清理旧的 WASM 文件
echo "🧹 清理旧的 WASM 文件..."
rm -f main.wasm

# 2. 执行构建（如果有 build.sh）
if [ -f "./build.sh" ]; then
    echo "🔨 执行构建脚本..."
    chmod +x ./build.sh
    ./build.sh
else
    # 如果没有 build.sh，检查是否有 main.wasm
    if [ ! -f "main.wasm" ]; then
        echo "❌ 错误: 既没有 build.sh 也没有 main.wasm"
        exit 1
    fi
    echo "✅ 使用现有的 main.wasm 文件"
fi

# 3. 构建镜像
echo "🐳 构建 Docker 镜像..."
docker build -t ${PLUGIN_NAME}:${TIMESTAMP} -f "${DOCKERFILE}" .

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
