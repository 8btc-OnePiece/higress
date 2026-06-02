# AI Adapter - Image 字段处理说明

## 概述

AI Adapter 插件支持对请求 body 中的 `image` 数组字段进行智能处理，支持 Base64 和 URL 格式的图片输入并转换为 multipart/form-data 格式。

**重要说明**：
- **只支持 image 数组格式**
- 单个图片需要先转换为数组格式
- 不支持单个字符串格式的 image 字段
- **只支持 Base64 格式的图片，URL 格式在 WASM 环境中会导致死锁**

## 支持的 Image 数组格式

### Image 数组结构
```json
{
  "image": [
    "data:image/png;base64,iVBORw0KGgo...",
    "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD..."
  ]
}
```

**处理方式**：
- 遍历数组中的每个图片元素
- 每个元素独立处理（Base64 解码）
- 为每个元素创建单独的 multipart file 字段
- 字段名格式：`image[0]`, `image[1]`, `image[2]`...

## 支持的图片格式

### Base64 编码图片
- 格式：`data:image/<type>;base64,<data>`
- 支持类型：png, jpeg, jpg, gif, webp, bmp, tiff, svg+xml, x-icon
- **推荐使用此格式**

### URL 图片
- 格式：`http://...` 或 `https://...`
- **不支持**：在 WASM 环境中会导致死锁，会降级为文本处理
- URL 图片会被记录警告，并将原始 URL 作为文本处理
- **必须使用 Base64 格式**

## URL 下载限制

### WASM 环境限制

在 WASM 环境中，**不支持同步 HTTP 请求**，因为：

1. **单线程执行**：WASM 是单线程执行模型
2. **死锁风险**：使用 `sync.WaitGroup` 等待异步回调会导致死锁
3. **执行模型**：HTTP 回调在下一个事件循环中执行，无法同步等待

### 解决方案

**客户端预处理**（唯一可行方案）：
- 在客户端将图片转换为 Base64 格式
- 减少网关端的处理负担
- 提高可靠性和性能
- 避免死锁风险

```javascript
// 客户端示例：将图片转换为 Base64
async function imageToBase64(imageUrl) {
  const response = await fetch(imageUrl);
  const blob = await response.blob();
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => resolve(reader.result);
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
}

// 使用示例
const base64Image = await imageToBase64('https://cdn.wujiebantu.com/upload/.../image.png');
// base64Image 格式: "data:image/png;base64,iVBORw0KGgo..."
```

## 配置示例

### Azure 渠道配置
```yaml
- model: gpt-image-2
  provider: Azure
  requestTransform:
    type: format_transform
    formatTransform:
      targetFormat: multipart
      multipartConfig:
        fieldMapping:
          image: image
```

## Multipart 输出示例

### Image 数组输入
```json
{
  "model": "gpt-image-2",
  "image": [
    "data:image/png;base64,iVBORw0KGgo...",
    "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBD"
  ],
  "prompt": "multiple images"
}
```

**转换为 multipart/form-data**：
```
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW

------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="model"

gpt-image-2
------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="prompt"

multiple images
------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="image[0]"; filename="image[0].png"
Content-Type: image/png

<binary data from base64>
------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="image[1]"; filename="image[1].jpg"
Content-Type: image/jpeg

<binary data from base64>
------WebKitFormBoundary7MA4YWxkTrZu0gW--
```

## 请求示例

### 仅 Base64 格式（推荐）
```bash
curl -X POST http://gateway/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "image": [
      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII=",
      "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBD"
    ],
    "prompt": "process multiple base64 images"
  }'
```

### 混合格式（Base64 + URL）
```bash
curl -X POST http://gateway/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "image": [
      "data:image/png;base64,iVBORw0KGgo...",
      "https://cdn.wujiebantu.com/upload/.../image.png"
    ],
    "prompt": "mixed format example"
  }'
```

**处理结果**：
- Base64 格式：解码后转换为文件流
- URL 格式：记录警告，降级为文本处理
- 建议只使用 Base64 格式

## 处理逻辑说明

### 数组类型处理

1. **数组识别**：
   - 检查字段是否为数组类型
   - 检查字段名是否为 `image` 或以 `image` 开头
   - 只处理在 `fieldMapping` 中指定的字段

2. **数组元素处理**：
   - 遍历数组中的每个元素
   - 检查元素是否为字符串类型
   - 对每个元素应用以下处理：
     - Base64 编码的图片：解码并转换为文件流
     - URL 格式的图片：记录警告，降级为文本处理（避免死锁）
     - 其他格式：作为普通文本处理

3. **字段名生成**：
   - 数组元素字段名格式：`字段名[索引]`
   - 例如：`image[0]`, `image[1]`, `image[2]`

### 错误处理

- **Base64 解码失败**：记录错误日志，将原始值作为文本处理
- **URL 格式图片**：记录警告日志，将 URL 作为文本处理（避免死锁）
- **数组元素类型错误**：跳过非字符串元素，记录警告日志
- **空数组**：返回错误，记录日志

### 文件名生成

**数组元素图片**：
- 字段名（包含索引） + 图片扩展名
- 例如：`image[0].png`, `image[1].jpg`

**扩展名提取**：
- Base64：从 data URI 中提取（如 `data:image/png` → `.png`）
- 默认：`.png`

## 限制说明

### 功能限制

1. **图片格式**：只支持数组格式
2. **URL 下载**：在 WASM 环境中不支持，会导致死锁
3. **处理方式**：URL 格式会降级为文本处理
4. **推荐格式**：必须使用 Base64 格式

### 配置建议

1. **图片格式**：确保 image 字段是数组格式
2. **图片格式**：必须使用 Base64 格式
3. **图片数量**：建议数组中图片数量不超过 10 张
4. **客户端预处理**：在客户端将 URL 图片转换为 Base64 格式

## 日志输出

### 成功处理
```
[ai-adapter] processed base64 image for field image[0], size: 1024 bytes
[ai-adapter] processed image array for field image, count: 2
```

### URL 格式警告
```
[ai-adapter] URL image download is not supported in WASM environment to avoid deadlock. URL: https://example.com/image.png. Please use Base64 format instead.
```

### 错误处理
```
[ai-adapter] failed to decode base64 image for field image[0]: illegal base64 data at input byte 0
```

## 最佳实践

1. **使用 Base64 格式**：必须使用 Base64 格式，避免死锁
2. **客户端预处理**：在客户端将 URL 图片转换为 Base64 格式
3. **使用数组格式**：确保 image 字段是数组格式
4. **控制图片大小**：建议对图片进行压缩后再编码
5. **监控日志**：定期检查处理日志，及时发现 URL 警告

## 客户端预处理示例

### JavaScript 示例
```javascript
// 将 URL 图片转换为 Base64
async function convertImageToBase64(imageUrl) {
  try {
    const response = await fetch(imageUrl);
    const blob = await response.blob();

    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onloadend = () => resolve(reader.result);
      reader.onerror = reject;
      reader.readAsDataURL(blob);
    });
  } catch (error) {
    console.error('Failed to convert image to Base64:', error);
    return null;
  }
}

// 批量转换
async function convertImagesToBase64(imageUrls) {
  const base64Images = await Promise.all(
    imageUrls.map(url => convertImageToBase64(url))
  );
  return base64Images.filter(img => img !== null);
}

// 使用示例
const imageUrls = [
  'https://cdn.wujiebantu.com/upload/.../image1.png',
  'https://cdn.wujiebantu.com/upload/.../image2.jpg'
];

const base64Images = await convertImagesToBase64(imageUrls);
// 然后将 base64Images 发送到网关
```

### Python 示例
```python
import base64
import requests
from io import BytesIO
from PIL import Image

def convert_url_to_base64(image_url):
    """将 URL 图片转换为 Base64 格式"""
    try:
        response = requests.get(image_url)
        response.raise_for_status()

        # 获取 Content-Type
        content_type = response.headers.get('Content-Type', 'image/jpeg')

        # 转换为 Base64
        base64_data = base64.b64encode(response.content).decode('utf-8')

        # 添加 data URI 前缀
        return f"data:{content_type};base64,{base64_data}"
    except Exception as e:
        print(f"Failed to convert image: {e}")
        return None

# 批量转换
def convert_images_to_base64(image_urls):
    """批量转换图片 URL 为 Base64"""
    base64_images = []
    for url in image_urls:
        base64_image = convert_url_to_base64(url)
        if base64_image:
            base64_images.append(base64_image)
    return base64_images

# 使用示例
image_urls = [
    'https://cdn.wujiebantu.com/upload/.../image1.png',
    'https://cdn.wujiebantu.com/upload/.../image2.jpg'
]

base64_images = convert_images_to_base64(image_urls)
# 然后将 base64_images 发送到网关
```

## 性能考虑

1. **Base64 处理**：Base64 解码性能良好，无需网络请求
2. **内存占用**：图片数据会暂存在内存中
3. **处理时间**：Base64 解码速度快，不影响整体性能
4. **WASM 限制**：URL 下载在 WASM 环境中不可用，避免死锁

## 安全考虑

1. **Base64 验证**：验证 Base64 数据的有效性
2. **大小限制**：通过 multipart 配置限制总请求大小
3. **错误隔离**：单个图片处理失败不会影响其他图片
4. **日志记录**：记录所有处理过程，便于审计