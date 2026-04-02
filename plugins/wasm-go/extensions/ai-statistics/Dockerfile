# AI Statistics Plugin - Dockerfile for Higress
# 使用 Go 1.24 原生编译 + 简化的 Dockerfile
# 参考：https://higress.cn/docs/latest/plugins/custom/

FROM scratch

# 将编译好的 wasm 文件添加到镜像中（根目录）
COPY main.wasm plugin.wasm

# Wasm 插件镜像规范要求的元数据
# 这些 Labels 帮助 Higress Console 正确识别和展示插件
LABEL "higress-plugin.name"="ai-statistics"
LABEL "higress-plugin.version"="1.0.0"
LABEL "higress-plugin.category"="custom"
LABEL "higress-plugin.phase"="AUTHN"
LABEL "higress-plugin.priority"="100"
LABEL "higress-plugin.title.zh-CN"="AI 模型统计"
LABEL "higress-plugin.title.en-US"="AI Model Statistics"
LABEL "higress-plugin.description.zh-CN"="统计 AI 模型的 Token 使用量，支持上报到外部 API"
LABEL "higress-plugin.description.en-US"="Collect AI model token usage statistics and report to external API"
LABEL "higress-plugin.icon-url"="https://img.alicdn.com/imgextra/i2/O1CN01z3z5xz1iVZ2VCvtxL_!!6000000004429-2-tps-42-42.png"

# OCI 镜像元数据（可选但推荐）
LABEL "org.opencontainers.image.authors"="Higress Team"
LABEL "org.opencontainers.image.description"="AI Statistics Plugin for Higress Gateway"
LABEL "org.opencontainers.image.title"="ai-statistics"
LABEL "org.opencontainers.image.version"="1.0.0"

# Wasm 插件不需要进程，禁用健康检查
HEALTHCHECK NONE
