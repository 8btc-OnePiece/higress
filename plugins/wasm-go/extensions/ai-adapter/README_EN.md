# AI Adapter Plugin for Higress

## Overview

The AI Adapter plugin resolves protocol differences between different AI service channels. When a model needs to access multiple channels, each channel may have inconsistent request URLs, request parameter structures, and response structures. This plugin automatically adapts to these differences at the request and response stages through a configuration-based approach.

## Features

- **Model to Channel Mapping**: Route specific models to different channels
- **Request Transformation**: Support URL rewriting, parameter transformation, format transformation, header transformation
- **Response Transformation**: Support response header transformation and response body transformation
- **Flexible Configuration**: Support JSON format configuration, easy to extend
- **Memory Safety**: Timely memory release, prevent memory leaks

## Use Cases

### Use Case 1: Azure Channel Adaptation

For the `gpt-image-2` model, when routing to the Azure channel, the `image` parameter in the request body needs to be converted to multipart file format.

**Configuration Example:**
```yaml
modelProviderMappings:
  - model: "gpt-image-2"
    provider: "Azure"
    requestTransform:
      type: "format_transform"
      formatTransform:
        targetFormat: "multipart"
        multipartConfig:
          fieldMapping:
            image: "file"  # Map image field to file field
          removeFields:
            - "prompt"    # Remove unnecessary fields
```

### Use Case 2: RightCode Channel Adaptation

For the `gpt-image-2` model, when routing to the RightCode channel, both text-to-image `/v1/images/generations` and image-to-image `/v1/images/edits` should be rewritten to `/v1/images/generations`.

**Configuration Example:**
```yaml
modelProviderMappings:
  - model: "gpt-image-2"
    provider: "RightCode"
    requestTransform:
      type: "url_rewrite"
      urlRewrite:
        fromPattern: "/v1/images/edits"
        toPattern: "/v1/images/generations"
```

### Use Case 3: Qiniu Channel Adaptation

For the `gpt-image-2` model, when routing to the Qiniu channel, no transformation is needed.

**Configuration Example:**
```yaml
modelProviderMappings:
  - model: "gpt-image-2"
    provider: "Qiniu"
    requestTransform:
      type: "none"
```

## Configuration

### Complete Configuration Structure

```yaml
{
  "modelProviderMappings": [
    {
      "model": "Model name (supports wildcard *)",
      "provider": "Channel name",
      "requestTransform": {
        "type": "Transformation type",
        "urlRewrite": {
          "path": "New path",
          "fromPattern": "Match pattern",
          "toPattern": "Replace pattern"
        },
        "paramTransform": {
          "renameMap": {
            "old_param_name": "new_param_name"
          },
          "removeParams": ["parameters to remove"],
          "addParams": {
            "new_param_name": "parameter value"
          }
        },
        "formatTransform": {
          "targetFormat": "Target format",
          "multipartConfig": {
            "fieldMapping": {
              "old_field_name": "new_field_name"
            },
            "addFields": {
              "new_field_name": "field value"
            },
            "removeFields": ["fields to remove"]
          }
        },
        "headerTransform": {
          "addHeaders": {
            "new_header_name": "header value"
          },
          "removeHeaders": ["headers to remove"],
          "renameHeaders": {
            "old_header_name": "new_header_name"
          }
        },
        "bodyTransform": {
          "setValues": {
            "JSON path": "value"
          },
          "removePaths": ["JSON paths to remove"]
        }
      },
      "responseTransform": {
        // Same structure as requestTransform
      }
    }
  ],
  "defaultProvider": "Default channel name",
  "enableRequestTransform": true,
  "enableResponseTransform": true
}
```

## Execution Order

This plugin should execute after the following plugins:

1. **key-auth**: Authentication plugin
2. **model-router**: Model routing plugin
3. **ai-proxy**: Body rewriting plugin

Execution flow:

```
Request → Auth → Model Routing → Body Rewrite → AI Adapter (Request Transform) → Upstream Call
                                                ↓
Response ← AI Adapter (Response Transform) ← Upstream Response
```

## Memory Management

This plugin adopts the following measures to ensure memory safety:

1. **Configuration Reuse**: Configuration is loaded and reused at plugin startup to avoid repeated parsing
2. **Timely Cleanup**: Temporary data in context is automatically cleaned up after request processing
3. **Size Limit**: Request and response body size limited to 100MB
4. **Memory Rebuild**: Rebuild configuration after every 1000 requests or when memory exceeds 200MB

## Dependencies

This plugin depends on the following Higress WASM SDK:

- `github.com/higress-group/proxy-wasm-go-sdk`
- `github.com/higress-group/wasm-go`
- `github.com/tidwall/gjson` (JSON parsing)
- `github.com/tidwall/sjson` (JSON modification)

## Build

```bash
# Build with Go 1.24 native compiler
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm

# Or build with TinyGo
./build.sh
```

## Deployment

### Docker Image

```bash
# Build image
docker build -t ai-adapter:1.0.0 .

# Push to registry
docker push your-registry/ai-adapter:1.0.0
```

### Higress Configuration

Configure plugin in Higress:

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: ai-adapter
  namespace: higress-system
spec:
  image: your-registry/ai-adapter:1.0.0
  priority: 500  # Ensure execution after ai-proxy
  phase: AUTHN
  matchRules:
    - config:
        modelProviderMappings:
          - model: "gpt-image-2"
            provider: "Azure"
            requestTransform:
              type: "format_transform"
              formatTransform:
                targetFormat: "multipart"
                multipartConfig:
                  fieldMapping:
                    image: "file"
        enableRequestTransform: true
        enableResponseTransform: false
```

## Notes

1. **Provider Identification**: Plugin identifies channel through request header `X-Provider` or Envoy property `wasm.ai_provider`
2. **Base64 Image Processing**: Supports automatic conversion of Base64 encoded image data URI to multipart file format
3. **Query Parameter Preservation**: Query parameters from original request are preserved during URL rewriting
4. **Error Handling**: Logs errors when transformation fails, but doesn't block request processing
5. **Performance Consideration**: Complex format transformations (like multipart) have some performance overhead

## Version History

### v1.0.0
- Initial version
- Support model to channel mapping
- Support multiple request/response transformation types
- Support multipart format transformation
- Memory safety guarantee

## License

Apache License 2.0