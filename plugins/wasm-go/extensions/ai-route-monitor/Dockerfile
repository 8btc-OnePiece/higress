# AI Route Monitor Plugin - Dockerfile for Higress
# 使用 Go 1.24 原生编译

FROM scratch

# 将编译好的 wasm 文件添加到镜像中（根目录）
COPY main.wasm plugin.wasm

LABEL "higress-plugin.name"="ai-route-monitor"
LABEL "higress-plugin.version"="1.0.0"
LABEL "higress-plugin.category"="custom"
LABEL "higress-plugin.phase"="AUTHN"
LABEL "higress-plugin.priority"="100"
LABEL "higress-plugin.title.zh-CN"="AI 路由业务监控"
LABEL "higress-plugin.title.en-US"="AI Route Business Monitor"
LABEL "higress-plugin.description.zh-CN"="监控大模型路由的业务指标，包括 QPS、成功率、耗时，支持 Grafana 展示"
LABEL "higress-plugin.description.en-US"="Monitor AI routing business metrics including QPS, success rate, and latency for Grafana visualization"

HEALTHCHECK NONE
