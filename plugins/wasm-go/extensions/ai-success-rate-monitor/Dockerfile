# AI Success Rate Monitor Plugin Docker Image

FROM scratch

# Copy wasm file (Higress expects plugin.wasm)
COPY main.wasm /plugin/plugin.wasm

# Label image with plugin metadata
LABEL maintainer="Higress Team"
LABEL description="AI Success Rate Monitor Plugin for Higress - Monitor API success rate and send DingTalk alerts"
LABEL version="1.0.0"
LABEL plugin-name="ai-success-rate-monitor"
