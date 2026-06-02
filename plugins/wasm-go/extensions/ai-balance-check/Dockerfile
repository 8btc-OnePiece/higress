# AI Balance Check Plugin Docker Image

FROM scratch

# Copy wasm file (Higress expects plugin.wasm)
COPY main.wasm /plugin/plugin.wasm

# Label image with plugin metadata
LABEL maintainer="Higress Team"
LABEL description="AI Balance Check Plugin for Higress"
LABEL version="1.0.0"
LABEL plugin-name="ai-balance-check"
