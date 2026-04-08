./#!/bin/bash
# Higress pprof 简化版 - 直接可用

set -e

KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"
POD_NAME="higress-allinone-5cc7d4c7d-kgnk9"
NAMESPACE="agent"
OUTPUT_DIR="/tmp/higress-pprof-$(date +%Y%m%d_%H%M%S)"
DURATION=${1:-30}  # 默认采样30秒

export KUBECONFIG="$KUBECONFIG"

echo "=================================================="
echo "Higress pprof 火焰图获取"
echo "=================================================="
echo "Pod: $POD_NAME"
echo "采样时长: ${DURATION}秒"
echo "输出目录: $OUTPUT_DIR"
echo ""

# 创建输出目录
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"

# 安装 FlameGraph（如果需要）
if [ ! -f /tmp/FlameGraph/flamegraph.pl ]; then
    echo "安装 FlameGraph..."
    git clone https://github.com/brendangregg/FlameGraph /tmp/FlameGraph 2>/dev/null || true
fi

# 函数：采集单个组件
capture() {
    local name=$1
    local port=$2

    echo "=================================================="
    echo "采集 $name (端口 $port)"
    echo "=================================================="

    # 启动端口转发
    echo "启动端口转发..."
    kubectl port-forward -n $NAMESPACE pod/$POD_NAME $port:$port > /tmp/pf-$port.log 2>&1 &
    PF_PID=$!
    sleep 3

    # 检查连接
    if ! curl -s http://localhost:$port/debug/pprof/ > /dev/null 2>&1; then
        echo "❌ 无法连接到 pprof 端点"
        cat /tmp/pf-$port.log
        kill $PF_PID 2>/dev/null || true
        return 1
    fi

    echo "✅ pprof 端点已连接"

    # 采集 CPU profile
    echo "正在采集 CPU profile (${DURATION}秒)..."
    echo "   (请耐心等待)..."

    if curl -s "http://localhost:$port/debug/pprof/profile?seconds=$DURATION" \
            -o "${name}-cpu.prof" && [ -s "${name}-cpu.prof" ]; then

        echo "✅ CPU profile 采集完成"
        echo "   大小: $(du -h "${name}-cpu.prof" | cut -f1)"

        # 生成火焰图
        echo "正在生成火焰图..."
        if go tool pprof -raw "${name}-cpu.prof" 2>/dev/null | \
           /tmp/FlameGraph/flamegraph.pl > "${name}-cpu.svg" 2>/dev/null; then
            echo "✅ 火焰图: ${name}-cpu.svg"

            # 获取文件大小
            size=$(du -h "${name}-cpu.svg" | cut -f1)
            echo "   大小: $size"
        else
            echo "⚠️  火焰图生成失败，但 profile 数据已保存"
        fi

        # 显示 Top 10
        echo ""
        echo "Top 10 耗时函数:"
        echo "-----------------------------------"
        go tool pprof -top -nodecount=10 "${name}-cpu.prof" 2>/dev/null | \
            grep -v "Filename:" | head -15 || true
        echo "-----------------------------------"
    else
        echo "❌ CPU profile 采集失败"
    fi

    # 采集 Heap profile
    echo ""
    echo "正在采集 Heap profile..."

    if curl -s "http://localhost:$port/debug/pprof/heap" \
            -o "${name}-heap.prof" && [ -s "${name}-heap.prof" ]; then

        echo "✅ Heap profile 采集完成"

        # 生成内存火焰图
        if go tool pprof -raw "${name}-heap.prof" 2>/dev/null | \
           /tmp/FlameGraph/flamegraph.pl > "${name}-heap.svg" 2>/dev/null; then
            echo "✅ 内存火焰图: ${name}-heap.svg"
        fi
    fi

    # 停止端口转发
    kill $PF_PID 2>/dev/null || true
    wait $PF_PID 2>/dev/null || true

    echo ""
}

# 检查哪个端口可用
echo "检测 pprof 端点..."
echo ""

# 测试 Higress Gateway
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 2

if curl -s http://localhost:8080/debug/pprof/ > /dev/null 2>&1; then
    echo "✅ Higress Gateway (8080) - 支持 pprof"
    SUPPORTED_GATEWAY=1
else
    echo "❌ Higress Gateway (8080) - 不支持 pprof"
    SUPPORTED_GATEWAY=0
fi
kill $PF_PID 2>/dev/null || true
wait $PF_PID 2>/dev/null || true

# 测试 Pilot Discovery
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 15014:15014 > /dev/null 2>&1 &
PF_PID=$!
sleep 2

if curl -s http://localhost:15014/debug/pprof/ > /dev/null 2>&1; then
    echo "✅ Pilot Discovery (15014) - 支持 pprof"
    SUPPORTED_PILOT=1
else
    echo "❌ Pilot Discovery (15014) - 不支持 pprof"
    SUPPORTED_PILOT=0
fi
kill $PF_PID 2>/dev/null || true
wait $PF_PID 2>/dev/null || true

echo ""
echo "=================================================="
echo "开始采集"
echo "=================================================="
echo ""

# 采集 Higress Gateway
if [ $SUPPORTED_GATEWAY -eq 1 ]; then
    capture "higress-gateway" 8080
fi

# 采集 Pilot Discovery
if [ $SUPPORTED_PILOT -eq 1 ]; then
    capture "pilot-discovery" 15014
fi

echo "=================================================="
echo "采集完成！"
echo "=================================================="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
ls -lh "$OUTPUT_DIR"/*.svg 2>/dev/null || echo "没有生成 SVG 文件"
echo ""
echo "查看火焰图:"
echo "  cd $OUTPUT_DIR"
echo "  open *.svg"
echo ""
echo "交互式分析:"
echo "  cd $OUTPUT_DIR"
echo "  go tool pprof higress-gateway-cpu.prof"
echo ""
echo "Web UI 分析:"
echo "  go tool pprof -http=:9999 higress-gateway-cpu.prof"
echo "  # 然后访问: http://localhost:9999"
echo ""
