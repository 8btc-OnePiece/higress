#!/bin/bash
# Higress pprof 火焰图获取脚本

KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"
POD_NAME="higress-allinone-5cc7d4c7d-kgnk9"
NAMESPACE="agent"
OUTPUT_DIR="/tmp/higress-pprof-$(date +%Y%m%d_%H%M%S)"

echo "=================================================="
echo "Higress pprof 火焰图获取工具"
echo "=================================================="
echo ""

# 创建输出目录
mkdir -p "$OUTPUT_DIR"
echo "输出目录: $OUTPUT_DIR"
echo ""

# 检查必要工具
echo "1. 检查必要工具..."
if ! command -v go tool pprof &> /dev/null; then
    echo "❌ go tool pprof 未安装"
    echo "   安装: go install github.com/google/pprof@latest"
    exit 1
fi

if ! command -v dot &> /dev/null; then
    echo "❌ dot (Graphviz) 未安装"
    echo "   macOS: brew install graphviz"
    echo "   Ubuntu: apt-get install graphviz"
    exit 1
fi

if ! command -v火焰图生成器 &> /dev/null; then
    if [ ! -f /tmp/FlameGraph/flamegraph.pl ]; then
        echo "⚠️  FlameGraph 未安装，正在安装..."
        git clone https://github.com/brendangregg/FlameGraph /tmp/FlameGraph 2>/dev/null || {
            echo "❌ 无法下载 FlameGraph"
            exit 1
        }
    fi
fi

echo "✅ 所有必要工具已安装"
echo ""

# 函数：获取 pprof 数据
get_pprof() {
    local name=$1
    local port=$2
    local duration=$3
    local profile_type=$4

    echo "=================================================="
    echo "获取 $name 的 pprof 数据"
    echo "=================================================="
    echo ""

    # 启动端口转发
    echo "启动端口转发: localhost:$port -> Pod:$port"
    export KUBECONFIG="$KUBECONFIG"
    kubectl port-forward -n $NAMESPACE pod/$POD_NAME $port:$port > /dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # 检查端口转发是否成功
    if ! nc -z localhost $port 2>/dev/null; then
        echo "❌ 端口转发失败"
        kill $PF_PID 2>/dev/null
        return 1
    fi
    echo "✅ 端口转发成功 (PID: $PF_PID)"
    echo ""

    # 获取 CPU profile
    if [ "$profile_type" == "cpu" ]; then
        echo "正在采集 CPU profile ($duration 秒)..."
        curl -s "http://localhost:$port/debug/pprof/profile?seconds=$duration" \
            -o "$OUTPUT_DIR/${name}-cpu.prof"

        if [ $? -eq 0 ] && [ -s "$OUTPUT_DIR/${name}-cpu.prof" ]; then
            echo "✅ CPU profile 采集完成"
            echo "   文件: $OUTPUT_DIR/${name}-cpu.prof"

            # 生成火焰图
            echo "正在生成火焰图..."
            go tool pprof -raw "$OUTPUT_DIR/${name}-cpu.prof" \
                | /tmp/FlameGraph/flamegraph.pl \
                > "$OUTPUT_DIR/${name}-cpu.svg"

            if [ $? -eq 0 ]; then
                echo "✅ 火焰图生成完成"
                echo "   文件: $OUTPUT_DIR/${name}-cpu.svg"
            fi

            # 生成调用图
            echo "正在生成调用图..."
            go tool pprof -pdf "$OUTPUT_DIR/${name}-cpu.prof" \
                > "$OUTPUT_DIR/${name}-cpu-callgraph.pdf" 2>/dev/null

            if [ $? -eq 0 ]; then
                echo "✅ 调用图生成完成"
                echo "   文件: $OUTPUT_DIR/${name}-cpu-callgraph.pdf"
            fi

            # Top 耗时函数
            echo ""
            echo "Top 10 耗时函数:"
            go tool pprof -top -nodecount=10 "$OUTPUT_DIR/${name}-cpu.prof" | head -20
        else
            echo "❌ CPU profile 采集失败"
        fi
    fi

    # 获取 Heap profile
    echo ""
    echo "正在采集 Heap profile..."
    curl -s "http://localhost:$port/debug/pprof/heap" \
        -o "$OUTPUT_DIR/${name}-heap.prof"

    if [ $? -eq 0 ] && [ -s "$OUTPUT_DIR/${name}-heap.prof" ]; then
        echo "✅ Heap profile 采集完成"
        echo "   文件: $OUTPUT_DIR/${name}-heap.prof"

        # 生成内存火焰图
        echo "正在生成内存火焰图..."
        go tool pprof -raw "$OUTPUT_DIR/${name}-heap.prof" \
            | /tmp/FlameGraph/flamegraph.pl \
            > "$OUTPUT_DIR/${name}-heap.svg"

        if [ $? -eq 0 ]; then
            echo "✅ 内存火焰图生成完成"
            echo "   文件: $OUTPUT_DIR/${name}-heap.svg"
        fi

        # Top 内存占用
        echo ""
        echo "Top 10 内存占用:"
        go tool pprof -top -nodecount=10 "$OUTPUT_DIR/${name}-heap.prof" | head -20
    fi

    # 停止端口转发
    echo ""
    kill $PF_PID 2>/dev/null
    echo "✅ 端口转发已停止"
    echo ""
}

# 主逻辑
echo "2. 开始采集 pprof 数据..."
echo ""
echo "可用的组件:"
echo "  1. Higress Gateway (端口 8080)"
echo "  2. Pilot Discovery (端口 15014/15080)"
echo ""
echo "选择要采集的组件:"
echo "  1 - Higress Gateway"
echo "  2 - Pilot Discovery"
echo "  3 - 全部"
echo ""
read -p "请输入选择 (1/2/3): " choice

case $choice in
    1)
        get_pprof "higress-gateway" 8080 30 "cpu"
        ;;
    2)
        get_pprof "pilot-discovery" 15014 30 "cpu"
        ;;
    3)
        get_pprof "higress-gateway" 8080 30 "cpu"
        get_pprof "pilot-discovery" 15014 30 "cpu"
        ;;
    *)
        echo "❌ 无效选择"
        exit 1
        ;;
esac

echo "=================================================="
echo "采集完成！"
echo "=================================================="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
echo "文件列表:"
ls -lh "$OUTPUT_DIR"
echo ""
echo "查看火焰图:"
echo "  # 在浏览器中打开 SVG 文件"
echo "  open $OUTPUT_DIR/*.svg"
echo ""
echo "交互式分析:"
echo "  go tool pprof $OUTPUT_DIR/higress-gateway-cpu.prof"
echo "  go tool pprof $OUTPUT_DIR/pilot-discovery-cpu.prof"
echo ""
