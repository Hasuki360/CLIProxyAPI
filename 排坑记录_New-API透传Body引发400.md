# 排坑记录：New-API「透传 Body」引发 Unknown name "messages" / "temperature" 400 错误

## 1. 故障现象
在使用沉浸式翻译或其他 OpenAI 格式客户端，通过 New-API 调用 Google Gemini 模型（经由 CLIProxyAPI 或直连官方）时，偶发或必发 HTTP 400 报错：
```json
HTTP 400
{
  "error": {
    "code": 400,
    "message": "Invalid JSON payload received. Unknown name \"temperature\" at 'request': Cannot find field.\nInvalid JSON payload received. Unknown name \"messages\" at 'request': Cannot find field.",
    "status": "INVALID_ARGUMENT",
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.BadRequest",
        "fieldViolations": [
          {
            "field": "request",
            "description": "Invalid JSON payload received. Unknown name \"temperature\" at 'request': Cannot find field."
          },
          {
            "field": "request",
            "description": "Invalid JSON payload received. Unknown name \"messages\" at 'request': Cannot find field."
          }
        ]
      }
    ]
  }
}
路由: server ESF
响应: application/json; charset=UTF-8
```

---

## 2. 根本原因

### 2.1 协议差异
- **OpenAI Chat Completions 协议**：请求体结构为顶层包含 `messages`、`model`、`temperature`、`top_p`、`stream` 等。
- **Google Gemini generateContent 协议**：请求体结构顶层仅允许 `contents`、`systemInstruction`、`generationConfig`、`safetySettings`、`tools`、`toolConfig` 等。参数如 `temperature` 必须放在 `generationConfig.temperature` 下，完全不存在 `messages` 这个顶层键名。

### 2.2 New-API「透传 Body」机制的破坏
New-API 针对类型为 24（Google Gemini）的渠道：
- **默认状态 (`pass_through_body_enabled: false`)**：New-API 接收客户端的 OpenAI 报文后，会将 `messages` 逐条解析转换为 Gemini 的 `contents`，并将 `temperature` 放入 `generationConfig`，然后向下游发起标准的 `:generateContent` 请求。
- **误开透传 (`pass_through_body_enabled: true`)**：New-API 会认为下游端点完全接收并理解客户端的原生请求，直接将客户端包含 `messages`、`temperature` 的 OpenAI JSON 报文**原封不动透传**给 Gemini 端点。
- **Google ESF 拒收**：Google Cloud 的前端代理 ESF（Protobuf 反序列化层）严格强校验，一旦发现 `request` 中有未定义的键名（`messages`、`temperature`），直接阻断并抛出 `Unknown name ... Cannot find field.` 错误。

---

## 3. 处理原则与结论

1. **不在 CPA 层面做过度工程**：
   - 曾测试在 CPA 转译层加入 OpenAI-in-Gemini 的兼容层，但这属于将上游中间件配置失误强加给底层代理反代的过度工程行为，破坏了清晰的职责边界。
   - 代码已按莲纪要求干净撤回，保持 CPA 代码库小巧精简、聚焦本职。
2. **正确配置规范**：
   - 在 New-API 中添加或修改类型为 **Google Gemini (24)** 的渠道时，**切勿勾选「透传 Body」**。
   - 确认设置中的 `pass_through_body_enabled` 为 `false` 即可彻底杜绝该报错。
