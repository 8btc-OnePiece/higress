# Key Auth Plugin - Dockerfile for Higress
# 使用官方 Makefile 编译

FROM scratch

# 将官方 Makefile 生成的 plugin.wasm 添加到镜像中
COPY plugin.wasm plugin.wasm

# Wasm 插件镜像规范要求的元数据
# 这些 Labels 帮助 Higress Console 正确识别和展示插件
LABEL "higress-plugin.name"="key-auth"
LABEL "higress-plugin.version"="1.0.0"
LABEL "higress-plugin.category"="auth"
LABEL "higress-plugin.phase"="AUTHN"
LABEL "higress-plugin.priority"="321"
LABEL "higress-plugin.title.zh-CN"="Key 认证"
LABEL "higress-plugin.title.en-US"="Key Auth"
LABEL "higress-plugin.description.zh-CN"="基于 API Key 进行认证鉴权"
LABEL "higress-plugin.description.en-US"="API Key authentication"
LABEL "higress-plugin.icon-url"="https://img.alicdn.com/imgextra/i4/O1CN01BPFGlT1pGZ2VDLgaH_!!6000000005333-2-tps-42-42.png"

# OCI 镜像元数据（可选但推荐）
LABEL "org.opencontainers.image.authors"="Higress Team"
LABEL "org.opencontainers.image.description"="Key Auth Plugin for Higress Gateway"
LABEL "org.opencontainers.image.title"="key-auth"
LABEL "org.opencontainers.image.version"="1.0.0"

# Wasm 插件不需要进程，禁用健康检查
HEALTHCHECK NONE
