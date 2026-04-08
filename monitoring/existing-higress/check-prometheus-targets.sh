#!/bin/bash
# 检查 Prometheus 是否在抓取 Higress 指标

set -e

KUBECONFIG="/Users/xiaodian/Desktop/supernode-prod-kubeconfig.yaml"

export KUBECONFIG="$KUBECONFIG"

echo "=================================================="
echo "检查 Prometheus 抓取状态"
echo "=================================================="
echo ""

# 1. 找到 Prometheus Pod
echo "1. 查找 Prometheus Pod"
echo "-----------------------------------"
PROM_POD=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
echo "Prometheus Pod: $PROM_POD"
echo ""

# 2. 检查 Prometheus 配置
echo "2. 检查 Prometheus 是否加载了 PodMonitor"
echo "-----------------------------------"
kubectl exec $PROM_POD -n monitoring -- cat /etc/prometheus/config_out/prometheus.env.yaml 2>/dev/null | grep -A 5 "higress-allinone" | head -20 || echo "未在配置中找到 higress-allinone"
echo ""

# 3. 检查 ServiceMonitor 是否被识别
echo "3. 检查 PodMonitor 状态"
echo "-----------------------------------"
kubectl get podmonitor -n agent -o jsonpath='{.items[0].status}' | python3 -c "import sys, json; data=json.load(sys.stdin); print(json.dumps(data, indent=2))" 2>/dev/null || echo "无法获取状态"
echo ""

# 4. 检查 Prometheus Operator 日志
echo "4. 检查 Prometheus Operator 日志 (最后 20 行)"
echo "-----------------------------------"
OPERATOR_POD=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$OPERATOR_POD" ]; then
    kubectl logs $OPERATOR_POD -n monitoring --tail=20 2>/dev/null | grep -i "higress\|podmonitor" || echo "没有找到相关日志"
else
    echo "未找到 Prometheus Operator Pod"
fi
echo ""

echo "=================================================="
echo "诊断建议"
echo "=================================================="
echo ""
echo "如果 Prometheus 没有抓取 Higress 指标，可能的原因："
echo ""
echo "1. PodMonitor namespace 不匹配"
echo "   - Prometheus 的 podMonitorNamespaceSelector 可能没有包含 'agent' namespace"
echo ""
echo "2. Service 端口名称不匹配"
echo "   - PodMonitor 指向 'stats-prom' 端口"
echo "   - Service 确实有这个端口名"
echo ""
echo "3. Pod 标签不匹配"
echo "   - PodMonitor selector: app=higress-allinone"
echo "   - Pod labels: app=higress-allinone ✓"
echo ""
echo "4. Prometheus 需要重启"
echo "   - 修改 PodMonitor 后，Prometheus 可能需要时间重新加载配置"
echo ""
echo "5. 检查 Prometheus 的 podMonitorSelector"
echo "   运行: kubectl get prometheus prometheus-kube-prometheus-prometheus -n monitoring -o yaml | grep -A 10 podMonitorSelector"
echo ""
