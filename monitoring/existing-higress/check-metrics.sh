#!/bin/bash
# 检查 Prometheus 中的 Higress 指标

export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

echo "=========================================="
echo "检查 Higress 指标可用性"
echo "=========================================="
echo ""

PROM_POD=$(kubectl get pod -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}')

if [ -z "$PROM_POD" ]; then
    echo "❌ 未找到 Prometheus Pod"
    exit 1
fi

echo "✅ Prometheus Pod: ${PROM_POD}"
echo ""

# 测试查询函数
test_query() {
    local query="$1"
    local description="$2"

    echo "测试: $description"
    echo "查询: $query"

    result=$(kubectl exec -n monitoring ${PROM_POD} -- curl -s "http://localhost:9090/api/v1/query?query=$(echo $query | sed 's/ /%20/g')" | jq -r '.data.result[] | .value[1]' 2>/dev/null)

    if [ -n "$result" ] && [ "$result" != "null" ]; then
        # 转换为可读格式
        bytes=$(echo "$result" | awk '{print int($1)}')
        if [ "$bytes" -gt 0 ] 2>/dev/null; then
            mb=$((bytes / 1024 / 1024))
            gb=$((bytes / 1024 / 1024 / 1024))
            if [ "$gb" -gt 0 ]; then
                echo "✅ 结果: ${gb}GB (${bytes} bytes)"
            else
                echo "✅ 结果: ${mb}MB (${bytes} bytes)"
            fi
        else
            echo "✅ 结果: $result"
        fi
    else
        echo "❌ 无数据"
    fi
    echo ""
}

echo "=========================================="
echo "容器级别指标（应该显示 ~2GB）"
echo "=========================================="
echo ""

test_query \
    'container_memory_working_set_bytes{namespace="agent",pod=~"higress-allinone.*",container=""}' \
    "Pod Working Set 内存（总内存）"

test_query \
    'container_memory_rss{namespace="agent",pod=~"higress-allinone.*",container=""}' \
    "Pod RSS（常驻内存）"

test_query \
    'container_memory_cache{namespace="agent",pod=~"higress-allinone.*",container=""}' \
    "Pod Cache（页缓存）"

test_query \
    'container_spec_memory_limit_bytes{namespace="agent",pod=~"higress-allinone.*",container=""}' \
    "Pod 内存限制"

echo "=========================================="
echo "Istio Agent 指标（应该显示 ~150MB）"
echo "=========================================="
echo ""

test_query \
    'istio_agent_go_memstats_sys_bytes{namespace="agent",pod=~"higress-allinone.*"}' \
    "Go 进程 Sys 内存"

test_query \
    'istio_agent_go_memstats_heap_inuse_bytes{namespace="agent",pod=~"higress-allinone.*"}' \
    "Go Heap 内存"

test_query \
    'istio_agent_go_goroutines{namespace="agent",pod=~"higress-allinone.*"}' \
    "Goroutines 数量"

echo "=========================================="
echo "诊断建议"
echo "=========================================="
echo ""

echo "检查容器指标是否可用："
WORKING_SET=$(kubectl exec -n monitoring ${PROM_POD} -- curl -s "http://localhost:9090/api/v1/query?query=container_memory_working_set_bytes{namespace=\"agent\",pod=~\"higress-allinone.*\",container=\"\"}" | jq -r '.data.result | length')

if [ "$WORKING_SET" == "0" ] || [ "$WORKING_SET" == "null" ]; then
    echo "⚠️  容器指标不可用！"
    echo ""
    echo "可能原因："
    echo "1. Prometheus 没有配置 cAdvisor 抓取"
    echo "2. Pod label 或 annotation 不匹配"
    echo "3. Kubelet metrics service 不可用"
    echo ""
    echo "解决方案："
    echo "1. 检查 Prometheus Targets: http://localhost:9090/targets"
    echo "2. 搜索 'cadvisor' 或 'kubelet' targets"
    echo "3. 如果没有，需要配置 Prometheus 抓取 cAdvisor"
else
    echo "✅ 容器指标可用！"
    echo ""
    echo "下一步："
    echo "1. 在 Grafana 中导入修正版 Dashboard"
    echo "   文件: higress-allinone-dashboard-fixed.json"
    echo "2. 第一个图应该显示 ~2GB（不是 8MB）"
fi

echo ""
echo "=========================================="
echo "完成"
echo "=========================================="
