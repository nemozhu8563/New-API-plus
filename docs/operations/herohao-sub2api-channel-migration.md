# HeroHao Sub2API 渠道上线 SOP

本文用于把 `https://sub2.herohao.top` 接入 new-api 的测试或正式环境，并在迁移后核对模型、Responses 接口和计费。文档不包含任何上游或下游密钥；密钥只能通过部署环境的密钥管理或管理后台填写。

这份 SOP 对应当前已发布的 HeroHao 适配：渠道转发继续使用现有 Sub2API，价格同步由模型定价页的“上游价格同步”调用 HeroHao 专用价格接口。上线时不需要单独运行价格导入脚本，但每次价格变更仍必须由管理员在页面审核并点击应用。

## 结论

HeroHao 当前返回标准 OpenAI 兼容协议，可以直接使用现有 **Sub2API（渠道类型 59）**，不需要 Advanced Custom。当前代码已经包含 HeroHao 价格适配器，已验证的上游接口包括：

- `GET /v1/models`：标准模型列表，返回 21 个模型；
- `POST /v1/chat/completions`：返回标准 Chat Completions 和 `usage`；
- `POST /v1/responses`：返回 `object: "response"`、`status: "completed"`、`output` 和 `usage.input_tokens/output_tokens/total_tokens`。

模型列表和价格是两条独立链路：渠道的“从上游获取”读取 `/v1/models`；模型定价页的“上游价格同步”读取 `/pricing/api/pricing`，只取 `official` 字段，生成 `tiered_expr` 表达式，管理员确认后才写入本地模型价格。同步请求不会把 `selling`、`reference` 或其他上游售价覆盖到本地价格。

## 架构图

```mermaid
flowchart LR
    A[管理员保存渠道] --> B[Sub2API 渠道类型 59]
    B --> C[GET /v1/models]
    C --> D[渠道 models / abilities]
    D --> E[统一路由与 /v1/models]
    E --> F[下游 Chat Completions]
    E --> G[下游 Responses]
    F --> H[HeroHao /v1/chat/completions]
    G --> I[HeroHao /v1/responses]
    H --> J[usage]
    I --> J
    J --> K[预扣费 / 结算 / 使用日志]
    P[GET /pricing/api/pricing] --> Q[HeroHao official 适配器]
    Q --> R[管理员审核差异]
    R --> S[new-api 模型价格配置]
    S --> K
```

## 正式上线前置条件

- 目标环境已运行包含 HeroHao 适配器的版本；可在 `/api/status` 查看服务健康状态，并在发布记录中核对镜像对应的 Git SHA。
- 已准备 HeroHao 上游 Key 和正式环境管理员账号。Key 只在渠道管理页填写，不写入仓库、脚本、工单或日志。
- 已从上游价格页保存一份价格快照，作为本次审核和回滚依据。
- 已确认本地要公开的模型名称与 HeroHao `/v1/models` 返回的 `id` 完全一致；模型别名必须通过渠道模型映射维护。

当前适配器随 new-api 构建版本发布；它不是独立的常驻同步进程，也不是上游定时任务。正式环境发布后，管理员需要在页面手动执行一次价格同步。若目标环境仍是未包含该适配器的旧镜像，先完成应用发布，再执行本 SOP 的价格步骤。

## 渠道配置

在测试或正式环境的“渠道”页面新建渠道：

| 字段 | 值 |
| --- | --- |
| 类型 | `Sub2API`（类型编号 59） |
| 名称 | 例如 `HeroHao-国模-测试` |
| API 地址 | `https://sub2.herohao.top` |
| API 密钥 | 上游提供的 Key（不要提交到 Git） |
| 分组 | `default`，或按现有路由约定填写 |
| 模型 | 点击“从上游获取”，确认加载 21 个模型 |

API 地址不要带 `/v1`。系统会根据请求格式拼接路径；填成 `https://sub2.herohao.top/v1` 可能导致 `/v1/v1/models`。

保存后执行一次渠道测试，并在渠道详情确认模型列表和状态均为启用。若上游模型发生增删，可使用“检测上游模型变更”后再人工应用；不要在第一次接入时直接删除本地模型。

## 价格同步规则

价格接口：

```text
GET https://sub2.herohao.top/pricing/api/pricing
```

接口声明的单位是：

- `token.*`：元 / 百万 Token；
- `request.*`：元 / 次。

本项目约定把接口返回的 `CNY` 数值直接当作 new-api 的 `credit` 数值，**不做汇率换算**。只转换“单位说明”，不转换数值。

### Token 价格字段映射

适配器会读取顶层 `models[]`；如果不存在，则读取 `token.models[]`。对每个启用模型按名称匹配：

| HeroHao 字段 | new-api 含义 |
| --- | --- |
| `model` | 模型名（必须与渠道模型完全一致） |
| `prices.input.official` | 输入 Token 价格（对外售价） |
| `prices.output.official` | 输出 Token 价格（对外售价） |
| `prices.cacheRead.official` | 缓存读价格（对外售价） |
| `prices.cacheWrite.official` | 缓存写价格（对外售价）；缺失时按上游指南记为 0 |
| `enabled` | 为 `false` 时跳过该条目 |
| `manualOverride` | 当前适配器不读取该字段；标记为 `true` 的模型由管理员人工跳过或单独复核 |
| `tiers` | 当前适配器不会自动展开分档；需要人工转换为表达式后再保存 |

每条有效记录会生成类似下面的本地表达式（数值按接口原样保留）：

```text
p * <input.official> + c * <output.official> + cr * <cacheRead.official> + cc * <cacheWrite.official>
```

`official` 是对外展示和本地计费的价格来源；本项目约定把 HeroHao 返回的 CNY 数值直接作为 credit 数值使用，不做汇率换算。缺失或无法解析的价格条目会被跳过，不会生成免费价格。

建议正式迁移先选一个 Token 计费模型（例如 `deepseek-v4-flash-0731`），验证 Responses 的 usage 和额度变化后，再批量应用其余模型。

### DeepSeek 分时价格

HeroHao 价格响应里的 `tiers` 不会由当前适配器自动转换为时间条件。需要高峰/平时两档时，先同步平时 `official` 价格，再在该模型的表达式编辑器中保存两档表达式，使用项目约定的北京时间条件。保存后重新发起一次同步预览，确认该模型显示为 `tiered_expr`，且没有被普通数字价格覆盖。

### 按次价格

`request.rules[]` 是按次计费规则，字段含义如下：

- `models`：适用模型；
- `providerKeys`：适用上游 Key；
- `baseSelling`：基础每次价格；
- `tiers[].minTokens/maxTokens/selling`：按 Token 区间的每次价格；
- `enabled`：规则开关。

当前价格数据存在需要人工复核的异常，不能无审查地全量导入：

- `glm-5.2` 的基础价与第一档 tier 不一致；
- `Kimi-K2.6`、`deepseek-v4.1-flash` 存在高档价格低于低档价格的区间；
- `upstream:hy4-preview` 有启用规则但 selling 为空；
- 部分模型标记 `manualOverride=true`。

遇到这些情况，先保留人工配置或禁用该规则，并在价格核对表中记录原因。

## 测试验收（正式环境照此执行）

### 1. 创建下游临时 Key

在测试环境管理后台创建一个临时下游 API Key，给足够但有限的测试额度，并记录：

- Key 名称或末四位（不要记录完整 Key）；
- 测试用户/分组；
- 请求前 quota；
- 创建时间。

测试完成后先禁用；确认日志、计费和回滚检查完成后再删除。若需要保留排障窗口，记录 Key 名称和末四位即可，不能记录完整 Key。

### 2. 验证模型列表

```bash
export NEW_API_BASE='https://test.tryvalo.com'
export NEW_API_KEY='从测试后台临时复制，勿写入脚本或日志'
curl -sS "$NEW_API_BASE/v1/models" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  | jq '.data | length, .[].id'
```

应能看到 HeroHao 渠道启用的模型。若没有模型，先检查渠道状态、分组、模型同步结果和缓存刷新。

### 3. 验证 Chat Completions

```bash
curl -sS "$NEW_API_BASE/v1/chat/completions" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-v4-flash-0731",
    "messages": [{"role": "user", "content": "只回复 OK"}],
    "stream": false
  }' | jq '{id,model,usage,choices: [.choices[]?.message?.content]}'
```

### 4. 验证 Responses

```bash
curl -sS "$NEW_API_BASE/v1/responses" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-v4-flash-0731",
    "input": "只回复 OK"
  }' | jq '{id,object,status,model,usage,output}'
```

验收条件：`object` 为 `response`，`status` 为 `completed`，存在 `output`，且 `usage.total_tokens` 等于输入和输出 Token 之和。若使用流式请求，还要确认事件流能正常结束并最终产生 usage。

### 5. 核对计费

请求前后分别记录测试用户 quota，并在使用日志中按 request id 核对：

```text
实际扣减 = 输入 Token × input credit / 1,000,000
        + 输出 Token × output credit / 1,000,000
        + 缓存读/写 Token × 对应 cache credit / 1,000,000
```

如果只配置输入/输出价格，缓存字段应明确配置为 0 或沿用系统默认值，不能留有不确定的隐式覆盖。Responses 和 Chat Completions 应使用同一个模型价格身份；不要通过模型别名绕过价格配置。

### 6. 本次测试环境实测（2026-09-12）

本次在 `https://test.tryvalo.com` 完成了以下配置和验证：

- 渠道：`HeroHao-国模-测试`，渠道类型 `Sub2API`（类型编号 59），渠道 ID `35`，分组 `default`，Base URL 为 `https://sub2.herohao.top`；
- 上游 `/v1/models` 同步 21 个模型，`deepseek-v4-flash-0731` 在下游 `/v1/models` 中可见，并声明支持 `openai`、`openai-response` 等端点；
- 21 个模型均已保存为 `tiered_expr`；其中 4 个 DeepSeek 模型已按北京时间配置 `peak` / `off_peak` 两档，其余模型使用当前快照的普通时段价格。`deepseek-v4-flash-0731` 平时为输入 `0.08`、输出 `0.32`、缓存读取 `0.0016`、缓存写入 `0`，高峰为输入 `0.16`、输出 `0.64`、缓存读取 `0.0032`、缓存写入 `0`（按上游 CNY 数值直接填入 credit，不做汇率换算）；
- Chat Completions 返回 HTTP 200，`usage` 为 `prompt_tokens=33`、`completion_tokens=31`、`total_tokens=64`；
- 非流式 Responses 返回 HTTP 200，`object=response`、`status=completed`，`usage` 为 `input_tokens=33`、`output_tokens=33`、`total_tokens=66`；
- 流式 Responses 返回 `text/event-stream`，收到 `response.completed`，最终状态为 `completed`，usage 为 `input_tokens=33`、`output_tokens=25`、`total_tokens=58`；
- 使用日志生成 2 笔非流式和 4 笔流式消耗记录，显示模型、渠道、令牌、Token 数和价格档位均正确；非流式两笔合计费用约 `$0.000026`，页面当前时段累计用量显示约 `$0.000074`（流式调试请求包含重复读取事件流的验证请求）；
- 另用此前未配置价格的 `MiniMax-M3` 创建临时 Key 做 Responses 验证：HTTP 200，`status=completed`，usage 为 `input_tokens=181`、`output_tokens=2`、`total_tokens=183`；额度从 `100000000` 减少到 `99999984`，使用日志 quota 为 `16`，`billing_mode=tiered_expr`、`matched_tier=base`、`request_path=/v1/responses`、`use_channel=["35"]`；验证后已删除 Key。未在仓库、文档或日志中保存完整密钥。

本次批量导入的是价格接口当前快照中的普通 Token 价格，并将 4 个 DeepSeek 模型的工作日高峰时段 tiers 转换为北京时间请求条件表达式；其他模型的时段 tiers 尚未转换。正式迁移前应按“价格同步规则”逐模型审核是否需要保留时段差异，并保存新的价格快照。

## 正式环境迁移步骤

### 应用发布

先发布包含 `controller/ratio_sync.go` HeroHao 适配的应用版本，再进行后台配置。测试环境使用 `ops/greencloud/publish-test.sh` 发布不可变镜像；该脚本的默认目标是测试主机，不得直接当作正式环境发布命令。正式环境应沿用现有发布流程，并在发布记录中保存最终镜像标签和 Git SHA。

1. 导出测试环境已验收的渠道字段：类型、名称、Base URL、分组、模型、状态和价格配置；密钥单独通过正式环境密钥流程填写。
2. 在正式环境创建同类型 Sub2API 渠道，Base URL 保持 `https://sub2.herohao.top`，填入正式上游 Key，先保持渠道禁用或仅绑定测试分组。
3. 点击“从上游获取”，确认模型数量、模型 ID 和端点能力；首次接入不要直接删除本地已有模型。
4. 打开“系统管理 → 计费与支付 → 模型定价 → 上游价格同步”，选择 HeroHao 渠道并执行同步。系统会自动把该域名的价格地址规范为 `https://sub2.herohao.top/pricing/api/pricing`。
5. 只勾选审核通过的 `official` 价格，查看冲突预览后点击“应用同步”。`manualOverride` 模型和有分时规则的模型逐个复核；DeepSeek 高峰/平时价格按上一节手工保存。
6. 用正式环境临时下游 Key 做一次 `/v1/models`、Chat Completions、非流式 Responses 和流式 Responses 小额请求。
7. 对照请求日志、usage、quota 和价格档位完成核对，确认分组倍率为预期值后，再开放给正式分组。
8. 保留渠道变更记录、价格快照、同步时间和发布 Git SHA，作为下一次价格刷新和回滚依据。

### 管理后台操作清单

| 顺序 | 页面 | 操作 | 通过标准 |
| --- | --- | --- | --- |
| 1 | 渠道管理 | 新建/编辑 Sub2API 渠道 | Base URL 无 `/v1`，Key 已填写，渠道测试通过 |
| 2 | 渠道管理 | 从上游获取模型 | 模型数量与 `/v1/models` 一致，模型 ID 无误 |
| 3 | 模型定价 → 上游价格同步 | 选择 HeroHao 渠道并获取价格 | 测试结果成功，价格来源显示 HeroHao |
| 4 | 上游价格同步 | 逐条检查 `official`、缓存字段和分时模型 | 没有把 `selling/reference` 当成对外价格 |
| 5 | 模型定价 | 应用同步并检查表达式 | 模式为 `tiered_expr`，表达式和快照一致 |
| 6 | API/使用日志 | 发起四类请求并核对扣费 | HTTP 200、usage 完整、quota 和日志费用一致 |

## 回滚

- 立即将 HeroHao 渠道状态设为禁用，停止新流量；
- 保留已产生的使用日志和订单，不回写或手工冲正 quota；
- 恢复迁移前的模型列表和价格快照；
- 若仅是上游 Key 失效，替换 Key 后先做单渠道测试，再恢复状态；
- 若是价格异常，锁定受影响模型，禁止继续导入价格，不要删除其他模型配置。

## 已验证与待复核

已验证：上游 `/v1/models`、`/v1/chat/completions`、`/v1/responses` 均为可用的标准接口；模型列表与价格接口模型名一致；Sub2API 渠道类型具备 OpenAI/Responses 路由能力。

正式迁移前仍需在目标环境重新复核：渠道保存结果、下游 Key 权限、流式 Responses、实际 quota 扣减、使用日志，以及价格接口是否发生变化。上游价格页不是 new-api 的自动同步源，价格变更必须重新审核后导入。

## 当前上游价格快照（仅供导入审核）

抓取时间：`2026-09-12T03:51:05.479Z`。以下为 `token.models[]` 的 CNY selling 值，按本项目约定可直接作为 credit 数值；本表与本次 21 个模型的测试环境导入一致。

| 模型 | 输入 | 输出 | 缓存读 | 缓存写 | manualOverride |
| --- | ---: | ---: | ---: | ---: | --- |
| `deepseek-v4-flash-0731` | 0.0800 | 0.3200 | 0.0016 | 0.0000 | `true` |
| `deepseek-v4-flash-vision-exp` | 0.1200 | 0.4800 | 0.0024 | 0.0000 | `false` |
| `deepseek-v4-pro-0813` | 0.3600 | 1.0800 | 0.0120 | 0.0000 | `true` |
| `deepseek-v4.1-flash` | 0.1200 | 0.4800 | 0.0024 | 0.0000 | `false` |
| `glm-5.1` | 0.6400 | 2.2400 | 0.1600 | 0.0000 | `true` |
| `glm-5.2` | 0.6400 | 2.2400 | 0.1600 | 0.0000 | `true` |
| `glm-5.3` | 0.6400 | 2.2400 | 0.1600 | 0.0000 | `true` |
| `glm-5.3-flash` | 0.0960 | 0.3400 | 0.0280 | 0.0000 | `false` |
| `hy3` | 0.0800 | 0.3200 | 0.0200 | 0.0000 | `true` |
| `hy4-preview` | 0.4800 | 1.4400 | 0.0240 | 0.0000 | `true` |
| `kimi-k2.6` | 0.5200 | 2.1600 | 0.0880 | 0.0000 | `true` |
| `kimi-k2.7-code` | 0.5200 | 2.1600 | 0.1040 | 0.0000 | `true` |
| `kimi-k3` | 1.6000 | 8.0000 | 0.1600 | 0.0000 | `true` |
| `mimo-v2.5` | 0.0800 | 0.1600 | 0.0080 | 0.0000 | `true` |
| `mimo-v2.5-pro` | 0.2480 | 0.4880 | 0.0080 | 0.0000 | `true` |
| `MiniMax-M2.7` | 0.1680 | 0.6720 | 0.0336 | 0.2104 | `true` |
| `MiniMax-M2.7-highspeed` | 0.3360 | 1.3440 | 0.0336 | 0.2104 | `true` |
| `MiniMax-M3` | 0.1680 | 0.6720 | 0.0336 | 0.0000 | `true` |
| `qwen3.7-max` | 1.1600 | 3.4600 | 0.2400 | 1.4400 | `false` |
| `qwen3.8-flash` | 0.0770 | 0.2600 | 0.0160 | 0.0960 | `false` |
| `qwen3.8-max` | 1.1600 | 3.4600 | 0.2400 | 1.4400 | `false` |
