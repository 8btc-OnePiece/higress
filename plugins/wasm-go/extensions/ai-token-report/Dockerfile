# AI Token Report Plugin Docker Image

FROM scratch

# Copy the wasm file (Higress expects plugin.wasm)
COPY main.wasm /plugin/plugin.wasm

# Label the image with plugin metadata
LABEL maintainer="Higress Team"
LABEL description="AI Token Report Plugin for Higress"
LABEL version="1.0.0"
LABEL plugin-name="ai-token-report"
