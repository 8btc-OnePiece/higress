#!/bin/bash
# Higress All-in-One 内存监控部署脚本
# 用于向现有的 Higress all-in-one 部署添加 Prometheus 监控

set -e

# 设置 KUBECONFIG
export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="agent"
HIGRESS_POD="higress-allinone"

echo "=========================================="
echo "Higress All-in-One 内存监控部署"
echo "=========================================="
echo "KUBECONFIG: ${KUBECONFIG}"
echo "命名空间: ${NAMESPACE}"
echo ""

# 检查 Higress Pod 是否运行
echo "步骤 1/6: 检查 Higress Pod 状态..."
if kubectl get pod -n ${NAMESPACE} -l app=${HIGRESS_POD} >/dev/null 2>&1; then
    POD_NAME=$(kubectl get pod -n ${NAMESPACE} -l app=${HIGRESS_POD} -o jsonpath='{.items[0].metadata.name}')
    echo "✅ 找到 Higress Pod: ${POD_NAME}"

    # 检查 metrics 端口
    echo "检查 metrics 端点..."
    if kubectl exec -n ${NAMESPACE} ${POD_NAME} -- curl -s http://localhost:15020/stats/prometheus >/dev/null 2>&1; then
        echo "✅ Metrics 端点正常 (15020端口)"
    else
        echo "❌ Metrics 端点不可访问"
        exit 1
    fi
else
    echo "❌ 未找到 Higress Pod"
    exit 1
fi
echo ""

# 安装 Prometheus Operator
echo "步骤 2/6: 检查并安装 Prometheus Operator..."
if ! kubectl get namespace monitoring >/dev/null 2>&1; then
    echo "安装 Prometheus Operator..."
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update

    kubectl create namespace monitoring

    helm install prometheus prometheus-community/kube-prometheus-stack \
      --namespace monitoring \
      --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
      --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
      --set prometheus.prometheusSpec.resources.requests.cpu=200m \
      --set prometheus.prometheusSpec.resources.requests.memory=512Mi \
      --set prometheus.prometheusSpec.resources.limits.cpu=1000m \
      --set prometheus.prometheusSpec.resources.limits.memory=2Gi \
      --set grafana.enabled=true \
      --set grafana.adminPassword=admin \
      --wait

    echo "✅ Prometheus Operator 安装完成"
else
    echo "✅ Prometheus Operator 已安装"
fi
echo ""

# 创建 Metrics Service
echo "步骤 3/6: 创建 Higress Metrics Service..."
kubectl apply -f "${SCRIPT_DIR}/higress-metrics-service.yaml"
echo "✅ Metrics Service 已创建"
echo ""

# 创建 PodMonitor
echo "步骤 4/6: 创建 Higress PodMonitor..."
kubectl apply -f "${SCRIPT_DIR}/higress-allinone-podmonitor.yaml"
echo "✅ PodMonitor 已创建"
echo ""

# 应用告警规则
echo "步骤 5/6: 应用内存告警规则..."
# 复制并修改告警规则命名空间
if [ -f "${SCRIPT_DIR}/../higress-memory-alerts.yaml" ]; then
    # 修改命名空间为 agent
    sed 's/namespace: higress-system/namespace: agent/g' "${SCRIPT_DIR}/../higress-memory-alerts.yaml" | \
    sed 's/pod=~"higress-gateway.*"/pod=~"higress-allinone.*"/g' | \
    sed 's/pod=~"higress-controller.*"/pod=~"higress-allinone.*"/g' | \
    kubectl apply -f -
    echo "✅ 告警规则已应用"
else
    echo "⚠️  告警规则文件不存在，跳过"
fi
echo ""

# 验证配置
echo "步骤 6/6: 验证监控配置..."
echo ""
echo "检查 Metrics Service..."
kubectl get svc -n ${NAMESPACE} higress-allinone-metrics
echo ""
echo "检查 PodMonitor..."
kubectl get podmonitor -n ${NAMESPACE}
echo ""
echo "检查 Higress Pod 状态..."
kubectl get pod -n ${NAMESPACE} -l app=${HIGRESS_POD}
echo ""

echo "=========================================="
echo "✅ Higress 内存监控配置完成！"
echo "=========================================="
echo ""
echo "访问方式："
echo ""
echo "1. Grafana Dashboard:"
echo "   kubectl port-forward -n monitoring svc/grafana 3000:3000"
echo "   浏览器访问: http://localhost:3000 (用户名/密码: admin/admin)"
echo ""
echo "2. Prometheus:"
echo "   kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090"
echo "   浏览器访问: http://localhost:9090"
echo ""
echo "3. 关键内存查询 PromQL:"
echo "   - Higress 内存使用:"
echo "     istio_agent_go_memstats_alloc_bytes{namespace=\"agent\"}"
echo ""
echo "   - 内存分配速率:"
echo "     rate(istio_agent_go_memstats_alloc_bytes_total{namespace=\"agent\"}[5m])"
echo ""
echo "   - Go Heap 内存:"
echo "     istio_agent_go_memstats_heap_inuse_bytes{namespace=\"agent\"}"
echo ""
echo "4. 导入 Grafana Dashboard:"
echo "   访问 Grafana → Import → 上传 ${SCRIPT_DIR}/../higress-memory-dashboard.json"
echo "   或使用 dashboard ID: 导入后搜索 'Higress'"
echo ""
echo "监控指标已配置，数据将在几分钟内开始显示！"
