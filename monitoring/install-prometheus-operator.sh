#!/bin/bash
# 安装 Prometheus Operator 和配置 Higress 监控

set -e

echo "=========================================="
echo "安装 Prometheus Operator"
echo "=========================================="

# 添加 prometheus-community repo
echo "添加 Helm repository..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 创建命名空间
echo "创建 monitoring 命名空间..."
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

# 安装 Prometheus Operator
echo "安装 Prometheus Operator..."
helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values /Users/xiaodian/IdeaProjects/higress/monitoring/prometheus-operator-values.yaml \
  --wait

echo "✅ Prometheus Operator 安装完成"

# 等待 Pod 启动
echo "等待 Prometheus Pods 启动..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=prometheus -n monitoring --timeout=300s || true
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=grafana -n monitoring --timeout=300s || true

echo "✅ Prometheus 服务已就绪"
echo ""
echo "=========================================="
echo "安装完成！访问信息："
echo "=========================================="
echo "Prometheus: http://localhost:9090"
echo "Grafana: http://localhost:3000 (用户名/密码: admin/admin)"
echo ""
echo "端口转发命令："
echo "  kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090"
echo "  kubectl port-forward -n monitoring svc/grafana 3000:3000"
