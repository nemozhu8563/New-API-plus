---
date: 2026-05-27
topic: kgapi-async-image-task-integration
---

# KGAPI Async Image Task Integration Requirements

## Problem Frame

KGAPI 的 `gpt-image-2` 图片生成接口在 `/v1/images/generations` 上返回 HTTP `202`，表示图片生成任务已进入异步队列。当前图片 relay 链路只把 HTTP `200` 视为成功，导致合法的异步任务回执被当成错误处理，日志中只留下 `status_code=202`，下游也无法继续查询图片结果。

本需求的目标是定义 `new-api` 对这类异步图片生成上游的接入标准，让下游可以只通过 `new-api` 完成提交、查询、计费追踪和结果读取，不需要直接持有或理解上游 KGAPI 的 key 与任务上下文。

## Observed Upstream Behavior

KGAPI 图片生成提交：

- Request path: `POST /v1/images/generations`
- Example model: `gpt-image-2`
- Success acknowledgement status: HTTP `202`
- Response object: image task acknowledgement
- Polling path observed: `GET /v1/images/tasks/{task_id}`
- Final result shape: `data[]` with `url`; `url` may be a `data:image/png;base64,...` URI, not necessarily an HTTP URL.

Representative HTTP `202` body:

```json
{
  "task_id": "276",
  "id": 276,
  "object": "image.task",
  "status": "queued",
  "endpoint": "/v1/images/generations",
  "model": "gpt-image-2",
  "created_at": 1779862438,
  "updated_at": 1779862438
}
```

## Goal

支持异步图片生成上游，使 `new-api` 能把 HTTP `202` 图片任务回执识别为成功提交，并向下游提供稳定的任务查询契约，直到任务成功、失败或超时。

## Non-Goals

- N1. 本需求不要求把所有图片模型都改成异步模式；同步返回 HTTP `200` 的图片上游必须继续按原路径工作。
- N2. 本需求不要求下游直接调用 KGAPI 查询任务。
- N3. 本需求不要求用状态码映射、重试或外部网关规则绕过 `202`。
- N4. 本需求不覆盖图片编辑、图片变体等其他端点，除非后续明确纳入同一异步任务契约。
- N5. 本需求不要求在第一版支持所有第三方厂商的任意任务格式；先覆盖 OpenAI-compatible image task acknowledgement 与 KGAPI 已验证行为。
- N6. 本需求不要求用长时间阻塞请求强行把所有异步图片任务变成同步接口；这会重新引入 CDN、反向代理和客户端超时风险。

## Recommended Direction

采用“`new-api` 托管任务上下文”的方向：

- 上游返回 HTTP `202` 时，`new-api` 将其识别为异步图片任务提交成功；
- `new-api` 生成并返回自己的公开任务 id；
- 上游任务 id、渠道、key 上下文、用户、token、模型、计费上下文由 `new-api` 保存；
- 下游后续只查询 `new-api` 的图片任务接口；
- `new-api` 使用原始渠道上下文去查询 KGAPI 的 `/v1/images/tasks/{upstream_task_id}`，再把状态和结果规范化返回给下游。

不推荐把 KGAPI 的上游 `task_id` 原样暴露为唯一查询依据，因为多渠道、多 key、重试和权限场景下，仅靠上游 id 无法稳定恢复查询上下文，也不利于日志与计费闭环。

## Requirements

### A. Submission Behavior

- R1. 当图片生成上游返回 HTTP `202` 且响应体是合法图片任务回执时，系统必须把它视为“任务提交成功”，不得进入普通错误处理链路。
- R2. 系统必须识别至少以下任务回执字段：`task_id` 或 `id`、`object`、`status`、`model`、`created_at`、`updated_at`。
- R3. 系统必须向下游返回稳定的任务对象，包含 `id`、`object`、`status`、`model`、`created_at` 等必要字段。
- R4. 下游可见的 `id` 必须是 `new-api` 可追踪的公开任务 id；上游 `task_id` 可以作为内部字段保存，不应作为下游唯一依赖。
- R5. 同步图片上游返回 HTTP `200` 且包含最终 `data[]` 时，现有响应行为必须保持不变。
- R6. HTTP `202` 不得触发请求重试；重试会创建重复的上游生成任务并可能造成重复成本。

### B. Task Persistence and Ownership

- R7. 系统必须保存足够的任务上下文，以便后续查询能够恢复原始上游调用环境。
- R8. 任务上下文至少应包含：公开任务 id、上游任务 id、渠道 id、模型、用户 id、token id、原始请求路径、创建时间、更新时间、当前状态、计费追踪信息。
- R9. 任务查询必须做权限校验；用户只能查询自己有权访问的任务。
- R10. 管理员日志必须能从一次异步图片任务追踪到提交渠道、上游任务 id、公开任务 id 和最终状态。

### C. Polling Behavior

- R11. 系统必须提供下游图片任务查询接口，推荐契约为 `GET /v1/images/tasks/{task_id}`。
- R12. 查询接口必须使用保存的渠道上下文调用上游任务接口；KGAPI 的已验证查询路径为 `GET /v1/images/tasks/{upstream_task_id}`。
- R13. 系统必须规范化任务状态，至少覆盖 `queued`、`generating`、`succeeded`、`failed`、`cancelled` 或等价终态。
- R14. 非终态任务必须返回当前状态和更新时间，不得伪造成成功图片响应。
- R15. 成功任务必须返回最终图片数据，保持 OpenAI-compatible 的 `data[]` 结构。
- R16. 返回结果中的 `data[].url` 可能是 `data:image/png;base64,...`，系统和下游均不得假设它一定是 HTTP URL。
- R17. 上游任务不存在、任务不属于当前用户、或保存的渠道上下文不可用时，系统必须返回明确错误，不得重新提交生成请求。

### D. Result Storage

- R18. 系统不得把大体积图片 base64 或完整 `data:image/...;base64,...` 长期存入任务表的 JSON 字段。
- R19. 任务表只应保存小体积结构化元数据，例如公开任务 id、上游任务 id、状态、模型、尺寸、mime type、结果对象引用、缩略信息和计费信息。
- R20. 当上游只返回 data URI 或 base64 图片时，系统应先把图片内容解码并写入对象存储或等价文件存储，再把可访问 URL 或存储 key 写回任务记录。
- R21. 如果部署环境没有可用对象存储，系统必须显式降级：限制可保存的图片大小、缩短结果保留时间，或返回明确配置错误；不得静默把大量图片写入数据库。
- R22. 最终返回给下游的 `data[].url` 应优先是 `new-api` 管理的可访问 URL；仅在明确允许的兼容模式下才返回 data URI。
- R23. 结果存储必须有保留期和清理策略，避免历史图片无限增长。

### E. Billing and Quota

- R24. HTTP `202` 任务提交成功时，不应按普通 relay 错误自动退款。
- R25. 系统必须定义异步图片任务的结算时点：任务成功后结算，任务失败或超时后退款，或沿用现有异步任务框架的等价机制。
- R26. 计费日志必须能表达“已提交异步任务”“任务成功结算”“任务失败退款”这三个阶段。
- R27. 同一个公开任务 id 的轮询不得重复扣费。
- R28. 如果任务长期没有终态，系统必须有超时或过期策略，避免预扣额度永久悬挂。

### F. Error Handling and Logging

- R29. HTTP `202` 图片任务回执不得记录为 `status_code=202` 错误日志。
- R30. 对合法异步回执的日志应记录为任务提交事件，并包含可排查的 request id、channel id、model、public task id、upstream task id。
- R31. 对真正的上游错误响应，仍必须走现有错误处理链路，并保留上游错误信息。
- R32. 如果 HTTP `202` 响应体不是可识别的任务回执，系统必须返回明确的适配错误，提示该上游返回了未知异步格式。

### G. Admin and User Log Placement

- R33. 异步图片任务的主记录应归入任务记录体系，而不是只写普通用量日志。
- R34. 现有“任务日志”应能按图片任务筛选，例如 `platform=image` 或等价平台标识。
- R35. 异步图片任务不进入“绘图日志”板块；用户和管理员统一在“任务日志”中查看。
- R36. 普通用量日志应记录计费事件和错误事件，并通过 `request_id`、公开 task id 或上游 task id 与任务记录关联。
- R37. 任务记录应展示任务生命周期字段：提交中、生成中、成功、失败、超时、退款状态、结果链接或结果预览入口。
- R38. 管理员视图应额外展示 channel id、channel name、upstream task id、request id、token、user、group、quota 和失败原因。
- R39. 用户视图不得展示上游 key、完整上游私有上下文或其他用户信息。
- R40. 不应把异步图片任务拆成“绘图日志一条 + 任务日志一条”两份业务记录；任务日志是唯一任务生命周期入口。

### H. Configuration and Operations

- R41. 运营配置中不得通过状态码映射把 `202` 改成 `200` 来适配异步任务；这会把错误链路伪装成成功链路，不能生成可查询任务。
- R42. 运营配置中不得为 HTTP `202` 配置自动重试；这会重复提交上游任务。
- R43. 在异步图片任务正式支持前，运营方可以临时禁用 KGAPI `gpt-image-2` 渠道，或把该模型路由到同步返回的上游。
- R44. 渠道配置应能表达某个图片模型或渠道使用异步任务模式，避免把所有图片请求都强行套入异步流程。

### I. Downstream Contract

- R45. 下游调用图片生成接口时，必须能处理两种成功形态：
  - HTTP `200`：同步完成，直接读取 `data[]`；
  - HTTP `202`：异步任务已提交，读取任务 `id` 并进入轮询。
- R46. 下游轮询 `GET /v1/images/tasks/{task_id}`，直到任务进入 `succeeded`、`failed`、`cancelled` 或超时终态。
- R47. 下游必须支持最终 `data[].url` 为 data URI 的兼容情况，但不应要求 `new-api` 永久保存 data URI。
- R48. 下游不得使用 KGAPI 上游 key 直接查询任务，也不得依赖 KGAPI 上游任务 id 作为跨系统公开 id。
- R49. 下游应把非终态展示为“生成中”或等价状态，而不是当成失败。

### J. Optional Blocking Compatibility Mode

- R50. 系统可以提供可选的短等待模式，用于把“很快完成”的异步图片任务转换成同步 HTTP `200` 响应。
- R51. 短等待模式必须是显式 opt-in，推荐通过请求参数或请求头表达，例如 `wait=true`、`timeout=30` 或 `Prefer: wait=30`。
- R52. 短等待模式必须有服务端最大等待上限，不得允许下游无限阻塞 HTTP 请求。
- R53. 如果任务在等待窗口内成功，系统可以返回 HTTP `200` 和最终 `data[]`。
- R54. 如果任务在等待窗口内未完成，系统必须返回 HTTP `202` 和公开任务对象，让下游继续轮询。
- R55. 短等待模式不得跳过任务持久化；即使最后同步返回成功，也应保留任务追踪和计费日志。
- R56. 短等待模式不得作为 Cloudflare 或其他代理超时问题的主要解法；它只适合任务耗时稳定短于代理、服务端和客户端超时预算的场景。
- R57. 短等待模式必须限制并发和资源占用，避免大量阻塞请求耗尽 worker、连接池或 goroutine 资源。

## Key Flows

### F1. Submit Async Image Task

1. 下游请求 `POST /v1/images/generations`。
2. `new-api` 选择 KGAPI 图片渠道并转发请求。
3. KGAPI 返回 HTTP `202` 和图片任务回执。
4. `new-api` 保存任务上下文，生成公开任务 id。
5. `new-api` 向下游返回 HTTP `202` 和公开任务对象。

### F2. Poll Until Success

1. 下游请求 `GET /v1/images/tasks/{public_task_id}`。
2. `new-api` 校验任务归属和可访问性。
3. `new-api` 使用保存的渠道上下文请求 KGAPI `GET /v1/images/tasks/{upstream_task_id}`。
4. KGAPI 返回当前状态。
5. `new-api` 更新本地任务状态，并把规范化结果返回下游。
6. 如果任务成功，下游读取 `data[]` 中的图片结果。

### F2b. Optional Blocking Wait

1. 下游请求 `POST /v1/images/generations`，并显式声明短等待模式。
2. `new-api` 提交上游异步任务并保存任务上下文。
3. `new-api` 在服务端等待窗口内轮询上游任务。
4. 如果任务及时成功，`new-api` 返回 HTTP `200` 和最终 `data[]`。
5. 如果任务未及时完成，`new-api` 返回 HTTP `202` 和公开任务 id。
6. 下游继续按标准异步任务查询接口轮询。

### F3. Failure, Timeout, and Refund

1. 查询发现上游任务进入失败终态，或超过系统定义的任务过期时间。
2. `new-api` 标记公开任务为失败或超时。
3. 系统按异步任务计费规则退款或释放预扣额度。
4. 后续查询返回稳定失败结果，不重新提交上游生成。

## Acceptance Examples

- AE1. KGAPI 返回 HTTP `202` 且 body 包含 `task_id` 时，请求日志不再出现 `status_code=202` 错误；下游收到可轮询的图片任务对象。
- AE2. 下游使用公开任务 id 查询 `GET /v1/images/tasks/{task_id}`，任务从 `queued` 或 `generating` 更新到 `succeeded` 后，响应包含最终 `data[]`。
- AE3. 最终 `data[].url` 为 `data:image/png;base64,...` 时，下游仍能正常展示或保存图片。
- AE4. 同步图片上游返回 HTTP `200` 时，现有 `data[]` 响应不受异步适配影响。
- AE5. 对 HTTP `202` 配置状态码映射或自动重试不被视为合格方案。
- AE6. 用户 A 不能查询用户 B 的图片任务。
- AE7. 上游任务失败或超时时，系统产生明确失败日志并执行对应退款或释放额度逻辑。
- AE8. 下游显式启用短等待模式，且任务在等待窗口内完成时，接口返回 HTTP `200` 和 `data[]`。
- AE9. 下游显式启用短等待模式，但任务未在等待窗口内完成时，接口返回 HTTP `202` 和可继续轮询的公开任务 id。
- AE10. KGAPI 返回 data URI 图片结果时，任务表只保存小体积元数据和结果引用，不保存完整 base64 图片内容。
- AE11. 对象存储未配置且上游只返回 base64 图片时，系统返回明确配置错误或执行受限降级策略，不静默膨胀数据库。
- AE12. 异步图片任务在任务日志中可按图片平台筛选，且不出现在绘图日志中。
- AE13. 管理员可从任务记录追踪到 request id、channel id、upstream task id 和计费事件；用户只能看到自己的任务状态和结果。

## Success Criteria

- KGAPI `gpt-image-2` 的 HTTP `202` 回执被识别为异步任务提交成功，而不是 relay 错误。
- 下游可以只通过 `new-api` 完成提交、轮询、读取结果。
- 轮询不依赖下游保存 KGAPI key 或直接访问 KGAPI。
- 同步图片生成链路保持兼容。
- 异步图片任务不会因重试产生重复上游任务。
- 计费、退款、日志能按公开任务 id 串起来。
- 图片二进制内容不长期存放在数据库任务表中。
- 管理员和用户都能在任务日志中找到异步图片任务，但系统不重复写两套业务记录。
- 可选短等待模式不会造成长时间 HTTP 阻塞，也不会破坏标准异步轮询契约。

## Scope Boundaries

- 本需求覆盖图片生成的异步任务提交与查询契约。
- 本需求优先覆盖 KGAPI `gpt-image-2` 的已验证行为。
- 本需求可以复用现有任务框架，但不在需求阶段规定具体代码组织方式。
- 本需求不要求一次性统一视频、音乐、MJ 和图片任务的全部内部实现，只要求下游契约保持稳定。
- 本需求不要求下游理解每个上游厂商的私有任务接口。
- 本需求不把数据库任务表定义为图片对象存储；大体积图片结果应进入对象存储或等价文件存储。

## Key Decisions

- 把 HTTP `202` 图片回执定义为“异步任务提交成功”，不是错误。
- 使用 `new-api` 公开任务 id 作为下游查询依据。
- 上游任务 id 保留为内部追踪字段，避免把渠道上下文泄露给下游。
- 轮询必须经由 `new-api`，这样才能保持权限、渠道、计费和日志闭环。
- 状态码映射和自动重试不是合格适配方式。
- 任务表保存任务状态和小元数据，图片本体不长期写入任务表。
- 异步图片任务以任务日志为唯一生命周期入口，不进入绘图日志。
- 允许可选短等待模式，但默认契约仍以异步任务和轮询为准。

## Dependencies / Assumptions

- KGAPI 当前使用 `POST /v1/images/generations` 提交图片任务。
- KGAPI 当前使用 `GET /v1/images/tasks/{task_id}` 查询图片任务。
- KGAPI 当前可能返回 `queued`、`generating`、`succeeded` 等状态。
- KGAPI 成功结果中的 `data[].url` 可能是 data URI。
- `new-api` 当前已有异步任务相关能力，可作为规划阶段的优先参考。
- 生产级纯异步图片支持需要可用的对象存储或等价文件存储，用于承载上游返回的图片本体。

## Source Notes

- 当前图片 relay 行为参考：`relay/image_handler.go`
- 普通错误处理行为参考：`service/error.go`
- 现有异步任务入口参考：`relay/relay_task.go`
- 现有任务模型参考：`model/task.go`
- 现有视频任务路由参考：`router/video-router.go`
- 本文不记录任何上游密钥；密钥只用于临时验证 KGAPI 行为。
