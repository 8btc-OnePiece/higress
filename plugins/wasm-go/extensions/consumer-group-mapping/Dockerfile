# Consumer Group Mapping Plugin - Dockerfile for Higress
# 使用 Go 1.24 原生编译 + 简化的 Dockerfile
# 参考：https://higress.cn/docs/latest/plugins/custom/

FROM scratch

# 将编译好的 wasm 文件添加到镜像中（根目录）
COPY main.wasm plugin.wasm

# Wasm 插件镜像规范要求的元数据
# 这些 Labels 帮助 Higress Console 正确识别和展示插件
LABEL "higress-plugin.name"="consumer-group-mapping"
LABEL "higress-plugin.version"="1.0.0"
LABEL "higress-plugin.category"="custom"
LABEL "higress-plugin.phase"="AUTHN"
LABEL "higress-plugin.priority"="100"
LABEL "higress-plugin.title.zh-CN"="消费者分组映射"
LABEL "higress-plugin.title.en-US"="Consumer Group Mapping"
LABEL "higress-plugin.description.zh-CN"="在鉴权前将消费者标识替换为分组标识，鉴权后还原，实现多消费者共享认证凭证"
LABEL "higress-plugin.description.en-US"="Replace consumer identity with group identity before authentication, restore after, enabling shared credentials among multiple consumers"
LABEL "higress-plugin.icon-url"="https://img.alicdn.com/imgextra/i2/O1CN01z3z5xz1iVZ2VCvtxL_!!6000000004429-2-tps-42-42.png"

# OCI 镜像元数据（可选但推荐）
LABEL "org.opencontainers.image.authors"="Higress Team"
LABEL "org.opencontainers.image.description"="Consumer Group Mapping Plugin for Higress Gateway"
LABEL "org.opencontainers.image.title"="consumer-group-mapping"
LABEL "org.opencontainers.image.version"="1.0.0"

# Wasm 插件不需要进程，禁用健康检查
HEALTHCHECK NONE
