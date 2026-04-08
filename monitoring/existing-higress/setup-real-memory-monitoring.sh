#!/bin/bash
# 使用 kubectl metrics API 创建内存监控
# 将 Pod 内存数据导出到 Prometheus 可用的 Pushgateway

export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

# 安装 prometheus-pushgateway（如果需要）
echo "检查 Pushgateway..."
if ! kubectl get pod -n monitoring -l app=pushgateway 2>/dev/null | grep -q Running; then
    echo "安装 Prometheus Pushgateway..."
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm install pushgateway prometheus-community/prometheus-pushgateway \
      --namespace monitoring \
      --set service.type=ClusterIP
fi

# 获取 Pushgateway 服务
PGW_URL="http://pushgateway.monitoring.svc.cluster.local:9091"

echo "开始收集 Higress Pod 内存..."
POD_NAME="higress-allinone-5cc7d4c7d-kgnk9"

while true; do
    # 获取内存使用（字节）
    MEMORY_BYTES=$(kubectl top pod -n agent "$POD_NAME" --no-headers | awk '{printf "%.0f", $4*1024*1024}')

    if [ -n "$MEMORY_BYTES" ] && [ "$MEMORY_BYTES" != "0" ]; then
        # 推送到 Prometheus
        cat <<EOF | curl -s --data-binary @- "$PGW_URL/metrics/job/higress_memory/agent/$POD_NAME"
# HELP higress_pod_memory_bytes Higress Pod 内存使用（字节）
# TYPE higress_pod_memory_bytes gauge
higress_pod_memory_bytes{namespace="agent",pod="$POD_NAME"} $MEMORY_BYTES
EOF

        # 获取 CPU 使用
        CPU_CORES=$(kubectl top pod -n agent "$POD_NAME" --no-headers | awk '{printf "%.0f", $2*1000}')
        cat <<EOF | curl -s --data-binary @- "$PGW_URL/metrics/job/higress_cpu/agent/$POD_NAME"
# HELP higress_pod_cpu_cores Higress Pod CPU 使用（毫核）
# TYPE higress_pod_cpu_cores gauge
higress_pod_cpu_cores{namespace="agent",pod="$POD_NAME"} $CPU_CORES
EOF

        echo "$(date '+%Y-%m-%d %H:%M:%S') - 推送指标: 内存=${MEMORY_BYTES} bytes ($((MEMORY_BYTES/1024/1024))MB)"
    fi

    # 每 30 秒采集一次
    sleep 30
done
