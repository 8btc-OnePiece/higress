#!/bin/bash
# Higress 监控一键部署脚本
# 用于配置 Higress 的 Prometheus 监控（包括内存监控）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HIGRESS_PROJECT="/Users/xiaodian/IdeaProjects/higress"

echo "=========================================="
echo "Higress 内存监控配置"
echo "=========================================="
echo ""

# 步骤1: 安装 Prometheus Operator
echo "步骤 1/6: 检查并安装 Prometheus Operator..."
if ! kubectl get namespace monitoring >/dev/null 2>&1; then
    echo "安装 Prometheus Operator..."
    bash "${SCRIPT_DIR}/install-prometheus-operator.sh"
else
    echo "✅ Prometheus Operator 已安装"
fi
echo ""

# 步骤2: 应用 PodMonitor 配置
echo "步骤 2/6: 应用 Higress PodMonitor 配置..."
kubectl apply -f "${SCRIPT_DIR}/higress-gateway-podmonitor.yaml"
kubectl apply -f "${SCRIPT_DIR}/higress-controller-podmonitor.yaml"
echo "✅ PodMonitor 配置已应用"
echo ""

# 步骤3: 更新 Higress 配置启用监控
echo "步骤 3/6: 更新 Higress 配置..."
cd "${HIGRESS_PROJECT}/helm/core"
helm upgrade --install higress . \
  --namespace higress-system \
  --values "${SCRIPT_DIR}/higress-values-with-metrics.yaml" \
  --wait
echo "✅ Higress 配置已更新"
echo ""

# 步骤4: 等待 Higress Pods 重启
echo "步骤 4/6: 等待 Higress Pods 就绪..."
kubectl wait --for=condition=ready pod -l app=higress-gateway -n higress-system --timeout=180s
kubectl wait --for=condition=ready pod -l app=higress-controller -n higress-system --timeout=180s
echo "✅ Higress Pods 已就绪"
echo ""

# 步骤5: 导入 Grafana Dashboard
echo "步骤 5/6: 配置 Grafana Dashboard..."
# 通过 Grafana API 导入 dashboard
GRAFANA_URL="http://localhost:3000"
GRAFANA_USER="admin"
GRAFANA_PASSWORD="admin"

# 启动 port-forward（后台运行）
kubectl port-forward -n monitoring svc/grafana 3000:3000 >/dev/null 2>&1 &
PF_PID=$!
sleep 5

# 导入 dashboard
DASHBOARD_JSON=$(cat "${SCRIPT_DIR}/higress-memory-dashboard.json")
curl -X POST "${GRAFANA_URL}/api/dashboards/db" \
  -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
  -H "Content-Type: application/json" \
  -d "{\"dashboard\": ${DASHBOARD_JSON}, \"overwrite\": true}" >/dev/null 2>&1 || echo "⚠️  Dashboard 导入失败（请手动导入）"

# 停止 port-forward
kill $PF_PID 2>/dev/null || true
echo "✅ Grafana Dashboard 配置完成"
echo ""

# 步骤6: 验证监控配置
echo "步骤 6/6: 验证监控配置..."
echo ""
echo "检查 PodMonitor 状态..."
kubectl get podmonitor -n higress-system
echo ""
echo "检查 Prometheus targets..."
kubectl get prometheus -n monitoring
echo ""
echo "检查 Higress Pods 状态..."
kubectl get pods -n higress-system
echo ""

echo "=========================================="
echo "✅ Higress 内存监控配置完成！"
echo "=========================================="
echo ""
echo "访问方式："
echo "1. Grafana Dashboard:"
echo "   kubectl port-forward -n monitoring svc/grafana 3000:3000"
echo "   浏览器访问: http://localhost:3000 (用户名/密码: admin/admin)"
echo "   搜索: 'Higress Memory Monitoring'"
echo ""
echo "2. Prometheus:"
echo "   kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090"
echo "   浏览器访问: http://localhost:9090"
echo ""
echo "3. 常用内存查询 PromQL:"
echo "   - Gateway 内存使用:"
echo "     container_memory_working_set_bytes{namespace=\"higress-system\", pod=~\"higress-gateway.*\"}"
echo ""
echo "   - 内存使用率:"
echo "     rate(container_memory_working_set_bytes{namespace=\"higress-system\"}[5m])"
echo ""
echo "   - Go Heap 内存:"
echo "     go_memstats_heap_inuse_bytes{namespace=\"higress-system\"}"
echo ""
echo "监控指标已配置，数据将在几分钟内开始显示！"
