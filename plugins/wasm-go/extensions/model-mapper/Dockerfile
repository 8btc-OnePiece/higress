# Model Mapper Plugin Docker Image
FROM scratch

# Copy wasm file (Higress expects plugin.wasm)
COPY main.wasm /plugin/plugin.wasm

# Label image with plugin metadata
LABEL maintainer="Higress Team"
LABEL description="Model Mapper Plugin for Higress"
LABEL version="1.0.0"
LABEL plugin-name="model-mapper"
