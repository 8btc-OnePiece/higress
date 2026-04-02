#!/bin/bash

# Higress 插件构建和推送脚本
# 用途：构建 WASM 插件，打包成 Docker 镜像，并推送到华为云 SWR

set -e  # 遇到错误时退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查参数
if [ $# -lt 1 ]; then
    echo "用法: $0 <插件名> [Dockerfile路径]"
    echo "示例: $0 my-plugin"
    echo "示例: $0 my-plugin ./Dockerfile.custom"
    exit 1
fi

PLUGIN_NAME=$1
DOCKERFILE_PATH=${2:-"./Dockerfile"}

# 生成时间戳版本号
TIMESTAMP=$(date +"%Y%m%d%H%M%S")
IMAGE_NAME="${PLUGIN_NAME}"
IMAGE_TAG="${TIMESTAMP}"
SWR_REGISTRY="swr.cn-east-3.myhuaweicloud.com"
SWR_NAMESPACE="tob"
SWR_IMAGE_NAME="higress-${PLUGIN_NAME}"

print_info "======================================"
print_info "Higress 插件构建和推送脚本"
print_info "======================================"
print_info "插件名称: ${PLUGIN_NAME}"
print_info "版本号: ${IMAGE_TAG}"
print_info "Dockerfile: ${DOCKERFILE_PATH}"
print_info "SWR 仓库: ${SWR_REGISTRY}/${SWR_NAMESPACE}/${SWR_IMAGE_NAME}:${IMAGE_TAG}"
print_info "======================================"

# 1. 清理旧的 WASM 文件
print_info "步骤 1: 清理旧的 WASM 文件..."
if [ -f "main.wasm" ]; then
    rm -f main.wasm
    print_success "已删除旧的 main.wasm"
else
    print_info "没有找到旧的 main.wasm 文件"
fi

# 2. 检查构建脚本
print_info "步骤 2: 检查构建脚本..."
if [ ! -f "./build.sh" ]; then
    print_error "未找到 build.sh 脚本"
    exit 1
fi

# 3. 执行构建脚本
print_info "步骤 3: 执行构建脚本..."
if chmod +x ./build.sh; then
    ./build.sh
    if [ $? -eq 0 ]; then
        print_success "构建脚本执行成功"
    else
        print_error "构建脚本执行失败"
        exit 1
    fi
else
    print_error "无法执行构建脚本"
    exit 1
fi

# 4. 检查 WASM 文件是否生成
print_info "步骤 4: 检查 WASM 文件..."
if [ ! -f "main.wasm" ]; then
    print_error "构建完成后未找到 main.wasm 文件"
    exit 1
fi
print_success "main.wasm 文件已生成"

# 5. 检查 Dockerfile
print_info "步骤 5: 检查 Dockerfile..."
if [ ! -f "${DOCKERFILE_PATH}" ]; then
    print_error "未找到 Dockerfile: ${DOCKERFILE_PATH}"
    exit 1
fi

# 6. 构建 Docker 镜像
print_info "步骤 6: 构建 Docker 镜像..."
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" -f "${DOCKERFILE_PATH}" .
if [ $? -eq 0 ]; then
    print_success "Docker 镜像构建成功: ${IMAGE_NAME}:${IMAGE_TAG}"
else
    print_error "Docker 镜像构建失败"
    exit 1
fi

# 7. 标记镜像为 SWR 格式
print_info "步骤 7: 标记镜像为 SWR 格式..."
FULL_SWR_IMAGE="${SWR_REGISTRY}/${SWR_NAMESPACE}/${SWR_IMAGE_NAME}:${IMAGE_TAG}"
docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${FULL_SWR_IMAGE}"
if [ $? -eq 0 ]; then
    print_success "镜像标记成功: ${FULL_SWR_IMAGE}"
else
    print_error "镜像标记失败"
    exit 1
fi

# 8. 推送到 SWR
print_info "步骤 8: 推送镜像到华为云 SWR..."
print_info "这可能需要几分钟，请耐心等待..."
docker push "${FULL_SWR_IMAGE}"
if [ $? -eq 0 ]; then
    print_success "镜像推送成功!"
else
    print_error "镜像推送失败"
    exit 1
fi

# 9. 显示结果摘要
print_info "======================================"
print_success "构建和推送完成！"
print_info "======================================"
print_info "本地镜像: ${IMAGE_NAME}:${IMAGE_TAG}"
print_info "SWR 镜像: ${FULL_SWR_IMAGE}"
print_info ""
print_info "使用方法:"
print_info "  在 Higress 中配置插件时使用: ${FULL_SWR_IMAGE}"
print_info "======================================"

# 10. 清理本地镜像（可选）
print_warning "是否要删除本地构建的镜像? (y/N)"
read -r response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    print_info "删除本地镜像..."
    docker rmi "${IMAGE_NAME}:${IMAGE_TAG}" "${FULL_SWR_IMAGE}"
    print_success "本地镜像已删除"
fi

print_success "脚本执行完成!"
