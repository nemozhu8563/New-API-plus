# 外部集成事实

本文件记录外部服务商、第三方接口、账号能力、假实现、沙箱和真实接入边界。不要写未确认的供应商能力。

## 外部服务清单

| 服务 | 用途 | 当前状态 | 证据来源 | 最近确认 |
| --- | --- | --- | --- | --- |
| AI 上游渠道 | OpenAI/Claude/Gemini 兼容请求及 AWS、Azure、国内外模型与异步任务渠道适配 | 已确认：代码适配器和路由存在。2026-09-01 生产库近 24 小时存在来自两个不同渠道的消费成功记录，所查询条件下没有渠道错误记录；测试库当前配置的渠道均为禁用。该快照不证明所有账号、模型和能力可用 | `relay/channel/`、`constant/channel.go`、`router/relay-router.go`、`router/video-router.go`；GreenCloud 生产/测试库只读聚合 | 2026-09-01 |
| OAuth 与外部身份 | GitHub、Discord、OIDC、LinuxDO、WeChat、Telegram 及数据库配置的自定义 OAuth | 已确认：注册器、路由和实现存在；目标环境启用项与端到端登录待定 | `oauth/`、`router/api-router.go`、`model/custom_oauth_provider.go` | 2026-08-31 |
| 支付 | Stripe、Creem、Waffo、Waffo Pancake、Epay 与余额支付相关路径 | 已确认：支付请求、回调路由和模型存在；Stripe Sandbox 的已验证范围见下行，其他支付商当前环境配置以及 Stripe Live 签名回调、结算与退款状态待定 | `router/api-router.go`、`controller/topup_*.go`、`model/topup.go` | 2026-09-01 |
| Stripe Sandbox | 单次充值和月度订阅测试 | 部分已确认：CNY 20 单次充值闭环已完成。2026-09-01 Standard `CNY 259/月` 首购 E2E 又确认 Checkout `complete/paid`、invoice `paid`、subscription `active`，目标 webhook 均为 `succeeded` 且 `attempts=1`；最新订单账期 `1790838860` 与 invoice、唯一 settlement 和唯一 active 权益的结束时间一致，原始额度 `145000000` 显示为 290 Credits、已用 `0`。历史订单未做字段回填，但读取时可从最新已付 settlement 恢复账期，钱包页已显示下次账单日期。最新 Checkout 的 `managed_payments.enabled=true`，Checkout/subscription/invoice 均启用 Automatic Tax；Checkout 的 `liability.type=stripe`，invoice 税额为 `0`、`taxability_reason=product_exempt`。应用代码没有显式设置 `automatic_tax`。续费、退款、争议和真实非零税额交易仍待定 | 真实 Hosted Checkout；Stripe Sandbox Checkout/subscription/invoice 只读回读；GreenCloud `newapi_test` 只读回读；`model/subscription.go`、`model/subscription_billing_test.go`、`controller/topup_stripe_test.go`；`docs/operations/2026-09-01-stripe-subscription-period-test-deployment.md` | 2026-09-01 |
| Stripe Live 配置 | 月度订阅、一次性充值、Webhook 与受限服务端凭据 | 已确认（2026-08-30 快照）：三档月付 Price、一个一次性 Price、Webhook endpoint、restricted key 与 signing secret 已配置并脱敏回读；真实 Live 充值/订阅、签名回调入账、续费、退款和争议仍待定 | `docs/operations/2026-08-26-project-operating-status.md` | 2026-08-30 |
| Cloudflare Email Routing | `contract@tryvalo.com` 入站邮件转发 | 已确认（2026-08-27 快照）：路由规则、目标验证状态和公共 DNS 已验证；真实外部邮件到达目标邮箱的 E2E 待定 | `docs/operations/2026-08-27-tryvalo-email-routing.md` | 2026-08-27 |
| Redis | 配额、渠道与应用缓存 | 已确认：`REDIS_CONN_STRING` 配置和降级代码存在；2026-09-01 GreenCloud 生产 Redis 容器为 `running/healthy`、重启次数 `0`，最近五次容器健康检查均成功。缓存业务正确性和降级切换仍待定 | `common/redis.go`、`model/quota_reserve.go`、`.env.example`；GreenCloud Docker 只读回读 | 2026-09-01 |
| Google Analytics / Umami | 可选前端分析脚本注入 | 已确认：注入代码和配置入口存在；当前环境是否启用待定 | `main.go`、`common`/`setting` 相关配置、`.env.example` | 2026-08-31 |
| Pyroscope / Uptime Kuma / Turnstile / 邮件 | 可选性能观测、状态读取、反滥用和邮件能力 | 已确认：代码与配置入口存在；外部服务配置和实际可用性待定 | `main.go`、`router/api-router.go`、`.env.example` | 2026-08-31 |

## 假实现、沙箱和真实接入边界

- 本仓库中的适配器、SDK 依赖、路由或测试只证明代码边界存在，不证明外部账号已配置或生产调用成功。
- Waffo Pancake 回调路径以 `:env` 区分测试和生产注册槽位；当前目标环境注册结果待定。
- Stripe 本地测试覆盖 Checkout 参数、签名拒绝、Webhook claim/重试/幂等、`invoice.paid`、账期保存与历史 settlement 回退、退款和争议处理代码；真实提供商回调和持久化读回目前只确认了上表记录的 Sandbox 单次充值与月付首购，续费、退款和争议不能据此视为已完成。
- 不适用：files、fine-tunes、image variations 等路由当前明确挂载到 `RelayNotImplemented`，因此不属于当前已实现的 Relay 能力；不能把路由存在误写为功能可用。

## 凭据和密钥来源

- 上游渠道凭据由 Channel 配置/数据库与相关设置提供；OAuth、支付、Session、分析和观测凭据由环境变量、系统设置或平台 secret 提供。
- GitHub Actions 发布任务从仓库 Secrets 读取 Docker Hub 和签名凭据；Facts 不记录其值。
- `.env.example`、Compose 和 CI 文件是变量名与配置边界来源，不是可直接使用的生产凭据来源。
- 本 Facts 包未记录密钥、令牌、密码、Cookie、私钥或完整连接串。

## 回调和网络边界

- 支付回调包括 `/api/stripe/webhook`、`/api/creem/webhook`、`/api/waffo/webhook`、`/api/waffo-pancake/webhook/:env` 和 Epay notify 路由。
- OAuth 包括统一 `/api/oauth/:provider`、WeChat/Telegram 专用路由以及自定义 OAuth provider 管理接口。
- Relay 对外提供 `/v1`、`/v1beta`、`/mj`、`/suno`、`/kling/v1` 和视频任务相关入口；鉴权与限流按路由组分别配置。
- `SESSION_COOKIE_SECURE=true` 时，refresh/logout 路由启用严格 OriginGuard，且必须配置只含精确 HTTPS Origin 的 `SESSION_COOKIE_TRUSTED_URL`；该列表不是 CORS 白名单。关闭 Secure 模式时 OriginGuard 关闭且不得配置该列表。
- `TRUSTED_PROXIES` 未配置时只默认信任 loopback、RFC1918 和 IPv6 ULA；`none` 表示不信任任何代理；显式 IP/CIDR 列表会完全替代默认值，非法配置会阻止启动。
- `TRUSTED_REDIRECT_DOMAINS` 约束支付成功/取消回跳域名。以上配置的当前生产值均待定。

## 协议兼容与计费边界

- 当前 Claude 兼容规则将名称以 `claude-` 开头且版本段为 `4-6` 或 `4-7` 的模型视为不支持 assistant prefill。非 passthrough 请求仅在末条消息是纯文本 assistant 时追加 wire-only user continuation；已有消息的顺序与内容保持不变，转换写入 `request_conversion_meta`，并重新估算 prompt tokens。实现与回归测试位于 `relay/claude_prefill_compat.go`、`relay/claude_handler.go` 和 `relay/claude_prefill_compat_test.go`。
- OpenAI Responses 的流式与非流式处理均识别 `image_generation_call`。结果项必须包含非空最终结果，且其状态不能是 failed、incomplete、cancelled/canceled 或 partial；若整个响应以 failed、incomplete 或 cancelled/canceled 结束，已观察到的图片结果也不计费。重复事件会按输出标识或结果去重，最终计数受 `dto.MaxImageN` 限制。工具价格来自可由运营配置覆盖的统一价格索引，不采用旧文档中的固定九档矩阵。实现与测试位于 `relay/common/tool_usage.go`、`relay/channel/openai/relay_responses.go`、`relay/channel/openai/stream_buffer.go` 和对应测试。

## 已知限制

- 外部服务清单是代码边界摘要，不是逐供应商、逐模型、逐区域能力矩阵。
- 本次对 GreenCloud 生产/测试渠道配置和日志只做脱敏聚合读取，没有读取渠道凭据，也没有主动发起付费模型调用；因此只能确认部分生产上游近期成功，不能确认逐模型能力。
- 仓库内的 2026-08-27/30 运维记录提供 Stripe 与邮件配置/交易快照；2026-09-01 重新连接了 GreenCloud、当前公网和 Tryvalo Stripe Sandbox，但没有重新连接 Stripe Live 或 Cloudflare 控制台。
- Stripe Sandbox 最新月付首购已将账期同时写入订单、`user_subscriptions` 和 settlement；较早订单的 `stripe_current_period_end=0` 历史值仍保留，但账单读取会在订单字段缺失或较旧时采用最新已付 settlement，钱包页已确认能显示历史订单的下次账单日期。
- Stripe Sandbox 最新对象上的 Automatic Tax 是 Stripe Managed Payments 生成结果，不是仓库代码显式开启的能力；本轮 `product_exempt` 且税额为 `0`，不证明非零税额计算、Live Tax 注册或申报准备度。
- 上游响应、网络超时、账号权限和地区限制均不能从本地静态检查与单元测试确认。

## 待确认事项

- 待定：生产实际启用的上游、OAuth、支付、邮件、分析和观测服务清单。
- 待定：各回调 URL 的平台登记、签名 secret 来源、最近成功回执和失败重试状态。
- 待定：真实 AI 渠道的 StreamOptions、工具调用、图片/音频/视频和 reasoning 能力矩阵。
- 待定：Stripe Sandbox 月度订阅续费、退款和争议 E2E，以及一笔产生非零税额的真实计税测试。
- 待定：Stripe Live 真实充值、订阅、`invoice.paid` 权益入账、续费、退款与争议闭环。
- 待定：Stripe Live Automatic Tax、有效税务注册和申报准备度。
- 待定：`contract@tryvalo.com` 从外部发信到目标邮箱的真实投递回读。
