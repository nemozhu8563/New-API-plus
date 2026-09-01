# 业务领域事实

本文件记录跨范围共同成立的业务对象、业务规则、状态语义、权限语义和业务不变量。不要写某个子项目的具体实现细节。

## 业务对象

| 对象 | 当前含义 | 适用范围 | 确认来源 |
| --- | --- | --- | --- |
| User | 账号主体，持有角色、状态、钱包额度、负债、分组、邀请关系和外部支付客户标识 | 全局 | `model/user.go` 的 `User` |
| UserSession | 用户登录会话及撤销、签发和审计边界 | Dashboard/API | `model/user_session.go`、`router/api-router.go` |
| Token | API 调用凭据，绑定用户、额度、过期时间、模型限制、分组和跨组重试设置 | Relay | `model/token.go` 的 `Token` |
| Channel | 上游渠道配置，包含类型、状态、权重、BaseURL、模型映射、分组、优先级和多 Key 信息 | Relay | `model/channel.go` 的 `Channel` |
| Log | 消费与请求审计记录，关联用户、Token、模型、渠道、分组、token 用量、quota 和 request ID | 全局 | `model/log.go` 的 `Log` |
| Task | Midjourney、Suno、视频等异步上游任务，保存平台、渠道、预扣额度、状态、进度和结果数据 | 异步 Relay | `model/task.go` 的 `Task` |
| TopUp | 钱包充值订单，关联金额、兑换 quota、交易号、支付方式、提供商标识和状态 | 钱包 | `model/topup.go` 的 `TopUp` |
| SubscriptionPlan、SubscriptionOrder、UserSubscription | 订阅计划、支付订单和用户权益，配套锁与预扣记录 | 订阅 | `model/subscription.go`；`model/main.go` 迁移清单 |
| AffiliateAgent、AffiliateCommission、AffiliateConversion、AffiliateWithdrawal | 精选代理策略、邀请奖励账本、返现转站内额度和提现审核记录 | 钱包/管理端 | `model/affiliate_cashback.go` |

## 业务规则

- Relay 路由按入口使用 Token、用户会话或二者之一进行认证，并在发送上游前执行性能检查、模型限流和渠道分发；具体中间件组合以 `router/relay-router.go` 和 `router/video-router.go` 为准。
- 提示词敏感词策略在检查开关启用时按 `high_risk` 阻断、`nsfw` 阻断、`audit` 仅审计的固定优先级求值；同一词出现在多个管理员列表时，阻断优先于仅审计。`high_risk` 和 `nsfw` 命中返回既有的 HTTP `403 content_policy_violation` 且不重试，`audit` 命中记录类别、动作和命中词后继续请求。当前默认词表把 2,094 个有效来源词互斥划分为 475 个高风险阻断词、548 个 NSFW 阻断词和 1,071 个仅审计词；三个管理员选项可以分别完整覆盖各自列表。实现与测试位于 `setting/sensitive.go`、`service/sensitive.go`、`controller/relay.go` 及对应测试。
- 用户钱包与 Token 的配额预扣在 Redis 可用时使用 Lua 原子更新；缓存缺失或错误时回退数据库条件更新；持久化失败时尝试补偿缓存。主实现位于 `model/quota_reserve.go`。
- 充值在付款前检查钱包容量，结算时再次用带上限条件的原子更新限制 int32 额度边界，见 `model/topup.go`。
- quota 数值转换集中在 `common/quota_math.go`；越界或 NaN 会饱和到 int32 边界并生成可审计的 `QuotaClamp`，严格预扣版本返回错误。
- 分层计费表达式从配置存储进入预扣、结算和日志展示；实际结算使用捕获的 billing snapshot 与上游实际 usage，见 `pkg/billingexpr/expr.md`。
- Dashboard Access Token 是 15 分钟 JWT；登录 Session 最长 30 天。Refresh Token 是不透明值，服务端只保存 HMAC 摘要，客户端通过 `HttpOnly`、`SameSite=Strict` Cookie 持有并在刷新时轮换，见 `service/auth_token.go`、`service/auth_session.go` 和 `model/user_session.go`。
- 公开组计费先生成 `GroupBillingResolution`：默认归因和倍率回退到公开组；存在唯一启用 tag 时，路由与计费归因切换到该 tag；显式 model-tag override 是严格路由，目标 tag 不可用时直接报错；多 tag 且无 override 时保持公开组回退。实现与测试位于 `service/group_tag_resolver.go` 和 `service/group_tag_resolver_test.go`。
- 兑换码邀请奖励分为互斥的两条路径：启用的精选代理对受邀用户的每次成功兑换按配置的 `5%`～`10%` 比例记入独立返现账户；否则普通邀请人只在每名受邀用户首次成功兑换时获得固定 `5%` 站内额度。返现转站内额度有独立账本；提现流程只冻结并记录管理员的线下处理决定，不会自动发起外部转账。实现与测试位于 `model/affiliate_cashback.go`、`model/affiliate_cashback_test.go` 和 `controller/affiliate_cashback_test.go`。

## 状态语义

- 异步任务状态包含 `NOT_START`、`SUBMITTED`、`QUEUED`、`IN_PROGRESS`、`FAILURE`、`SUCCESS` 和 `UNKNOWN`，定义在 `model/task.go`。
- User、Token、Channel、TopUp、订阅计划与订阅实例各自拥有独立状态字段；这些字段不是 Facts 的“已确认/待定/冲突/不适用”状态。
- 支付代码路径存在不等于真实支付完成；外部结算和权益状态只有在对应签名回调与持久化结果得到目标环境证据时才成立。

## 权限语义

- `/api` 路由区分匿名、已登录用户、管理员和导航模块授权，使用 `UserAuth`、`AdminAuth`、`HeaderNavModuleAuth` 等中间件。
- Relay API 主要使用 Token 鉴权；Playground 和部分视频代理允许用户会话或 Token，具体以路由中间件为准。
- 系统启动后周期同步授权策略，以在多节点部署中传播权限变化；实现入口为 `service/authz` 和 `main.go`。

## 业务不变量

- 负数 quota 不能进入用户或 Token 预扣；余额不足时预扣失败而不是产生负余额。
- 单次 quota 转换不能因整数溢出把收费变成负向额度；越界必须钳制、报错或留下审计标记。
- 充值后的钱包额度不能达到或超过 `common.MaxQuota`；付款前检查与回调结算都执行容量约束。
- `relaykit` 转换层只表达协议与 DTO，不持有根服务的数据库、运行配置或业务状态。
- 已识别的旧 Refresh Token 在 30 秒竞态窗口之后再次使用会撤销整个登录会话；未知摘要只拒绝请求，不撤销仍有效的会话。

## 关键流程

- API 消费：Token 鉴权 → 模型/渠道分发 → 预扣 quota → 上游调用 → 解析实际 usage → 结算差额 → 写消费日志。
- 钱包充值：创建支付请求 → 外部支付 → 回调验签与幂等处理 → 原子增加 quota → 更新充值记录。
- 订阅：读取计划 → 创建订单/订阅 → 支付回调或周期任务 → 权益与周期额度更新；真实环境状态待定。
- 异步任务：提交并预扣 → 保存 Task → 周期轮询 → 成功/失败结算与结果更新。

## 术语和显示含义

- `quota` 是项目内部统一计费额度，不等同于直接显示的法币金额。
- `group` 同时参与用户/Token 可用范围、渠道选择和定价倍率。
- `channel` 表示可被调度的上游提供方配置；`model` 表示客户端或上游模型标识，二者通过能力与映射关联。
- `relay` 表示将兼容协议请求转换并转发给上游的主链路。

## 待确认事项

- 待定：生产配置下 quota 与各币种金额的当前换算、折扣和分组策略。
- 待定：所有支付提供商的实际启用状态、沙箱/生产边界、退款和订阅权益证据。
- 待定：所有业务对象状态的完整迁移图和跨提供商失败恢复语义。
