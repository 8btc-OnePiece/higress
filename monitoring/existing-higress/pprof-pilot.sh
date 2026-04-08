#!/bin/bash
# Higress Pilot Discovery pprof 采集脚本

set -e

KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"
POD_NAME="higress-allinone-5cc7d4c7d-kgnk9"
NAMESPACE="agent"
OUTPUT_DIR="/tmp/higress-pprof-$(date +%Y%m%d_%H%M%S)"
DURATION=${1:-30}  # 默认采样30秒

export KUBECONFIG="$KUBECONFIG"

echo "=================================================="
echo "Pilot Discovery pprof 火焰图获取"
echo "=================================================="
echo "Pod: $POD_NAME"
echo "组件: Pilot Discovery (Istio 控制平面)"
echo "端口: 15014"
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

echo "=================================================="
echo "采集 Pilot Discovery (端口 15014)"
echo "=================================================="

# 启动端口转发
echo "启动端口转发..."
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 15014:15014 > /tmp/pf-15014.log 2>&1 &
PF_PID=$!
sleep 3

# 检查连接
if ! curl -s http://localhost:15014/debug/pprof/ > /dev/null 2>&1; then
    echo "❌ 无法连接到 pprof 端点"
    echo "错误日志:"
    cat /tmp/pf-15014.log
    kill $PF_PID 2>/dev/null || true
    exit 1
fi

echo "✅ pprof 端点已连接"
echo ""

# 采集 CPU profile
echo "正在采集 CPU profile (${DURATION}秒)..."
echo "   (请耐心等待)..."

if curl -s "http://localhost:15014/debug/pprof/profile?seconds=$DURATION" \
        -o "pilot-discovery-cpu.prof" && [ -s "pilot-discovery-cpu.prof" ]; then

    echo "✅ CPU profile 采集完成"
    echo "   大小: $(du -h "pilot-discovery-cpu.prof" | cut -f1)"
    echo ""

    # 生成火焰图
    echo "正在生成火焰图..."
    if go tool pprof -raw "pilot-discovery-cpu.prof" 2>/dev/null | \
       /tmp/FlameGraph/flamegraph.pl > "pilot-discovery-cpu.svg" 2>/dev/null; then
        echo "✅ 火焰图: pilot-discovery-cpu.svg"
        echo "   大小: $(du -h "pilot-discovery-cpu.svg" | cut -f1)"
    else
        echo "⚠️  火焰图生成失败，但 profile 数据已保存"
        echo "   可以使用 go tool pprof 交互式分析"
    fi
    echo ""

    # 显示 Top 10
    echo "Top 10 耗时函数:"
    echo "-----------------------------------"
    go tool pprof -top -nodecount=10 "pilot-discovery-cpu.prof" 2>/dev/null | \
        grep -v "Filename:" | head -15 || echo "无法分析"
    echo "-----------------------------------"
else
    echo "❌ CPU profile 采集失败"
fi

# 采集 Heap profile
echo ""
echo "正在采集 Heap profile..."

if curl -s "http://localhost:15014/debug/pprof/heap" \
        -o "pilot-discovery-heap.prof" && [ -s "pilot-discovery-heap.prof" ]; then

    echo "✅ Heap profile 采集完成"
    echo "   大小: $(du -h "pilot-discovery-heap.prof" | cut -f1)"
    echo ""

    # 生成内存火焰图
    echo "正在生成内存火焰图..."
    if go tool pprof -raw "pilot-discovery-heap.prof" 2>/dev/null | \
       /tmp/FlameGraph/flamegraph.pl > "pilot-discovery-heap.svg" 2>/dev/null; then
        echo "✅ 内存火焰图: pilot-discovery-heap.svg"
    fi
else
    echo "❌ Heap profile 采集失败"
fi

# 采集 Goroutine profile
echo ""
echo "正在采集 Goroutine profile..."

if curl -s "http://localhost:15014/debug/pprof/goroutine" \
        -o "pilot-discovery-goroutine.prof" && [ -s "pilot-discovery-goroutine.prof" ]; then
    echo "✅ Goroutine profile 采集完成"
else
    echo "❌ Goroutine profile 采集失败"
fi

# 停止端口转发
kill $PF_PID 2>/dev/null || true
wait $PF_PID 2>/dev/null || true

echo ""
echo "=================================================="
echo "采集完成！"
echo "=================================================="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
ls -lh "$OUTPUT_DIR"
echo ""
echo "查看火焰图:"
echo "  cd $OUTPUT_DIR"
echo "  open pilot-discovery-cpu.svg"
echo "  open pilot-discovery-heap.svg"
echo ""
echo "交互式分析:"
echo "  cd $OUTPUT_DIR"
echo "  go tool pprof pilot-discovery-cpu.prof"
echo ""
echo "Web UI 分析:"
echo "  cd $OUTPUT_DIR"
echo "  go tool pprof -http=:9999 pilot-discovery-cpu.prof"
echo "  # 然后访问: http://localhost:9999"
echo ""
echo "查看 Goroutine:"
echo "  go tool pprof pilot-discovery-goroutine.prof"
echo ""
