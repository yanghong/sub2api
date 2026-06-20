# 生图接口对接文档

本文档面向调用 Sub2API 网关的业务方，描述 OpenAI 平台分组下的图片生成与图片编辑接口。

## 1. 接入前提

- API Key 必须属于 `openai` 平台分组。
- 分组需要开启 `允许当前分组生图`（字段：`allow_image_generation=true`）。
- 非 OpenAI 分组调用图片接口会返回 `404 Images API is not supported for this platform`。
- 未开启生图权限的分组会返回 `403 Image generation is not enabled for this group`。
- 请求体大小受网关 `Gateway.MaxBodySize` 配置限制。

## 2. 鉴权

所有请求都使用 Bearer Token：

```http
Authorization: Bearer sk-xxxx
Content-Type: application/json
```

`BASE_URL` 使用你的 Sub2API 服务地址，例如：

```text
https://api.example.com
```

## 3. 生成图片

### 3.1 Endpoint

```http
POST {BASE_URL}/v1/images/generations
```

兼容别名：

```http
POST {BASE_URL}/images/generations
```

### 3.2 请求参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `prompt` | string | 是 | 生图提示词 |
| `model` | string | 否 | 图片模型，默认 `gpt-image-2`；必须是 `gpt-image-*` |
| `size` | string | 否 | 图片尺寸，如 `1024x1024`、`1024x1536`、`1536x1024`、`2048x2048`、`2048x1152`、`3840x2160`、`auto` |
| `n` | number | 否 | 生成张数，默认 `1`，必须大于 `0` |
| `response_format` | string | 否 | `b64_json` 或 `url`；`url` 在网关 OAuth 转发场景会返回 `data:image/...;base64,...` |
| `stream` | boolean | 否 | 是否流式返回 |
| `quality` | string | 否 | 渲染质量，如 `low`、`medium`、`high`、`auto`，具体取值取决于上游模型 |
| `background` | string | 否 | 背景设置，如 `transparent`、`opaque`、`auto`，具体取值取决于上游模型 |
| `output_format` | string | 否 | 输出格式：`png`、`jpeg`、`webp` |
| `output_compression` | number | 否 | JPEG/WebP 压缩参数，通常为 `0-100` |
| `moderation` | string | 否 | 上游支持的图片审核策略参数 |
| `style` | string | 否 | 上游支持的风格参数 |
| `partial_images` | number | 否 | 流式模式下的中间图数量，通常为 `1-3` |

### 3.3 cURL 示例

```bash
curl -X POST "$BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "生成一张写实风格的产品海报，主体是一杯冰美式，背景为明亮咖啡店",
    "size": "1024x1024",
    "quality": "medium",
    "output_format": "png",
    "response_format": "b64_json"
  }'
```

### 3.4 非流式返回

```json
{
  "created": 1710000000,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAA...",
      "revised_prompt": "..."
    }
  ],
  "model": "gpt-image-2",
  "size": "1024x1024",
  "quality": "medium",
  "output_format": "png",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 200,
    "total_tokens": 210
  }
}
```

当 `response_format=url` 时，返回项可能是：

```json
{
  "url": "data:image/png;base64,iVBORw0KGgoAAA..."
}
```

## 4. 编辑图片

### 4.1 Endpoint

```http
POST {BASE_URL}/v1/images/edits
```

兼容别名：

```http
POST {BASE_URL}/images/edits
```

### 4.2 JSON 方式

JSON 方式用于传入公网可访问图片或 base64 data URL。

```bash
curl -X POST "$BASE_URL/v1/images/edits" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "把图片背景替换成极光夜空，保持人物主体不变",
    "images": [
      {
        "image_url": "https://example.com/input.png"
      }
    ],
    "size": "1024x1024",
    "response_format": "b64_json"
  }'
```

说明：

- `images` 必须是数组，且至少包含一个 `image_url`。
- `images[].file_id` 暂不支持。
- 遮罩图使用 `mask.image_url`：

```json
{
  "mask": {
    "image_url": "https://example.com/mask.png"
  }
}
```

### 4.3 multipart/form-data 方式

multipart 方式用于直接上传本地文件。

```bash
curl -X POST "$BASE_URL/v1/images/edits" \
  -H "Authorization: Bearer $API_KEY" \
  -F "model=gpt-image-2" \
  -F "prompt=把背景换成干净的白色摄影棚，保留商品主体" \
  -F "image=@./input.png" \
  -F "mask=@./mask.png" \
  -F "size=1024x1024" \
  -F "response_format=b64_json"
```

说明：

- 图片字段名支持 `image` 或 `image[index]`。
- 遮罩字段名为 `mask`。
- 单个上传图片 part 最大约 `20MB`。

## 5. 流式返回

请求中设置：

```json
{
  "stream": true,
  "partial_images": 2
}
```

返回为 SSE：

```text
event: image_generation.partial_image
data: {"type":"image_generation.partial_image","created_at":1710000000,"partial_image_index":0,"b64_json":"..."}

event: image_generation.completed
data: {"type":"image_generation.completed","created_at":1710000001,"b64_json":"...","usage":{"input_tokens":10,"output_tokens":200,"total_tokens":210}}
```

编辑接口的事件名前缀为 `image_edit`：

```text
event: image_edit.partial_image
event: image_edit.completed
```

## 6. 通过 Responses API 生图

如果业务需要在对话、多轮编辑或工具调用流程里生图，可以调用 Responses API：

```http
POST {BASE_URL}/v1/responses
```

示例：

```bash
curl -X POST "$BASE_URL/v1/responses" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "input": "生成一张 16:9 的 SaaS 仪表盘宣传图，风格专业、真实、不夸张",
    "tools": [
      {
        "type": "image_generation",
        "model": "gpt-image-2",
        "size": "2048x1152",
        "quality": "medium",
        "output_format": "png"
      }
    ],
    "tool_choice": {
      "type": "image_generation"
    }
  }'
```

注意：

- `model` 使用支持 Responses 的文本模型。
- 图片模型放在 `tools[].model`。
- 直接把 `model` 写成 `gpt-image-*` 时，网关会尝试按图片生图意图处理，但推荐使用上面的显式 `tools` 写法。

## 7. 计费口径

网关会记录图片张数和图片尺寸层级：

| 层级 | 典型尺寸 |
| --- | --- |
| `1K` | 最大边不超过 `1024` |
| `2K` | 最大边不超过 `2048`，如 `2048x2048`、`2048x1152` |
| `4K` | 最大边超过 `2048`，如 `3840x2160`、`2160x3840` |

分组计费字段：

- `image_price_1k`
- `image_price_2k`
- `image_price_4k`
- `image_rate_independent`
- `image_rate_multiplier`

当 `image_rate_independent=false` 时，图片费用使用当前分组有效倍率；当为 `true` 时，使用 `image_rate_multiplier`。

## 8. 常见错误

| HTTP 状态码 | error.type | 说明 |
| --- | --- | --- |
| `400` | `invalid_request_error` | 请求体为空、JSON 无法解析、参数类型错误、`prompt` 或图片输入缺失 |
| `401` | `authentication_error` | API Key 无效或缺失 |
| `403` | `permission_error` | 当前分组未开启生图 |
| `404` | `not_found_error` | 当前分组不是 OpenAI 平台，或接口路径不支持 |
| `413` | `invalid_request_error` | 请求体超过网关大小限制 |
| `503` | `api_error` | 没有可用的兼容上游账号 |

错误返回示例：

```json
{
  "error": {
    "type": "permission_error",
    "message": "Image generation is not enabled for this group"
  }
}
```

## 9. 最小前端处理示例

```ts
const res = await fetch(`${BASE_URL}/v1/images/generations`, {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${API_KEY}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    model: 'gpt-image-2',
    prompt: '生成一张科技产品发布会主视觉',
    size: '1024x1024',
    response_format: 'b64_json',
  }),
})

if (!res.ok) {
  const error = await res.json().catch(() => null)
  throw new Error(error?.error?.message ?? `request failed: ${res.status}`)
}

const json = await res.json()
const image = json.data?.[0]
const src = image?.url ?? `data:image/png;base64,${image?.b64_json}`
```

## 10. 参考

- OpenAI Image API 支持生成与编辑图片，并提供 Generations 和 Edits 两类端点。
- OpenAI Responses API 可通过 `image_generation` 内置工具在对话或多步流程中生成图片。
- Sub2API 当前实现会在 OpenAI 分组下转发上述能力，并在网关侧做权限、调度、并发、审核与用量记录。
