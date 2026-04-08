---
title: QPS 监控
keywords: [higress, QPS, 监控, 可观测]
description: QPS 监控插件配置参考
---

## 介绍

提供 QPS（每秒请求数）和成功率监控能力，通过 Prometheus Counter 指标暴露，支持按路由、集群、消费者等多维度统计。

## 运行属性

插件执行阶段：`默认阶段`
插件执行优先级：`200`

## 配置说明

| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
|------|----------|----------|--------|------|
| `enable_route_dimension` | bool | 非必填 | true | 是否按路由维度统计 |
| `enable_cluster_dimension` | bool | 非必填 | true | 是否按集群维度统计 |
| `enable_consumer_dimension` | bool | 非必填 | true | 是否按消费者维度统计 |
| `custom_labels` | map[string]string | 非必填 | - | 自定义标签，追加到指标中 |

## 指标说明

插件暴露以下 Prometheus Counter 指标：

| 指标名称 | 类型 | 说明 |
|----------|------|------|
| `qps_monitor_{dimensions}_metric_request_total` | Counter | 请求总数 |
| `qps_monitor_{dimensions}_metric_request_success` | Counter | 成功请求数（2xx, 3xx） |
| `qps_monitor_{dimensions}_metric_request_error_4xx` | Counter | 4xx 错误数 |
| `qps_monitor_{dimensions}_metric_request_error_5xx` | Counter | 5xx 错误数 |

其中 `{dimensions}` 根据配置动态生成，格式为：
```
qps_monitor_route_{route}_cluster_{cluster}_consumer_{consumer}_metric_{metric_name}
```

## Prometheus 查询示例

### QPS 计算

```promql
# 整体 QPS（最近 1 分钟）
irate(qps_monitor_metric_request_total[1m])

# 按路由分组的 QPS
sum by (route) (irate(qps_monitor_route_.*_metric_request_total[1m]))

# 按集群分组的 QPS
sum by (cluster) (irate(qps_monitor_cluster_.*_metric_request_total[1m]))
```

### 成功率计算

```promql
# 整体成功率
sum(irate(qps_monitor_metric_request_success[1m]))
/
sum(irate(qps_monitor_metric_request_total[1m]))

# 按路由分组的成功率
sum by (route) (irate(qps_monitor_route_.*_metric_request_success[1m]))
/
sum by (route) (irate(qps_monitor_route_.*_metric_request_total[1m]))
```

### 错误率计算

```promql
# 4xx 错误率
sum(irate(qps_monitor_metric_request_error_4xx[1m]))
/
sum(irate(qps_monitor_metric_request_total[1m]))

# 5xx 错误率
sum(irate(qps_monitor_metric_request_error_5xx[1m]))
/
sum(irate(qps_monitor_metric_request_total[1m]))
```

## 配置示例

### 基础配置（使用默认值）

```yaml
# 空配置即可，所有维度默认启用
```

### 禁用某些维度

```yaml
enable_consumer_dimension: false  # 不按消费者维度统计
```

### 添加自定义标签

```yaml
custom_labels:
  env: production
  region: cn-hangzhou
```

### 完整配置

```yaml
enable_route_dimension: true
enable_cluster_dimension: true
enable_consumer_dimension: true
custom_labels:
  env: production
  region: cn-hangzhou
```

## Grafana 面板示例

### QPS 面板

```
# Panel 1: 总 QPS
sum(rate(qps_monitor_metric_request_total[1m]))

# Panel 2: QPS by Route (Graph)
sum by (route) (rate(qps_monitor_route_.*_metric_request_total[5m]))

# Panel 3: QPS by Cluster (Graph)
sum by (cluster) (rate(qps_monitor_cluster_.*_metric_request_total[5m]))
```

### 成功率面板

```
# Panel 1: 整体成功率 (Stat)
sum(rate(qps_monitor_metric_request_success[1m]))
/
sum(rate(qps_monitor_metric_request_total[1m]))
* 100

# Panel 2: 成功率趋势 (Graph)
sum(rate(qps_monitor_metric_request_success[5m]))
/
sum(rate(qps_monitor_metric_request_total[5m]))
* 100
```

### 错误分布面板

```
# Panel 1: 4xx 错误率
sum(rate(qps_monitor_metric_request_error_4xx[1m]))
/
sum(rate(qps_monitor_metric_request_total[1m]))
* 100

# Panel 2: 5xx 错误率
sum(rate(qps_monitor_metric_request_error_5xx[1m]))
/
sum(rate(qps_monitor_metric_request_total[1m]))
* 100
```

## 部署示例

```yaml
apiVersion: extensions.istio.io/v1alpha1
kind: WasmPlugin
metadata:
  name: qps-monitor
  namespace: higress-system
spec:
  url: oci://your-registry/qps-monitor:latest
  pluginConfig:
    enable_route_dimension: true
    enable_cluster_dimension: true
    enable_consumer_dimension: true
    custom_labels:
      env: production
```