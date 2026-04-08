#!/bin/bash
# 探索 Prometheus 中所有 Higress 相关指标

export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

echo "=================================================="
echo "1. 启动 Prometheus 端口转发..."
echo "=================================================="

# 后台启动端口转发
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
PF_PID=$!

# 等待端口转发就绪
sleep 5

echo ""
echo "=================================================="
echo "2. 查询所有 Higress Pod 相关的指标..."
echo "=================================================="

# 查询所有包含 namespace="agent" 和 pod=~"higress.*" 的指标
echo "正在查询所有可用指标..."
METRICS=$(curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' | \
  grep -E '(higress|istio|envoy|pilot|galley|citadel)' | \
  sort -u)

echo "找到以下相关指标类:"
echo "$METRICS" | nl

echo ""
echo "=================================================="
echo "3. 详细分析各指标类别..."
echo "=================================================="

# Envoy 指标
echo "--- Envoy 指标 ---"
curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' | grep -i envoy | head -20

# Istio 指标
echo ""
echo "--- Istio 指标 (除了 go_memstats) ---"
curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' | grep -E '^istio_agent' | grep -v go_memstats | head -20

# Go runtime 指标
echo ""
echo "--- Go Runtime 指标 ---"
curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' | grep -E 'go_.*' | head -30

echo ""
echo "=================================================="
echo "4. 检查是否有容器级指标..."
echo "=================================================="

CONTAINER_METRICS=$(curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' | grep -E 'container_.*|kubelet_.*|cadvisor_.*')

if [ -n "$CONTAINER_METRICS" ]; then
  echo "✅ 找到容器级指标:"
  echo "$CONTAINER_METRICS" | head -20
else
  echo "❌ 没有找到容器级指标 (container_*, kubelet_*, cadvisor_*)"
fi

echo ""
echo "=================================================="
echo "5. 测试关键指标是否有数据..."
echo "=================================================="

# 测试一些关键指标
test_metric() {
  local metric=$1
  local result=$(curl -s "http://localhost:9090/api/v1/query?query=${metric}{namespace=\"agent\",pod=~\"higress.*\"}" | \
    jq -r '.data.result | length')
  echo "  $metric: $result 条数据"
}

echo "--- Istio Agent 内存指标 ---"
test_metric "istio_agent_go_memstats_sys_bytes"
test_metric "istio_agent_go_memstats_heap_inuse_bytes"
test_metric "istio_agent_go_memstats_stack_inuse_bytes"

echo ""
echo "--- Envoy 内存指标 (如果存在) ---"
test_metric "envoy_memory_heap_size"
test_metric "envoy_memory_physical_size"

echo ""
echo "--- 进程级指标 ---"
test_metric "process_resident_memory_bytes"
test_metric "process_virtual_memory_bytes"

echo ""
echo "=================================================="
echo "6. 获取实际 Pod 内存使用 (kubectl top)..."
echo "=================================================="

kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9

echo ""
echo "=================================================="
echo "7. 查看所有 Higress Pod 中的容器..."
echo "=================================================="

kubectl get pod -n agent higress-allinone-5cc7d4c7d-kgnk9 -o jsonpath='{.spec.containers[*].name}'

echo ""
echo ""
echo "=================================================="
echo "8. 保存所有可用指标到文件..."
echo "=================================================="

# 保存所有指标名称
curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' > /tmp/all_metrics.txt

# 保存 Higress 相关指标
curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  jq -r '.data[]' | grep -E '(higress|istio|envoy)' > /tmp/higress_metrics.txt

echo "✅ 所有指标已保存到 /tmp/all_metrics.txt"
echo "✅ Higress 相关指标已保存到 /tmp/higress_metrics.txt"
echo "   Higress 相关指标数量: $(wc -l < /tmp/higress_metrics.txt)"

echo ""
echo "=================================================="
echo "9. 清理端口转发..."
echo "=================================================="

kill $PF_PID 2>/dev/null

echo ""
echo "✅ 探索完成！"
echo ""
echo "下一步: 查看文件查看所有可用指标"
echo "  cat /tmp/higress_metrics.txt"
