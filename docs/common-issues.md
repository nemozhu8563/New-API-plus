# 常见问题处理

这份文档用于记录日常使用和运维中常见的错误信息、判断方法和处理动作。

## `Upstream API error (status=500): field messages is required`

### 这是什么问题

这是请求体缺少 `messages` 字段。

虽然客户端看到的是 `Upstream API error (status=500)`，但这个错误通常不是上游模型挂了，也不是渠道 Key 坏了，而是请求没有按当前接口要求传入对话消息。new-api 在解析 `/v1/chat/completions` 或 Claude `/v1/messages` 请求时会校验 `messages`，缺失或为空就会返回 `field messages is required`。

简单说：请求打到了聊天接口，但发的 body 不是聊天接口格式。

### 常见原因

- 把 `/v1/responses` 的 `input` 格式发到了 `/v1/chat/completions`。
- 客户端只传了 `model`、`prompt` 或 `input`，没有传 `messages`。
- 请求路径用的是 Claude `/v1/messages`，但 body 仍然是 OpenAI Responses 或旧 completions 格式。
- 中间层、SDK 适配层或参数覆盖规则把 `messages` 删除或覆盖成了空数组。
- 客户端构造请求时有空会话、空历史或编辑中的草稿，最后生成了 `messages: []`。

### 错误示例

把 Responses 格式发到 Chat Completions：

```json
{
  "model": "gpt-5.1",
  "input": "你好"
}
```

或者只传旧 completions 风格字段：

```json
{
  "model": "gpt-5.1",
  "prompt": "你好"
}
```

### 正确写法

OpenAI Chat Completions `/v1/chat/completions`：

```json
{
  "model": "gpt-5.1",
  "messages": [
    {
      "role": "user",
      "content": "你好"
    }
  ]
}
```

Claude Messages `/v1/messages`：

```json
{
  "model": "claude-opus-4-7",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "你好"
    }
  ]
}
```

如果调用方想使用 Responses 格式，就不要发到 `/v1/chat/completions`，应该改用 `/v1/responses`：

```json
{
  "model": "gpt-5.1",
  "input": "你好"
}
```

### 先怎么判断

1. 用错误里的 request id 查日志，确认请求路径是 `/v1/chat/completions`、`/v1/messages` 还是 `/v1/responses`。
2. 查看客户端原始 request body，确认是否存在非空 `messages`。
3. 如果客户端原始 body 有 `messages`，继续检查参数覆盖、转换链路或代理层是否把它删掉了。
4. 如果客户端原始 body 只有 `input`，说明它发的是 Responses 格式，需要改接口路径或改 payload。
5. 如果客户端原始 body 只有 `prompt`，说明它发的是旧 completions 格式，需要改成 chat `messages` 或使用对应 completions 接口。

### 处理动作

- Chat Completions 调用方：补齐 `messages`，至少包含一条 `role: "user"` 消息。
- Responses 调用方：改走 `/v1/responses`，不要把 `input` body 发到 `/v1/chat/completions`。
- Claude SDK 调用方：确认 base URL 指向中转站根地址，让 SDK 自己拼 `/v1/messages`，并传 `messages` 和 `max_tokens`。
- 检查渠道参数覆盖规则，不要误删 `messages` 或把它改成空数组。
- 如果这是生产告警，不要优先重启服务；先按 request id 查请求路径和请求体。

### 相关代码检查点

- `relay/helper/valid_request.go` 的 `GetAndValidateClaudeRequest`：Claude `/v1/messages` 要求 `messages` 非空。
- `relay/helper/valid_request.go` 的 `GetAndValidateTextRequest`：`/v1/chat/completions` 要求 `messages` 非空，除非是 FIM 的 `prefix`/`suffix` 请求。
- `controller/relay.go` 会把本地校验错误附加 request id 返回，所以看到 `(request id: ...)` 只能说明错误经过了 new-api，并不代表一定是上游返回的。

## `status_code=400, ...content.0: Invalid signature in thinking block`

### 这是什么问题

这是 Claude/Anthropic 的扩展思考块验签失败。

请求里带了历史 assistant 消息的 `thinking` 块，并且这个块里有 `signature`。这个签名是上游生成的，用来证明这段思考内容没有被改过。后续请求如果把这段 `thinking` 再发回上游，内容和签名必须原样匹配；只要中间被裁剪、拼接、转格式、脱敏、重写字段，都会被上游拒绝并返回 `400`。

简单说：不是额度问题，也通常不是 Key 坏了，而是历史消息里的思考块不可信了。

### 常见原因

- 客户端保存了 Claude 返回的 `thinking`，但没有完整保存对应的 `signature`。
- 流式响应时没有正确收集 `signature_delta`，下一轮请求带回了不完整签名。
- 客户端或中间层改写了 `messages.*.content.*.thinking` 或 `signature`。
- 把 OpenAI 兼容格式里的 `reasoning_content` 伪装成 Claude 的 `type: "thinking"`。
- 参数覆盖、日志脱敏、格式转换、历史消息压缩，把 thinking block 的内容或顺序改了。
- tool use 连续对话中，只回传了工具结果，没有把上一轮 assistant 的 thinking block 按上游原样带回。

### 先怎么判断

1. 按请求 ID 查到实际发往上游的 request body。
2. 找到报错位置，例如 `messages.*.content.0`，看这个块是不是 `type: "thinking"` 或 `type: "redacted_thinking"`。
3. 检查这个块是否同时包含上游返回的原始 `thinking` 和 `signature`。
4. 如果走了 Claude 到 OpenAI、OpenAI 到 Claude 的格式转换，重点检查是否把 `reasoning_content` 转成了 Claude `thinking`。
5. 如果渠道开启了请求体直通，优先检查客户端原始请求；如果没有直通，再检查 new-api 的转换、过滤和参数覆盖链路。

### 处理动作

- 不需要保留历史思考内容时，删除历史消息里的 `thinking` / `redacted_thinking` 块。
- 需要保持 Claude 原生连续上下文时，必须原样保存并原样回传上游返回的 thinking block，包括 `signature`。
- OpenAI 兼容客户端不要自己构造 Claude `thinking` block；`reasoning_content` 不能当成已签名 thinking 回传。
- 排查是否有参数覆盖规则在修改 `messages.*.content.*`，尤其是 trim、replace、prune、delete、set 这类操作。
- 对有工具调用的 Claude 请求，确认上一轮 assistant 消息、thinking block、tool_use 和本轮 tool_result 的历史顺序没有被压缩器改乱。

### 相关代码检查点

- `dto.ClaudeMediaMessage` 里有 `Signature` 字段，Claude 原生请求结构可以承载签名。
- `service.ClaudeToOpenAIRequest` 会把 Claude 消息转成 OpenAI 兼容消息，当前主要处理 text/image/tool_use/tool_result，不会把 Claude thinking block 当作已签名 thinking 原样保留。
- `relay/claude_handler.go` 中如果开启全局或渠道请求体直通，会直接使用客户端原始 body；否则会经过渠道转换、禁用字段过滤和参数覆盖。

## `status_code=400, This model does not support assistant message prefill. The conversation must end with a user message.`

### 这是什么问题

这是上游 Claude/Anthropic 兼容接口拒绝了 assistant prefill 请求。

请求的 `messages` 最后一条是 `role: "assistant"`。这类请求通常表示“我先给 assistant 填一段开头，让模型接着写”。但是部分 Claude 模型不支持 assistant message prefill，要求对话必须以 `user` 消息结尾，所以返回 `400`。

简单说：不是 Key 坏了，也通常不是 new-api 进程故障，而是客户端发来的对话历史形态不被当前模型支持。

### 典型错误请求

```json
{
  "model": "claude-opus-4-7",
  "max_tokens": 100,
  "messages": [
    {
      "role": "user",
      "content": "写一句介绍"
    },
    {
      "role": "assistant",
      "content": "当然，下面是"
    }
  ]
}
```

### 常见原因

- 客户端把上一轮 assistant 草稿、开头或半截回复放到了最后一条消息里。
- OpenAI 兼容客户端使用了 assistant prefill 写法，但实际路由到了不支持 prefill 的 Claude 模型。
- 历史消息压缩、断点续写或失败重试时，只保留了 assistant 前缀，没有保留后续 user 指令。
- 中间层做 OpenAI 到 Claude 格式转换时，原样保留了最后一条 assistant 消息。
- 客户端想实现“从这段话继续写”，但没有把这段话放进 `user` 指令里。

### 先怎么判断

1. 按请求 ID 查到实际发往上游的 request body。
2. 检查 `messages` 最后一项的 `role`。
3. 如果最后一项是 `assistant`，并且报错包含 `does not support assistant message prefill`，基本可以确认是 assistant prefill 不兼容。
4. 如果最后一项 `assistant` 里还有 `tool_use`、`tool_calls` 或工具调用内容，还要继续检查是否缺少对应的 tool result。
5. 如果请求走的是 OpenAI `/v1/chat/completions`，但渠道最终转成 Claude `/v1/messages`，重点检查转换后的 Claude 请求，而不是只看客户端原始 OpenAI 请求。

### 调用方怎么改

把 assistant 前缀改写进最后一条 `user` 指令里：

```json
{
  "model": "claude-opus-4-7",
  "max_tokens": 100,
  "messages": [
    {
      "role": "user",
      "content": "写一句介绍。开头请使用：当然，下面是"
    }
  ]
}
```

如果最后一条 assistant 是空内容，可以直接删除这条空 assistant。

如果最后一条 assistant 是工具调用，不能简单改成 user；需要补齐对应的 tool result，或者让客户端重新构造完整工具调用上下文。

### new-api 无法拿到原本用户消息时怎么兜底

如果只能看到已经转换后的消息，无法还原原始用户内容，不要猜用户原本想问什么。可以按损失最小原则处理：

- 最后一条是空 assistant：删除这条空消息。
- 最后一条是普通文本 assistant：保留这条 assistant，并追加一条合成 user 消息，例如 `Please continue.`，让对话以 user 结尾。
- 最后一条 assistant 含工具调用：不要自动续写，返回更明确的错误，提示调用方补齐 tool result。

这种兜底只能提高兼容性，不能保证语义完全等同于原始 assistant prefill。最根本的修复仍然是调用方不要向不支持 prefill 的模型发送最后一条 assistant。

### 用 Claude SDK 调试

Anthropic 官方 SDK 不需要登录网页账号，本质上只是 HTTP client。

- 直连 Anthropic：使用 `ANTHROPIC_API_KEY`。
- 走中转站：使用中转站的 Key，并把 `base_url` 指向中转站根地址。
- 中转站必须兼容 Anthropic `/v1/messages`；如果只兼容 OpenAI `/v1/chat/completions`，就不能直接用 Anthropic SDK。

Python 最小复现：

```python
import anthropic

client = anthropic.Anthropic(
    api_key="sk-your-key",
    base_url="https://your-new-api-domain",
)

try:
    msg = client.messages.create(
        model="claude-opus-4-7",
        max_tokens=100,
        messages=[
            {"role": "user", "content": "写一句介绍"},
            {"role": "assistant", "content": "当然，下面是"},
        ],
    )
    print(msg.to_json())
except Exception as e:
    print(type(e).__name__)
    print(e)
```

注意：`base_url` 填中转站域名根路径即可，不要手动加 `/v1/messages`，SDK 会自己拼接路径。

### 什么时候需要找上游

如果同一个请求 body 直连 Anthropic 和走中转站都返回相同的 `assistant message prefill` 错误，通常不需要找上游，这是模型能力限制。

如果直连 Anthropic 成功，但走 new-api 或某个中转站失败，需要带上以下信息排查转换链路：

- 模型名。
- 失败时间段。
- new-api 日志里的 request id。
- 客户端原始请求 body。
- 实际发往上游的 request body。
- 上游返回的完整错误信息。

## `status_code=429, Upstream rate limit exceeded, please retry later`

### 这是什么问题

这是上游服务返回的限流错误。

请求已经进入 new-api，并且 new-api 已经把请求转发给对应渠道；但是上游模型服务或上游中转服务认为当前请求量、并发量、Token 速率或账号额度超过限制，于是返回 `429`。

简单说：网关本身不一定坏了，通常是被选中的上游渠道暂时扛不住或额度不够。

### 常见原因

- 单个渠道的上游 RPM、TPM 或并发限制被打满。
- 当前渠道 Key 的额度、套餐或账号池资源不足。
- 多个请求被路由到同一个压力较大的渠道。
- 客户端自动重试或并发较高，把短时间流量放大了。
- 渠道亲和性把同类请求持续固定到某个已限流渠道。

### 先怎么判断

1. 看日志里的 `channel_id`、模型名、请求时间和错误信息，确认是哪条渠道返回 `429`。
2. 如果只有某一条渠道集中报错，优先按渠道问题处理。
3. 如果多个渠道同时报错，检查上游供应商整体状态、账号池额度和当前并发。
4. 如果应用健康检查正常，但业务请求持续失败，按上游渠道退化处理，不要只看 `/api/status`。

### 处理动作

- 等待一段时间后重试，观察是否只是短时峰值。
- 临时降低客户端并发和重试频率。
- 切换到其他可用渠道或增加同模型可用渠道。
- 降低限流渠道的权重，避免继续承接大部分流量。
- 检查该渠道上游 Key 的 RPM、TPM、并发和余额/套餐限制。
- 如果生产环境希望自动摘除限流渠道，确认：
  - `AutomaticDisableChannelEnabled=true`
  - 渠道本身允许自动禁用，例如 `auto_ban=1`
  - `AutomaticDisableStatusCodes` 包含 `429`
- 如果使用渠道亲和性，确认失败后允许重试或失败转移，避免一直固定到同一个已限流渠道。
- 保持至少一个重试次数，例如 `RetryTimes=1`，让一次坏渠道选择有机会切到其他渠道。

### 有用的配置检查

```sql
select key, value
from options
where key in (
  'AutomaticDisableChannelEnabled',
  'AutomaticDisableStatusCodes',
  'AutomaticDisableKeywords',
  'AutomaticEnableChannelEnabled',
  'RetryTimes',
  'channel_affinity_setting.rules',
  'channel_affinity_setting.switch_on_success'
)
order by key;
```

### 什么时候需要找上游

如果同一个渠道长时间持续返回 `429`，并且降低并发、切换权重后仍然复现，需要带上以下信息找上游排查：

- 渠道 Key 或账号标识。
- 模型名。
- 失败时间段。
- 上游返回的完整错误信息。
- 如果响应头里有 `request-id`、`x-request-id`、`cf-ray` 等追踪 ID，也一起提供。

### 相关记录

- `docs/solutions/integration-issues/codex-upstream-channel-auto-disable-and-affinity-failover-2026-05-19.md`

## `status_code=400, Identity verification is required to continue`

### 这是什么问题

这是上游渠道返回的身份验证拦截。

请求已经进入 new-api，并且 new-api 已经把请求转发给对应渠道；但是上游账号、上游 Key 或上游中转账号被要求先完成身份验证，所以返回 `400`。

简单说：这通常不是 new-api 本身挂了，也不是普通请求参数写错了，而是被选中的上游账号暂时不能继续调用。

### 常见原因

- 上游账号新注册、风控触发，要求补充身份验证。
- 上游账号所在平台升级了验证要求。
- 上游 Key 绑定的账号未完成实名认证、付款验证或组织验证。
- 第三方中转渠道背后的账号池里有账号进入验证状态。
- 某些模型或接口比普通接口要求更严格，命中了需要验证的账号。

### 先怎么判断

1. 用日志里的 `request id` 查到对应请求，确认 `channel_id`、模型名和失败时间。
2. 看是否只有某一条渠道返回这类错误。
3. 如果只有单条渠道报错，优先按上游账号/渠道问题处理。
4. 如果多条渠道同时报错，检查它们是否来自同一个上游平台或同一个中转服务。
5. 如果 `/api/status` 正常，但业务请求持续失败，说明网关进程大概率正常，问题在具体上游渠道。

### 处理动作

- 登录对应上游平台后台，按提示完成身份验证。
- 如果是第三方中转渠道，把错误信息和 `request id` 发给中转服务商，让对方查对应账号池状态。
- 验证完成前，临时禁用这条渠道，或降低权重，避免继续命中。
- 如果同模型还有其他可用渠道，先切到已验证、可正常调用的渠道。
- 如果该渠道只对部分模型报错，可以先从渠道模型列表里移除受影响模型。
- 保留完整错误信息，不要只截取 `400`，因为真正原因在后面的 `Identity verification is required to continue`。

### 什么时候需要找上游

只要确认同一渠道稳定返回这个错误，就需要找上游或中转服务商处理。提交时带上：

- 渠道 Key 或账号标识。
- 模型名。
- 失败时间段。
- new-api 日志里的 `request id`。
- 上游返回的完整错误信息。
