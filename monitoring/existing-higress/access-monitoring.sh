#!/bin/bash
# 快速访问 Higress 监控的便捷脚本

export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

echo "=========================================="
echo "Higress 监控快速访问"
echo "=========================================="
echo ""
echo "请选择要访问的服务："
echo "1) Grafana Dashboard (推荐)"
echo "2) Prometheus"
echo "3) 同时启动 Grafana 和 Prometheus"
echo "4) 查看监控状态"
echo "5) 退出"
echo ""
read -p "请输入选项 (1-5): " choice

case $choice in
  1)
    echo "启动 Grafana 端口转发..."
    echo "访问地址: http://localhost:3000"
    echo "用户名: admin"
    echo "密码: admin"
    echo ""
    echo "按 Ctrl+C 停止端口转发"
    kubectl port-forward -n monitoring svc/grafana 3000:3000
    ;;
  2)
    echo "启动 Prometheus 端口转发..."
    echo "访问地址: http://localhost:9090"
    echo ""
    echo "按 Ctrl+C 停止端口转发"
    kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
    ;;
  3)
    echo "同时启动 Grafana 和 Prometheus..."
    echo "Grafana: http://localhost:3000 (admin/admin)"
    echo "Prometheus: http://localhost:9090"
    echo ""
    echo "按 Ctrl+C 停止所有端口转发"
    kubectl port-forward -n monitoring svc/grafana 3000:3000 &
    GRAFANA_PID=$!
    kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
    PROM_PID=$!

    # 等待任意键退出
    trap "kill $GRAFANA_PID $PROM_PID 2>/dev/null; exit" INT TERM
    wait
    ;;
  4)
    echo "=========================================="
    echo "监控状态检查"
    echo "=========================================="
    echo ""
    echo "1. Higress Pods:"
    kubectl get pods -n agent -l app=higress-allinone
    echo ""
    echo "2. Metrics Service:"
    kubectl get svc -n agent higress-allinone-metrics
    echo ""
    echo "3. PodMonitor:"
    kubectl get podmonitor -n agent
    echo ""
    echo "4. Prometheus:"
    kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus
    echo ""
    echo "5. Grafana:"
    kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana
    echo ""
    echo "6. 告警规则:"
    kubectl get prometheusrule -n agent | head -5
    echo ""
    echo "7. 查询示例（复制到 Prometheus UI）:"
    echo "   istio_agent_go_memstats_alloc_bytes{namespace=\"agent\"}"
    ;;
  5)
    echo "退出"
    exit 0
    ;;
  *)
    echo "无效选项"
    exit 1
    ;;
esac
