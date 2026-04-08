# AI Proxy Plugin Docker Image

FROM scratch

# Copy the wasm file (Higress expects plugin.wasm)
COPY main.wasm /plugin/plugin.wasm

# Label the image with plugin metadata
LABEL maintainer="Higress Team"
LABEL description="AI Proxy Plugin for Higress - Multi-protocol AI service gateway"
LABEL version="1.0.0"
LABEL plugin-name="ai-proxy"
