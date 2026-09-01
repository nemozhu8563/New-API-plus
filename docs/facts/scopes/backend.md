# 后端与网关范围事实

## 范围

- 路径：根目录，以及 `router/`、`controller/`、`service/`、`model/`、`middleware/`、`relay/`、`setting/`、`common/`、`dto/`、`types/`、`constant/`、`oauth/`、`pkg/`。
- 技术栈及版本：Go `1.25.1` module directive、Gin `v1.9.1`、GORM `v1.25.2`。
- 相关横向事实：`docs/facts/architecture.md`、`docs/facts/product-domain.md`、`docs/facts/integrations.md`、`docs/facts/deployment.md`。

## 当前实现

根服务提供 Dashboard API、用户/管理员能力、OpenAI/Claude/Gemini 兼容 Relay、异步图片/音频/视频任务、渠道管理、计费、充值、订阅、审计日志、系统任务和静态前端托管。

## 事实来源

`go.mod`、`main.go`、`router/`、`controller/`、`service/`、`model/`、`relay/`、`setting/`、`common/`、`.github/workflows/ci.yml`，2026-08-31 的测试、vet 和 build 输出，以及 2026-09-01 的 GreenCloud 应用、PostgreSQL、Redis 和生产日志聚合只读回读。

## 如何承载全局业务规则

- 认证与权限由 `middleware/`、`router/` 和 `service/authz/` 承载。
- 渠道分发、协议适配、预扣和结算由 `middleware.Distribute`、`relay/`、`relay/helper/`、`service/` 与 `model/` 承载。
- 提示词敏感词策略由 `setting/` 保存三个独立列表和统一快照版本，`service/` 按高风险阻断、NSFW 阻断、仅审计的顺序匹配，`controller/relay.go` 对阻断命中返回既有的 `403 content_policy_violation`，对仅审计命中写日志后放行。兼容名称 `SensitiveWords` 现在仅代表 NSFW 阻断列表，新增持久化选项为 `SensitiveWordsHighRisk` 和 `SensitiveWordsAudit`。
- 用户、Token、Channel、Log、Task、TopUp 和订阅对象由 `model/` 持久化并通过 service/controller 暴露。
- Stripe 月付首购由 `subscription_orders` 记录订单、提供商绑定和当前账期，由 `stripe_webhook_events` 记录回调处理，由 `user_subscriptions` 和 `stripe_subscription_settlements` 记录首期权益与结算，并以 `top_ups` 保存支付镜像。`invoice.paid` 会把服务周期结束时间写入订单；读取账单时，订单账期缺失或早于最新已付 settlement 会采用 settlement 的 `MAX(period_end)`。2026-09-01 测试环境已通过真实 Sandbox 首购和钱包页回读验证该链路。

## 接口、页面、集合或模块

- Dashboard/API：`/api`。
- 同步 Relay：`/v1`、`/v1beta`、`/pg`。
- 异步任务：`/mj`、`/suno`、视频与 Kling/Jimeng 路由。
- 主要模块：`router`、`controller`、`service`、`model`、`middleware`、`relay`、`setting`、`common`。

## 验证状态

已确认：2026-08-31 执行 `make test`、`GOWORK=off go vet ./...` 和 `GOWORK=off go build ./...` 均通过。2026-09-01 敏感词分级改造通过 `setting`、`service`、`model`、`controller` 四个包的完整测试；默认词表互斥性、完整覆盖、选项持久化、匹配优先级、审计放行和阻断响应均有回归测试。同日该提交发布到 GreenCloud 测试环境，真实接口确认 NSFW 和高风险命中返回 `403 content_policy_violation`，audit 命中越过本地策略并写入审计日志；测试上游凭据随后返回 `401 Invalid API key`，所以允许路径成功生成仍未确认。2026-09-01 现场还确认生产和测试应用、生产 PostgreSQL/Redis 健康，公开与本机状态接口成功，且生产库近 24 小时存在部分渠道的消费成功记录；未主动验证全部上游模型。同日测试环境完成 Stripe Sandbox Standard 月付首购账期复验：目标 webhook 均一次成功，最新订单、唯一 settlement 和唯一 active 权益的结束时间一致，钱包 quota 未被当作订阅额度直接增加；历史订单字段保持原值，但账单读取和钱包页已从 settlement 恢复下次账单日期。`go test ./model ./controller -count=1` 在宿主可见边界重跑通过。

## 已知约束

- 根模块编译依赖已存在的 `web/dist`，CI 会先创建 embed placeholder，生产构建会先构建真实前端。
- 主数据库必须保持 SQLite、MySQL、PostgreSQL 兼容；日志数据库另支持 ClickHouse。
- 敏感词分级能力已提交并发布到测试环境，但尚未 push 或发布到生产。测试库没有三项敏感词 option 持久化行，因此测试运行镜像内置的三类默认词表；数据库中已持久化的旧 `SensitiveWords` 仍会覆盖内置 NSFW 默认表，生产在发布代码和审查三项 option 迁移前不会自动获得新的默认分类。
- `Task.ID` 当前 tag 与根 `AGENTS.md` 的主键规则存在冲突，见 `docs/facts/current-system.md`。
- 新产生的 Stripe 首期 invoice 会把结束时间写入订单、`user_subscriptions` 和 settlement。历史订单的 `stripe_current_period_end=0` 不会被读取路径回写，但 API 会采用最新已付 settlement，因此测试站已能显示其下次账单日期。
- 本轮账期查询和回归测试使用 SQLite；查询由 GORM 组合且未发现方言专属语法，但尚未在真实 MySQL 或 PostgreSQL 实例上执行该路径。

## 待确认事项

- 待定：数据库 dialect 兼容矩阵、Redis 降级与多节点行为，以及生产数据库/缓存的完整业务正确性。
- 待定：所有 Relay provider 的端到端能力和生产可用性。
- 待定：Stripe Sandbox 续费、退款和争议触发后的完整后端持久化闭环，以及 Stripe Live 支付生命周期。
