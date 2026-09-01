# 当前系统事实

本文件记录系统现在已知是什么。不要把未来计划写进这里。

`current-system.md` 是事实总览和索引，不承载分类事实正文。写入 `docs/facts/**` 前先读 `docs/facts/AGENTS.md` 和 `docs/facts/STANDARD.md`。

## 状态

已确认：当前仓库由根 Go API 网关服务、独立 Go 模块 `relaykit/` 和 React 前端 `web/` 三个稳定范围组成。根服务将 `web/dist` 嵌入可执行文件，也可以通过 `FRONTEND_BASE_URL` 将未匹配请求重定向到独立前端。2026-08-31 在以提交 `bdbc07608167` 为基线的当前工作树上完成了根模块与 `relaykit` 测试、静态检查和构建，以及前端类型检查、测试和构建。

已确认（2026-09-01 21:28～21:40，Asia/Shanghai）：GreenCloud 正式应用已运行提交 `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75` 的不可变 `linux/amd64` 镜像 `new-api:new-api-release-20260901T132333Z-fc6ebe122e`，其镜像 ID 与已验收测试镜像相同。正式应用为 `running/healthy`、重启次数 `0`，本机及 `api.tryvalo.com`、`new.tryvalo.com` 的 `/api/status` 均返回 HTTP `200`；PostgreSQL 和 Redis 未重建。测试应用此前已运行同一提交，测试渠道 `1` 在接口验收后恢复禁用。

已确认（2026-09-01，Asia/Shanghai）：`test.tryvalo.com` 已完成 Stripe Sandbox Standard `CNY 259/月` 首购、账期写入和账单日期显示 E2E。最新订单的账期与 invoice、唯一 settlement 及唯一 active 权益一致，290 Credits 权益已生效且已用额度为 `0`；历史订单没有做字段回填，但账单 API 已从最新已付 settlement 恢复账期，钱包页显示有效下次账单日期。详细对象状态、Automatic Tax 边界和发布证据由 `docs/facts/integrations.md` 与 `docs/operations/2026-09-01-stripe-subscription-period-test-deployment.md` 承载。该结果不确认续费、退款或争议。

已确认（2026-09-01，代码、GreenCloud 测试与正式环境）：提示词敏感词已从单一硬拦截表改为高风险硬拦截、NSFW 硬拦截和仅审计放行三层策略，默认 2,094 个有效来源词被互斥且完整地划分为 `475 + 548 + 1,071` 条。改动已提交为 `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`，测试与正式环境当前运行相同镜像 ID。真实测试接口确认 `成人色情` 与 `炸弹制作` 分别按 NSFW 和高风险策略返回 `403 content_policy_violation`，`淫威` 记录 audit 后越过本地策略；普通请求和 audit 请求随后均因测试渠道上游凭据无效返回 `401 Invalid API key`，因此允许路径成功生成仍为待定。正式库发布前存在的旧 `SensitiveWords` 覆盖已备份后删除，三项敏感词 option 当前均无持久化行，正式运行时使用镜像内置三层词表。生产业务请求 E2E 未执行。

## 事实文件索引

| 事实文件 | 状态 | 用途 |
| --- | --- | --- |
| `docs/facts/architecture.md` | 已确认 | 技术栈、模块边界、运行方式、共享契约位置。 |
| `docs/facts/product-domain.md` | 已确认 | 跨范围共同成立的业务对象、业务规则、状态语义和业务不变量。 |
| `docs/facts/ui-style.md` | 已确认 | 全局界面风格、交互原则、视觉约束和组件库使用边界；浏览器视觉验收仍为待定。 |
| `docs/facts/integrations.md` | 已确认 | 外部服务代码边界、协议兼容、回调入口、部分真实上游活动及带日期的 Stripe/邮件快照；Stripe Sandbox 首次月付订阅、账期和账单日期已完成 E2E，未执行的续费、退款、争议及 Live Tax 场景保持待定。 |
| `docs/facts/deployment.md` | 已确认 | 仓库内部署、数据库和迁移事实，以及 2026-09-01 GreenCloud、Zgo 和公网运行快照；备份和回滚演练仍未闭合。 |
| `docs/facts/verified-commands.md` | 已确认 | 项目命令来源及 2026-08-31 的本地验证结果。 |

## 生效技术栈

| 技术栈及版本 | 适用范围 | 确认来源 | 状态 |
| --- | --- | --- | --- |
| Go `1.25.1` module directive；本次验证运行时 `go1.26.3 darwin/arm64` | 根服务 | `go.mod`；`go version` 运行结果 | 已确认 |
| Go `1.25.1` module directive | `relaykit/` | `relaykit/go.mod` | 已确认 |
| React `^19.2.7`、Rsbuild `^2.1.4`、Base UI `^1.6.0`、Tailwind CSS `^4.3.2`、Bun | `web/` | `web/package.json`；`web/AGENTS.md`；本次 Bun `1.3.14` | 已确认 |
| Gin `v1.9.1`、GORM `v1.25.2`、Redis client `v8.11.5` | 根服务 | `go.mod` | 已确认 |

## 范围映射

| 范围 | 路径 | 技术栈 | 事实文件 | 确认来源 |
| --- | --- | --- | --- | --- |
| 后端与网关 | 根目录及 `router/`、`controller/`、`service/`、`model/`、`relay/` 等 | Go、Gin、GORM | `docs/facts/scopes/backend.md` | `go.mod`、`main.go`、`router/`、`model/`、`relay/` |
| Relay 转换契约 | `relaykit/` | 独立 Go module | `docs/facts/scopes/relaykit.md` | `relaykit/go.mod`、`relaykit/relayconvert/`、独立构建结果 |
| 管理与用户前端 | `web/` | React、TypeScript、Rsbuild、Base UI、Tailwind CSS、Bun | `docs/facts/scopes/frontend.md` | `web/package.json`、`web/src/`、前端检查结果 |

## 当前最大待确认事项

- 待定：各 AI 渠道和模型的完整能力矩阵；2026-09-01 的运行快照只确认生产中部分渠道近期产生消费成功记录，未主动发起逐模型付费探测。
- 待定：为 GreenCloud 测试环境恢复一个有效且默认禁用的上游测试凭据，再完成普通文本和 audit 文本的 HTTP `200` 生成 E2E；当前渠道 `1` 返回 `401 Invalid API key`。
- 待定：使用单独授权的可控生产凭据完成普通文本和 audit 文本成功生成 E2E，并复核 NSFW、高风险阻断及策略日志；当前只确认生产运行已验收镜像与内置三层词表，没有发起生产业务请求。
- 待定：Stripe Sandbox 月度订阅续费、退款和争议 E2E。首次 Checkout、真实 `invoice.paid`、首期权益、账期持久化及账单日期显示已确认。
- 待定：Stripe Live 真实充值、订阅、签名回调、结算、续费、退款和争议闭环。
- 待定：Stripe Live Automatic Tax、有效税务注册和申报准备度；Sandbox 本轮 invoice 税额为 `0` 且原因为 `product_exempt`，不能据此确认真实计税交易或 Live 税务状态。
- 待定：本次正式发布已保留 Compose、镜像配置、敏感词 option 导出和 PostgreSQL custom-format dump，但生产数据库完整恢复演练、Redis/应用数据与日志 volume 备份、监控告警和可执行全栈回滚流程仍未闭合。

## 最近事实刷新

| 刷新范围 | 日期 | 依据 | 结果 |
| --- | --- | --- | --- |
| 全部 Facts 与三个范围文件 | 2026-08-31（Asia/Shanghai） | 当前代码、测试、配置、`makefile`、GitHub Actions、Docker 文件和本次命令输出 | 已确认：建立当前事实索引；未能由仓库和本地运行证明的外部状态保持待定。 |
| 既有文档可复用事实 | 2026-09-01（Asia/Shanghai） | `docs/authentication.md`、渠道/计费 solutions、运维状态记录，并以当前代码和定向测试交叉复核 | 已确认：纳入稳定实现契约和带日期的远端快照；旧流程、计划、环境实例值及未复核结论未纳入。 |
| GreenCloud、Zgo、公网、生产聚合和 Stripe 测试边界 | 2026-09-01 09:21～09:33（Asia/Shanghai） | 当前 DNS/HTTP 响应头；GreenCloud 与 Zgo SSH 只读回读；Docker、PostgreSQL 聚合和既有 Stripe 验收记录 | 已确认：当前应用与依赖健康、生产边缘路径和部分真实上游活动；Stripe Sandbox 未完成的生命周期继续保持待定。 |
| Stripe Sandbox 首次月付订阅与账期修复 | 2026-09-01（Asia/Shanghai） | 两轮真实 Hosted Checkout、Stripe Sandbox Checkout/subscription/invoice 只读回读、GreenCloud 测试库、定向回归测试及钱包页回读 | 已确认：Standard `CNY 259/月` 首购、真实 `invoice.paid`、290 Credits 权益和持久化闭环成功；新订单账期与 invoice/settlement/权益一致，历史订单由 settlement 恢复账单日期。续费、退款和争议仍待定。 |
| 敏感词分级策略与 GreenCloud 测试发布 | 2026-09-01 15:06～15:27（Asia/Shanghai） | 精确提交与不可变镜像、三类默认词表、本地 backend/frontend 检查、GreenCloud Docker/PostgreSQL 回读、四组真实测试接口请求 | 已确认：高风险与 NSFW 本地阻断、仅审计放行和测试部署；允许路径因上游 `401 Invalid API key` 未取得成功模型响应。当时正式发布与 option 处理尚未执行，后续结果见下一行。 |
| 敏感词分级策略 GreenCloud 正式发布 | 2026-09-01 21:28～21:40（Asia/Shanghai） | 与测试相同的不可变镜像 ID、GreenCloud Docker/PostgreSQL 回读、备份校验、本机与两个正式公网入口状态接口 | 已确认：正式应用健康、三项持久化 option 均为 0 行、镜像内三层词表生效，PostgreSQL/Redis 未重建；生产业务请求 E2E 未执行。 |

## 待解决事实冲突

| 主张 | 冲突证据 | 当前处理 |
| --- | --- | --- |
| Task 主键是否遵循项目数据库约束 | 根 `AGENTS.md` 要求让 GORM 处理主键且不得直接使用 `AUTO_INCREMENT`；`model/task.go` 的 `Task.ID` 当前仍含 `gorm:"primary_key;AUTO_INCREMENT"` | 冲突：本次只记录现状，未修改 schema；需要在单独的数据库兼容性任务中核验迁移影响后处理。 |

## 已消解的历史冲突

| 主张 | 当前证据 | 处理 |
| --- | --- | --- |
| `api.tryvalo.com` 与 `new.tryvalo.com` 的当前边缘路径 | 2026-09-01 当前 DNS、响应头以及两台主机 Caddy 配置一致证明：`api.tryvalo.com` 指向 Zgo `64.83.30.150`，再以 `origin-api.tryvalo.com` 的 Host/SNI 固定回源 GreenCloud `173.249.203.66`；`new.tryvalo.com` 直接指向 GreenCloud | 已确认：采用当前现场读回；旧的“两域名都直连 GreenCloud”描述只保留为历史状态，不再作为当前冲突。 |
