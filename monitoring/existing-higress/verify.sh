#!/bin/bash
# Higress All-in-One 监控验证脚本

set -e

export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"
NAMESPACE="agent"
HIGRESS_POD="higress-allinone"

echo "=========================================="
echo "Higress 监控配置验证"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查函数
check_pass() {
    echo -e "${GREEN}✅ $1${NC}"
}

check_fail() {
    echo -e "${RED}❌ $1${NC}"
}

check_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# 1. 检查 Higress Pod
echo "检查 1/7: Higress Pod 状态"
POD_NAME=$(kubectl get pod -n ${NAMESPACE} -l app=${HIGRESS_POD} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$POD_NAME" ]; then
    POD_STATUS=$(kubectl get pod -n ${NAMESPACE} ${POD_NAME} -o jsonpath='{.status.phase}')
    if [ "$POD_STATUS" == "Running" ]; then
        check_pass "Higress Pod 运行正常: ${POD_NAME}"
    else
        check_fail "Higress Pod 状态异常: ${POD_STATUS}"
        exit 1
    fi
else
    check_fail "未找到 Higress Pod"
    exit 1
fi
echo ""

# 2. 检查 metrics 端口
echo "检查 2/7: Metrics 端口 (15020)"
if kubectl exec -n ${NAMESPACE} ${POD_NAME} -- curl -s http://localhost:15020/stats/prometheus >/dev/null 2>&1; then
    check_pass "Metrics 端点可访问"

    # 获取一些指标示例
    METRIC_COUNT=$(kubectl exec -n ${NAMESPACE} ${POD_NAME} -- curl -s http://localhost:15020/stats/prometheus | grep "^istio_agent_" | wc -l)
    echo "    找到 ${METRIC_COUNT} 个 istio_agent 指标"
else
    check_fail "Metrics 端点不可访问"
    exit 1
fi
echo ""

# 3. 检查 Metrics Service
echo "检查 3/7: Metrics Service"
if kubectl get svc -n ${NAMESPACE} higress-allinone-metrics >/dev/null 2>&1; then
    check_pass "Metrics Service 已创建"

    # 检查 endpoints
    ENDPOINTS=$(kubectl get endpoints -n ${NAMESPACE} higress-allinone-metrics -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | wc -w)
    if [ "$ENDPOINTS" -gt 0 ]; then
        echo "    Service endpoints: ${ENDPOINTS}"
    else
        check_warn "Service 没有可用的 endpoints"
    fi
else
    check_fail "Metrics Service 未创建"
fi
echo ""

# 4. 检查 PodMonitor
echo "检查 4/7: PodMonitor"
if kubectl get podmonitor -n ${NAMESPACE} >/dev/null 2>&1; then
    check_pass "PodMonitor 已创建"

    PODMONITOR_COUNT=$(kubectl get podmonitor -n ${NAMESPACE} | grep -v NAME | wc -l)
    echo "    找到 ${PODMONITOR_COUNT} 个 PodMonitor"
else
    check_fail "PodMonitor 未创建"
fi
echo ""

# 5. 检查 Prometheus
echo "检查 5/7: Prometheus Operator"
if kubectl get namespace monitoring >/dev/null 2>&1; then
    check_pass "Monitoring 命名空间存在"

    # 检查 Prometheus Pods
    PROM_PODS=$(kubectl get pod -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | wc -w)
    if [ "$PROM_PODS" -gt 0 ]; then
        check_pass "Prometheus Pods 运行中 (${PROM_PODS} 个)"
    else
        check_warn "Prometheus Pods 未找到"
    fi

    # 检查 Grafana
    GRAFANA_PODS=$(kubectl get pod -n monitoring -l app.kubernetes.io/name=grafana -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | wc -w)
    if [ "$GRAFANA_PODS" -gt 0 ]; then
        check_pass "Grafana Pods 运行中 (${GRAFANA_PODS} 个)"
    else
        check_warn "Grafana Pods 未找到"
    fi
else
    check_warn "Monitoring 命名空间不存在（需要先安装 Prometheus Operator）"
fi
echo ""

# 6. 检查告警规则
echo "检查 6/7: 告警规则"
if kubectl get prometheusrule -n ${NAMESPACE} >/dev/null 2>&1; then
    check_pass "PrometheusRule 已创建"

    RULE_COUNT=$(kubectl get prometheusrule -n ${NAMESPACE} -o jsonpath='{.items[*].spec.groups[*].rules[*]}' 2>/dev/null | wc -w)
    echo "    找到 ${RULE_COUNT} 条告警规则"
else
    check_warn "PrometheusRule 未创建"
fi
echo ""

# 7. 测试指标查询
echo "检查 7/7: 测试指标查询"
if [ "$PROM_PODS" -gt 0 ]; then
    PROM_POD=$(kubectl get pod -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}')

    # 查询 Higress 指标
    RESULT=$(kubectl exec -n monitoring ${PROM_POD} -- curl -s 'http://localhost:9090/api/v1/query?query=istio_agent_go_memstats_alloc_bytes{namespace%3D\"agent\"}' | grep -o '"result":[^}]*' | grep -o '"result":\[[^]]*\]' | grep -v '\[\]')

    if [ -n "$RESULT" ] && [ "$RESULT" != '""' ]; then
        check_pass "Prometheus 可以查询到 Higress 指标"
    else
        check_warn "Prometheus 尚未采集到 Higress 指标（可能需要等待几分钟）"
    fi
else
    check_warn "无法测试指标查询（Prometheus 未运行）"
fi
echo ""

echo "=========================================="
echo "验证完成！"
echo "=========================================="
echo ""

# 提供访问信息
echo "访问方式："
echo ""
echo "1. Grafana Dashboard:"
echo "   kubectl port-forward -n monitoring svc/grafana 3000:3000"
echo "   浏览器: http://localhost:3000 (admin/admin)"
echo ""
echo "2. Prometheus:"
echo "   kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090"
echo "   浏览器: http://localhost:9090"
echo ""
echo "3. 查询示例（在 Prometheus UI 中）:"
echo "   istio_agent_go_memstats_alloc_bytes{namespace=\"agent\"}"
echo ""
echo "如需详细配置信息，请查看: cat DEPLOYMENT_SUMMARY.txt"
