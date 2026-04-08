#!/bin/bash
# Higress pprof 简化版 - 快速获取火焰图

set -e

KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"
POD_NAME="higress-allinone-5cc7d4c7d-kgnk9"
NAMESPACE="agent"
OUTPUT_DIR="/tmp/higress-pprof"

export KUBECONFIG="$KUBECONFIG"

echo "=================================================="
echo "Higress pprof 火焰图快速获取"
echo "=================================================="
echo ""

# 创建输出目录
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"

# 安装 FlameGraph（如果需要）
if [ ! -f /tmp/FlameGraph/flamegraph.pl ]; then
    echo "安装 FlameGraph..."
    git clone https://github.com/brendangregg/FlameGraph /tmp/FlameGraph
fi

# 函数：获取单个组件的 profile
capture_profile() {
    local component=$1
    local port=$2
    local duration=${3:-30}

    echo "=================================================="
    echo "采集 $component (端口 $port)"
    echo "=================================================="

    # 启动端口转发（后台）
    kubectl port-forward -n $NAMESPACE pod/$POD_NAME $port:$port > /dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # 检查连接
    if ! curl -s http://localhost:$port/debug/pprof/ > /dev/null; then
        echo "❌ 无法连接到 pprof 端点"
        kill $PF_PID 2>/dev/null
        return 1
    fi

    echo "✅ pprof 端点已连接"

    # 采集 CPU profile (30秒)
    echo "正在采集 CPU profile (${duration}秒)..."
    curl -s "http://localhost:$port/debug/pprof/profile?seconds=$duration" \
        -o "${component}-cpu.prof"

    # 生成火焰图
    if [ -s "${component}-cpu.prof" ]; then
        echo "生成 CPU 火焰图..."
        go tool pprof -raw "${component}-cpu.prof" \
            | /tmp/FlameGraph/flamegraph.pl \
            > "${component}-cpu.svg"
        echo "✅ ${component}-cpu.svg"

        # 显示 Top 10
        echo ""
        echo "Top 10 耗时函数:"
        go tool pprof -top -nodecount=10 "${component}-cpu.prof" | head -15
    fi

    # 采集 Heap profile
    echo ""
    echo "正在采集 Heap profile..."
    curl -s "http://localhost:$port/debug/pprof/heap" \
        -o "${component}-heap.prof"

    if [ -s "${component}-heap.prof" ]; then
        echo "生成内存火焰图..."
        go tool pprof -raw "${component}-heap.prof" \
            | /tmp/FlameGraph/flamegraph.pl \
            > "${component}-heap.svg"
        echo "✅ ${component}-heap.svg"
    fi

    # 停止端口转发
    kill $PF_PID 2>/dev/null
    wait $PF_PID 2>/dev/null || true
    echo "✅ 完成"
    echo ""
}

# 检查哪个组件支持 pprof
echo "检查可用的 pprof 端点..."
echo ""

# 测试 Higress Gateway (8080)
echo "测试 Higress Gateway (8080)..."
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 2

if curl -s http://localhost:8080/debug/pprof/ > /dev/null 2>&1; then
    echo "✅ Higress Gateway 支持 pprof"
    SUPPORTED_GATEWAY=1
else
    echo "❌ Higress Gateway 不支持 pprof"
    SUPPORTED_GATEWAY=0
fi
kill $PF_PID 2>/dev/null
wait $PF_PID 2>/dev/null || true

# 测试 Pilot Discovery (15014)
echo "测试 Pilot Discovery (15014)..."
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 15014:15014 > /dev/null 2>&1 &
PF_PID=$!
sleep 2

if curl -s http://localhost:15014/debug/pprof/ > /dev/null 2>&1; then
    echo "✅ Pilot Discovery 支持 pprof"
    SUPPORTED_PILOT=1
else
    echo "❌ Pilot Discovery 不支持 pprof"
    SUPPORTED_PILOT=0
fi
kill $PF_PID 2>/dev/null
wait $PF_PID 2>/dev/null || true

echo ""
echo "=================================================="
echo "开始采集"
echo "=================================================="
echo ""

# 采集 Higress Gateway
if [ $SUPPORTED_GATEWAY -eq 1 ]; then
    capture_profile "higress-gateway" 8080 30
fi

# 采集 Pilot Discovery
if [ $SUPPORTED_PILOT -eq 1 ]; then
    capture_profile "pilot-discovery" 15014 30
fi

echo "=================================================="
echo "全部完成！"
echo "=================================================="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
ls -lh "$OUTPUT_DIR"/*.svg 2>/dev/null || echo "没有生成火焰图"
echo ""
echo "在浏览器中打开 SVG 文件查看火焰图:"
echo "  open $OUTPUT_DIR/*.svg"
echo ""
echo "交互式分析:"
echo "  cd $OUTPUT_DIR"
echo "  go tool pprof higress-gateway-cpu.prof"
echo "  go tool pprof pilot-discovery-cpu.prof"
echo ""
