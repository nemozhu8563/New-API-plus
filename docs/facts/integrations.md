# 外部集成事实

本文件记录外部服务商、第三方接口、账号能力、假实现、沙箱和真实接入边界。不要写未确认的供应商能力。

## 外部服务清单

| 服务 | 用途 | 当前状态 | 证据来源 | 最近确认 |
| --- | --- | --- | --- | --- |
| AI 上游渠道 | OpenAI/Claude/Gemini 兼容请求及 AWS、Azure、国内外模型与异步任务渠道适配 | 已确认：代码适配器和路由存在。2026-09-01 生产库近 24 小时存在来自两个不同渠道的消费成功记录，所查询条件下没有渠道错误记录；测试库当前配置的渠道均为禁用。该快照不证明所有账号、模型和能力可用 | `relay/channel/`、`constant/channel.go`、`router/relay-router.go`、`router/video-router.go`；GreenCloud 生产/测试库只读聚合 | 2026-09-01 |
| OAuth 与外部身份 | GitHub、Discord、OIDC、LinuxDO、WeChat、Telegram 及数据库配置的自定义 OAuth | 已确认：注册器、路由和实现存在；目标环境启用项与端到端登录待定 | `oauth/`、`router/api-router.go`、`model/custom_oauth_provider.go` | 2026-08-31 |
| 支付 | Stripe、Creem、Waffo、Waffo Pancake、Epay 与余额支付相关路径 | 已确认：支付请求、回调路由和模型存在；Stripe Sandbox 的已验证范围见下行，其他支付商当前环境配置以及 Stripe Live 签名回调、结算与退款状态待定 | `router/api-router.go`、`controller/topup_*.go`、`model/topup.go` | 2026-09-01 |
| Stripe Sandbox 交易边界 | 已部署旧版的支付验收快照 | 部分已确认：旧版 CNY 20 充值及 2026-09-01 Standard `CNY 259/月` recurring 首购、invoice、首期权益和账单日期曾完成 E2E；当次 Automatic Tax 税额为 `0`、`product_exempt`。这不证明当前 one-time-only 已发布版本的真实付款、回调、权益或退款流程，旧模型与账单 UI 已从本地实现移除 | `docs/operations/2026-09-01-stripe-subscription-period-test-deployment.md`；当前代码；本次未重新读取旧交易对象 | 2026-09-07 |
| Stripe Live 配置 | 月度订阅、一次性充值、Webhook 与受限服务端凭据 | 已确认（2026-08-30 快照）：三档月付 Price、一个一次性 Price、Webhook endpoint、restricted key 与 signing secret 已配置并脱敏回读；真实 Live 充值/订阅、签名回调入账、续费、退款和争议仍待定 | `docs/operations/2026-08-26-project-operating-status.md` | 2026-08-30 |
| Stripe Sandbox / Live 一次性套餐 | 三档套餐每次付款提供一个月内部权益 | 已确认：两环境各有三档 active one-time Price，CNY 259/599/1099，`recurring=null`；Sandbox 公开 Plan `2/3/4` 和 Live 公开 Plan `1/2/3` 已分别绑定这些 Price，两个应用已发布相同镜像且公开套餐 API 均标记 Stripe Checkout 可用。旧 Product 默认 Price 保持不变；未创建真实交易，未修改 webhook、Tax 或支付方式配置 | Stripe API 创建及读取结果；GreenCloud PostgreSQL/Docker 与公开套餐 API 只读回读；`docs/operations/2026-09-07-stripe-one-time-cutover.md` | 2026-09-07 |
| Cloudflare Email Routing | `contract@tryvalo.com` 入站邮件转发 | 已确认（2026-08-27 快照）：路由规则、目标验证状态和公共 DNS 已验证；真实外部邮件到达目标邮箱的 E2E 待定 | `docs/operations/2026-08-27-tryvalo-email-routing.md` | 2026-08-27 |
| Redis | 配额、渠道与应用缓存 | 已确认：`REDIS_CONN_STRING` 配置和降级代码存在；2026-09-01 GreenCloud 生产 Redis 容器为 `running/healthy`、重启次数 `0`，最近五次容器健康检查均成功。缓存业务正确性和降级切换仍待定 | `common/redis.go`、`model/quota_reserve.go`、`.env.example`；GreenCloud Docker 只读回读 | 2026-09-01 |
| Google Analytics 4（GA4）/ Microsoft Clarity | 公开站点访问与产品流程分析 | 部分已确认：正式站点已向精确 `https://tryvalo.com` 运行时注入公开配置，前端在用户明确允许 analytics 前不加载 GA4/Clarity。GA4 当前客户端事件为 `sign_up`、`login`、`api_key_created`、`model_request_started`、`model_request_succeeded`、`checkout_created`，参数过滤拒绝邮箱、密码、Token、API Key、URL、用户/会话标识及内容类字段；支付确认和权益发放仍不向 GA4 发送。GA4 当前 provider UI 已读回 Tryvalo Web stream（`https://tryvalo.com`、Measurement ID `G-T2LD0R73QD`），Realtime 报告当前为“没有可用数据”。Clarity provider UI 已读回 Tryvalo project（Project ID `ycgor9smow`），但当前停留在 `0 / 7` 安装引导，Dashboard/live users/Recordings 被重定向，尚无 provider data 证据。生产页尚未得到用户主动 analytics 同意，因此 GA4/Clarity transport 仍待定；初始页面未加载两者远程资源 | `router/site-runtime.go`、`web/src/lib/site-telemetry.ts`、`web/src/components/site-telemetry-consent.tsx`、各业务事件调用点；GreenCloud 正式配置摘要、Tryvalo Chrome Network、GA4 Realtime 与 Clarity project UI 只读回读 | 2026-09-03 |
| Google Search Console（GSC）/ Bing Webmaster | 搜索发现、站点验证和 sitemap 状态 | 部分已确认：正式 `https://tryvalo.com/sitemap.xml` 为 HTTP `200` 的有效 XML，含 4 个 canonical URL；`/robots.txt` 声明该 sitemap。GSC `sc-domain:tryvalo.com` 已在当前 Google 账号下显示“您是经过验证的所有者”，提交后精确 sitemap 在列表中读回为 `成功`、4 URL。Bing 的精确站点 `https://tryvalo.com/` 已通过 GSC Import 导入并从 provider 页面读回；同一 sitemap 已提交一次，provider 原始状态为 `Submitted / Processing`，当前显示 1 个已知 sitemap、0 errors、0 warnings 和 `0 / -` discovered URLs。该异步状态不确认 Bing 已抓取或收录；IndexNow 未请求 | `router/seo.go`、`router/seo_test.go`、`router/main.go`；正式公网 XML/robots 响应；当前 GSC 与 Bing Webmaster provider UI 回读；`docs/operations/2026-09-03-tryvalo-telemetry-search-production-release.md` | 2026-09-03 |
| Umami | 既有可选前端分析脚本注入 | 已确认：旧 `InjectUmamiAnalytics()` 仍可独立根据环境变量替换 HTML 占位符，且不受本次 GA4/Clarity 的精确 origin 或 consent 控制；当前 GreenCloud 正式 `new-api.env` 不含 `UMAMI_WEBSITE_ID`，因此没有该脚本的生产注入 | `main.go`、`web/index.html`、`docker-compose.yml`；GreenCloud 正式配置键存在性只读回读 | 2026-09-03 |
| Pyroscope / Uptime Kuma / Turnstile / 邮件 | 可选性能观测、状态读取、反滥用和邮件能力 | 已确认：代码与配置入口存在；外部服务配置和实际可用性待定 | `main.go`、`router/api-router.go`、`.env.example` | 2026-08-31 |

## Checkout 与 GA4 页面上下文契约

已确认（2026-09-07，当前发布版本）：

- Stripe 套餐 Checkout 只接受固定单价的 one-time Price，固定使用 `mode=payment`，拒绝 recurring Price；金额、币种、livemode 与单月权益契约仍需匹配。套餐有效期、额度和取消由内部系统管理，不创建 Stripe Subscription，也不由 invoice/subscription 生命周期事件发放权益。
- 一次性 Stripe 付款按下单时保存的套餐权益快照发放，不采用付款后修改的标题、额度、周期、分组或钱包溢出设置；套餐被删除也不使已购快照失效。套餐仍存在时，完成订单仍检查当前购买次数限制。重复成功回调不会重复发放权益。
- 管理员可在后台账号的订阅管理中取消内部订阅，立即停止权益并保留历史记录；该操作不自动退款。退款/争议仍通过 PaymentIntent/Charge 引用回收对应权益或记录债务；退款失败/撤销不能重新激活管理员已取消的权益。旧 Stripe 自动续费取消及 portal API/UI 已移除；代码不再维护旧 invoice settlement/lock 模型，但没有执行远端表、列或数据删除。
- GA4 初始化、路由 page_view 和自定义事件统一设置安全页面上下文：`page_location` 仅保留当前 origin 与 pathname，`page_referrer` 仅保留合法 HTTP(S) 来源的 origin 与 pathname，query/hash 被移除，非法来源清空。业务调用方不能覆盖这两个安全值；精确 origin 和用户明确同意的门禁保持不变。

来源：`controller/subscription_payment_stripe.go`、`controller/stripe_checkout_test.go`、`controller/stripe_one_time_contract_test.go`、`controller/topup_stripe_test.go`、`model/subscription.go`、`model/subscription_cancel_test.go`、`model/stripe_payment_adjustment_test.go`、前端订阅取消交互测试及 GA4 测试；本地相关 Go 包测试、vet、根模块构建和前端类型检查、全量测试、构建通过。以上不确认真实 Stripe 交易、GA4 transport 或付款后的权益/退款 E2E。

## 假实现、沙箱和真实接入边界

- 本仓库中的适配器、SDK 依赖、路由或测试只证明代码边界存在，不证明外部账号已配置或生产调用成功。
- Waffo Pancake 回调路径以 `:env` 区分测试和生产注册槽位；当前目标环境注册结果待定。
- Stripe 本地测试覆盖一次性 Checkout 参数、recurring 拒绝、签名拒绝、Webhook claim/重试/幂等、订单快照、内部取消、退款和争议处理；invoice/subscription 事件不会发放权益。真实提供商回调和持久化读回只确认过旧版 Sandbox 充值与 recurring 首购，不能替代当前一次性套餐 E2E。
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
- 2026-09-07 已重新读取 Tryvalo Sandbox 和 Live 的 Price/Product 与 webhook 配置，并读取 GreenCloud 数据库映射、应用镜像和公开套餐 API；新 Price 绑定与应用切换已完成，但这不表示真实 Checkout、付款、回调或权益发放已通过验收。
- 2026-09-01 Sandbox 的 Automatic Tax 结果由 Stripe Managed Payments 生成，不是仓库代码显式开启；当次 `product_exempt` 且税额为 `0`，不证明非零税额计算、Live Tax 注册或申报准备度。本次没有修改 Tax 配置。
- 上游响应、网络超时、账号权限和地区限制均不能从本地静态检查与单元测试确认。

## 待确认事项

- 待定：生产实际启用的上游、OAuth、支付、邮件、分析和观测服务清单。
- 待定：各回调 URL 的平台登记、签名 secret 来源、最近成功回执和失败重试状态。
- 待定：真实 AI 渠道的 StreamOptions、工具调用、图片/音频/视频和 reasoning 能力矩阵。
- 待定：Stripe Sandbox 一次性套餐切换后的真实付款、异步回调、内部权益发放、退款和争议 E2E，以及一笔产生非零税额的真实计税测试。
- 待定：Stripe Live 一次性套餐的真实充值、套餐付款、权益入账、退款与争议闭环；自动续费不属于当前本地实现。
- 待定：Stripe Live Automatic Tax、有效税务注册和申报准备度。
- 待定：`contract@tryvalo.com` 从外部发信到目标邮箱的真实投递回读。
