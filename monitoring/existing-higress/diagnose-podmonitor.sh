#!/bin/bash
# PodMonitor 诊断和修复脚本

set -e

KUBECONFIG="/Users/xiaodian/Desktop/supernode-prod-kubeconfig.yaml"
NAMESPACE="agent"
POD_NAME="higress-allinone-56dfffb4bf-c67kv"

export KUBECONFIG="$KUBECONFIG"

echo "=================================================="
echo "Higress PodMonitor 诊断"
echo "=================================================="
echo ""

# 1. 检查 Pod 端口
echo "1. 检查 Pod 端口配置"
echo "-----------------------------------"
kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.spec.containers[0].ports}' | python3 -c "import sys, json; ports=json.load(sys.stdin); print('\n'.join([f\"{p['name']}: {p['containerPort']}\" for p in ports]))" 2>/dev/null || echo "无法解析端口信息"
echo ""

# 2. 检查 Service 端口
echo "2. 检查 higress-allinone-metrics Service"
echo "-----------------------------------"
kubectl get svc higress-allinone-metrics -n $NAMESPACE -o jsonpath='{.spec.ports[0]}' | python3 -c "import sys, json; p=json.load(sys.stdin); print(f\"端口名: {p['name']}\n端口: {p['port']}\nTargetPort: {p['targetPort']}\")" 2>/dev/null
echo ""

# 3. 测试 metrics 端点
echo "3. 测试 metrics 端点 (通过 Service)"
echo "-----------------------------------"
kubectl exec $POD_NAME -n $NAMESPACE -- curl -s http://localhost:15020/stats/prometheus | head -20 || echo "❌ 无法访问 metrics 端点"
echo ""

# 4. 检查 Prometheus 是否抓取
echo "4. 检查 Prometheus targets"
echo "-----------------------------------"
echo "请在 Prometheus UI 中查看: http://your-prometheus:9090/targets"
echo "搜索: higress-allinone"
echo ""

# 5. 显示 PodMonitor 配置
echo "5. 当前 PodMonitor 配置"
echo "-----------------------------------"
kubectl get podmonitor higress-allinone -n $NAMESPACE -o yaml | grep -A 10 "podMetricsEndpoints"
echo ""

echo "=================================================="
echo "诊断完成"
echo "=================================================="
