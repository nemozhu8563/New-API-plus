# 当前系统事实

本文件记录系统现在已知是什么。不要把未来计划写进这里。

`current-system.md` 是事实总览和索引，不承载分类事实正文。写入 `docs/facts/**` 前先读 `docs/facts/AGENTS.md` 和 `docs/facts/STANDARD.md`。

## 状态

最新 Stripe 发布已确认（2026-09-08 10:25:42，Asia/Shanghai）：正式应用升级为 `3186b5c8d`，充值及一次性套餐 Checkout 显式退出 Managed Payments。新建 CNY 20 充值及 CNY 259 Standard Checkout 均已由 Stripe API 与页面确认微信支付可用，未进行付款。测试保留上一镜像，Stripe 全局设置及应用环境变量未变。当前部署见 `docs/facts/deployment.md`，支付与税务边界见 `docs/facts/integrations.md`，执行记录见 `docs/operations/2026-09-08-stripe-managed-payments-opt-out-release.md`；下列早期发布记录的同镜像与微信待定结论已由本条更新。

已确认：当前仓库由根 Go API 网关服务、独立 Go 模块 `relaykit/` 和 React 前端 `web/` 三个稳定范围组成。根服务将 `web/dist` 嵌入可执行文件，也可以通过 `FRONTEND_BASE_URL` 将未匹配请求重定向到独立前端。2026-08-31 在以提交 `bdbc07608167` 为基线的当前工作树上完成了根模块与 `relaykit` 测试、静态检查和构建，以及前端类型检查、测试和构建。

已确认（2026-09-07，Asia/Shanghai）：提交 `22a1b1d82ed26a03f4bcd76a8a1dd74c3332fc48` 已发布到 GreenCloud 测试和正式应用。测试 `new-api-test` 与正式 `new-api` 均运行镜像 ID `sha256:44fe064881dc38b72405976d72e8646fb1ff0ed6ce62ba909e253be4c5f388f6`，均为 `running/healthy`、重启次数 `0`。Sandbox 对外套餐 `2/3/4` 与 Live 对外套餐 `1/2/3` 已分别绑定新的 one-time Price，公开套餐接口均标记 Stripe Checkout 可用；正式 PostgreSQL 和 Redis 未重建。完整部署事实见 `docs/facts/deployment.md`，Stripe 集成边界见 `docs/facts/integrations.md`，执行和回滚边界见 `docs/operations/2026-09-07-stripe-one-time-cutover.md`。

已确认（2026-09-08，Asia/Shanghai）：GreenCloud 正式基础 Compose 已随提交 `7eee48288` 修复，未启用的 `cpacodexkeeper` 改为显式 keeper overlay，常规应用发布不再因 keeper 镜像变量阻塞。测试与正式基础 Compose 均通过现场配置校验，两个应用保持同一已验收镜像、`running/healthy` 和重启次数 `0`；三个公网状态入口和两环境公开套餐 API 均已重新回读。该结果不替代真实 Stripe 付款闭环验收。

已确认（2026-09-08 09:35:58～09:40，Asia/Shanghai）：提交 `369141e27` 已发布到 GreenCloud 测试和正式应用，两个应用运行同一不可变镜像 ID `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b`，均为 `running/healthy`、重启次数 `0`。订阅 Checkout 的 PMC 环境变量在测试保持不存在、在正式为已配置但不记录标识；因此测试继续使用 Stripe 默认动态支付方式，正式会对新订阅 Checkout 传入账户本地 PMC。正式 PostgreSQL、Redis 未重建；测试、`api.tryvalo.com`、`new.tryvalo.com` 的状态接口均为 HTTP `200`。真实新 Checkout 的微信展示以及支付、Webhook、权益、退款和争议闭环仍待定，详见 `docs/operations/2026-09-08-stripe-payment-method-configuration-release.md`。

已确认（2026-09-01，Asia/Shanghai）：`test.tryvalo.com` 已完成 Stripe Sandbox Standard `CNY 259/月` 首购、账期写入和账单日期显示 E2E。最新订单的账期与 invoice、唯一 settlement 及唯一 active 权益一致，290 Credits 权益已生效且已用额度为 `0`；历史订单没有做字段回填，但账单 API 已从最新已付 settlement 恢复账期，钱包页显示有效下次账单日期。详细对象状态、Automatic Tax 边界和发布证据由 `docs/facts/integrations.md` 与 `docs/operations/2026-09-01-stripe-subscription-period-test-deployment.md` 承载。该结果不确认续费、退款或争议。

已确认（2026-09-01，代码及当时 GreenCloud 测试与正式发布）：提示词敏感词已从单一硬拦截表改为高风险硬拦截、NSFW 硬拦截和仅审计放行三层策略，默认 2,094 个有效来源词被互斥且完整地划分为 `475 + 548 + 1,071` 条。改动已提交为 `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`，当时测试与正式环境运行相同镜像 ID。真实测试接口确认 `成人色情` 与 `炸弹制作` 分别按 NSFW 和高风险策略返回 `403 content_policy_violation`，`淫威` 记录 audit 后越过本地策略；普通请求和 audit 请求随后均因测试渠道上游凭据无效返回 `401 Invalid API key`，因此允许路径成功生成仍为待定。正式库发布前存在的旧 `SensitiveWords` 覆盖已备份后删除，三项敏感词 option 均无持久化行；生产业务请求 E2E 未执行。

已确认（2026-09-03 09:44:47，Asia/Shanghai；Bing 回读同日）：根 Go 服务的站点运行时配置、GA4/Clarity consent gate、`/robots.txt` 与 `/sitemap.xml` 已发布到 GreenCloud 正式 `new-api`。该次发布使用镜像 `new-api:new-api-release-20260903T094447Z-e40d88d1535`，应用为 `running/healthy`、重启次数 `0`；PostgreSQL 与 Redis 未重建。`https://tryvalo.com/` 已运行 canonical 与公开 telemetry payload，初始 analytics consent 为拒绝，未同意时没有 GA4/Clarity 远程资源。GA4 已有精确 Tryvalo Web stream（`https://tryvalo.com`、Measurement ID `G-T2LD0R73QD`），但 Realtime 当前没有可用数据；Clarity 已有精确 Tryvalo project（`ycgor9smow`），当前 provider 页面仍在安装引导，未读到 Dashboard/录制数据。`sc-domain:tryvalo.com` 已由当前 Google 账号以 Owner 身份验证，`https://tryvalo.com/sitemap.xml` 已在 GSC 读回为成功（4 URL）。Bing 的精确站点 `https://tryvalo.com/` 已通过 GSC Import 导入并从 provider 页面读回；同一 sitemap 已提交一次，provider 原始状态为 `Submitted / Processing`。该状态只证明 Bing 已接收并处理中，不确认抓取或收录。正式发布与 provider 回读细节见 `docs/operations/2026-09-03-tryvalo-telemetry-search-production-release.md`。

## 事实文件索引

| 事实文件 | 状态 | 用途 |
| --- | --- | --- |
| `docs/facts/architecture.md` | 已确认 | 技术栈、模块边界、运行方式、共享契约位置。 |
| `docs/facts/product-domain.md` | 已确认 | 跨范围共同成立的业务对象、业务规则、状态语义和业务不变量。 |
| `docs/facts/ui-style.md` | 已确认 | 全局界面风格、交互原则、视觉约束和组件库使用边界；浏览器视觉验收仍为待定。 |
| `docs/facts/integrations.md` | 已确认 | 一次性 Checkout、内部取消、订阅 Checkout PMC 选择和 GA4 页面上下文契约；Tryvalo Sandbox/Live 三档 one-time Price 已分别绑定并随应用发布。旧版 Sandbox 交易证据不能替代新版一次性套餐 E2E；退款、争议及 Live Tax 保持待定。 |
| `docs/facts/deployment.md` | 已确认 | 仓库内部署、数据库和迁移事实，以及 2026-09-08 GreenCloud 测试、正式、公网和配置备份快照；备份恢复与回滚演练仍未闭合。 |
| `docs/facts/verified-commands.md` | 已确认 | 项目命令来源及最近本地验证结果；2026-09-07 一次性套餐发布的应用重建命令和检查边界。 |

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
- 待定：Stripe Sandbox 一次性套餐切换后的真实付款、回调、权益、退款和争议 E2E；已有旧版 recurring 首购证据不能替代。
- 待定：Stripe Live 一次性套餐的真实充值、套餐付款、签名回调、结算、退款和争议闭环；自动续费不属于当前本地实现。
- 待定：Stripe Live Automatic Tax、有效税务注册和申报准备度；Sandbox 本轮 invoice 税额为 `0` 且原因为 `product_exempt`，不能据此确认真实计税交易或 Live 税务状态。
- 待定：2026-09-07 测试和正式发布均保留已校验备份目录，但生产数据库完整恢复演练、Redis/应用数据与日志 volume 备份、监控告警和可执行全栈回滚流程仍未闭合。
- 待定：用户尚未在生产页主动允许 analytics，因此 GA4/Clarity 的 production transport 尚无证据；GA4 Realtime 当前无数据，Clarity Dashboard/live users 与录制仍未可读回。Bing sitemap 当前为 `Submitted / Processing`，Bing 的抓取和页面收录同样未确认；IndexNow 本次未请求。发布与 sitemap receipt/processing 不替代这些 provider 结果。

## 最近事实刷新

| 刷新范围 | 日期 | 依据 | 结果 |
| --- | --- | --- | --- |
| Stripe Managed Payments opt-out 正式发布 | 2026-09-08 10:25～10:31（Asia/Shanghai） | `3186b5c8d`、本地测试/vet/镜像、GreenCloud 容器与配置摘要、两笔新 Live Session API 和页面 | 已确认：仅正式应用升级，充值及 Standard 微信选项出现，两笔均未付款；退出 Managed Payments 后两笔自动税务为关闭，支付与税务闭环不由本次验证证明。 |
| 全部 Facts 与三个范围文件 | 2026-08-31（Asia/Shanghai） | 当前代码、测试、配置、`makefile`、GitHub Actions、Docker 文件和本次命令输出 | 已确认：建立当前事实索引；未能由仓库和本地运行证明的外部状态保持待定。 |
| 既有文档可复用事实 | 2026-09-01（Asia/Shanghai） | `docs/authentication.md`、渠道/计费 solutions、运维状态记录，并以当前代码和定向测试交叉复核 | 已确认：纳入稳定实现契约和带日期的远端快照；旧流程、计划、环境实例值及未复核结论未纳入。 |
| GreenCloud、Zgo、公网、生产聚合和 Stripe 测试边界 | 2026-09-01 09:21～09:33（Asia/Shanghai） | 当前 DNS/HTTP 响应头；GreenCloud 与 Zgo SSH 只读回读；Docker、PostgreSQL 聚合和既有 Stripe 验收记录 | 已确认：当前应用与依赖健康、生产边缘路径和部分真实上游活动；Stripe Sandbox 未完成的生命周期继续保持待定。 |
| Stripe Sandbox 首次月付订阅与账期修复 | 2026-09-01（Asia/Shanghai） | 两轮真实 Hosted Checkout、Stripe Sandbox Checkout/subscription/invoice 只读回读、GreenCloud 测试库、定向回归测试及钱包页回读 | 已确认：Standard `CNY 259/月` 首购、真实 `invoice.paid`、290 Credits 权益和持久化闭环成功；新订单账期与 invoice/settlement/权益一致，历史订单由 settlement 恢复账单日期。续费、退款和争议仍待定。 |
| 敏感词分级策略与 GreenCloud 测试发布 | 2026-09-01 15:06～15:27（Asia/Shanghai） | 精确提交与不可变镜像、三类默认词表、本地 backend/frontend 检查、GreenCloud Docker/PostgreSQL 回读、四组真实测试接口请求 | 已确认：高风险与 NSFW 本地阻断、仅审计放行和测试部署；允许路径因上游 `401 Invalid API key` 未取得成功模型响应。当时正式发布与 option 处理尚未执行，后续结果见下一行。 |
| 敏感词分级策略 GreenCloud 正式发布 | 2026-09-01 21:28～21:40（Asia/Shanghai） | 与测试相同的不可变镜像 ID、GreenCloud Docker/PostgreSQL 回读、备份校验、本机与两个正式公网入口状态接口 | 已确认：正式应用健康、三项持久化 option 均为 0 行、镜像内三层词表生效，PostgreSQL/Redis 未重建；生产业务请求 E2E 未执行。 |
| 站点 telemetry、SEO 路由与本地验证 | 2026-09-03（Asia/Shanghai） | 当前 Go/React 代码、路由和嵌入资源测试；`bun run test`、`bun run typecheck`、`bun run lint`、`bun run format:check`、`bun run build`、`GOWORK=off go test ./...`、`GOWORK=off go vet ./...`、`GOWORK=off go build ./...` | 已确认：本地 GA4/Clarity consent/origin gate、业务事件、robots/sitemap 及首页运行时注入实现通过验证；该项不涵盖正式发布或外部 provider 回读，后续结果见下一行。 |
| Tryvalo telemetry/search 正式发布与 provider 回读 | 2026-09-03 09:44～当前（Asia/Shanghai） | GreenCloud Docker/配置摘要、正式公网与 Chrome Network；GA4、Clarity、GSC、Bing Webmaster 的当前 provider UI | 已确认：正式站点已提供 XML sitemap 与 consent-aware telemetry；GA4 精确 stream 和 Clarity 精确 project 已复用，GA4 Realtime 无数据、Clarity 数据面仍在安装引导；GSC Owner 已成功接收 4 URL sitemap。Bing 已通过 GSC Import 读回精确站点，并已一次提交同一 sitemap，provider 原始状态为 `Submitted / Processing`。Bing crawl/indexing 与 GA4/Clarity 数据面继续保持待定；IndexNow 未请求。 |
| Stripe 一次性套餐、内部取消与 GA4 本地修复及 Price 预配置 | 2026-09-07（Asia/Shanghai，发布前） | 当前代码和回归测试；相关 Go 包测试/vet、根模块构建、前端类型检查/全量测试/构建；Tryvalo Sandbox/Live Stripe API 创建与回读 | 已确认：Checkout 仅允许 one-time，订单快照发放、管理员取消及 GA4 URL 清洗通过本地验证；两环境六个新 Price 已准备。该行是发布前的本地与 provider 证据，后续实际切换见下一行。 |
| Stripe 一次性套餐测试与正式切换 | 2026-09-07（Asia/Shanghai） | 提交和不可变镜像、GreenCloud Docker/PostgreSQL/Redis 只读回读、套餐 API、直接公网 HTTPS、备份 `SHA256SUMS` | 已确认：测试 `2/3/4` 与正式 `1/2/3` 的套餐 Price 映射均已切换，两个应用健康且使用同一镜像 ID；正式仅重建应用，PostgreSQL/Redis 未重建。旧 Price、Product 默认 Price、Webhook、Tax 和支付方式均未改变；真实 Checkout、付款、Webhook、权益、退款和争议 E2E 仍未执行。 |
| GreenCloud Compose 配置修复与测试/正式复核 | 2026-09-08（Asia/Shanghai） | 提交 `7eee48288`、GreenCloud Compose/Docker 只读回读、本机和公网状态接口、公开套餐 API、发布前 Compose 备份 | 已确认：基础 Compose 与可选 keeper overlay 分离，测试与正式基础配置均可校验；应用容器未重建且继续健康。三个公网入口及测试/正式套餐 API 已重新回读；真实 Stripe 交易闭环保持待定。 |
| Stripe 订阅 Checkout PMC 测试与正式发布 | 2026-09-08 09:35:58～09:40（Asia/Shanghai） | 提交 `369141e27`、本地 linux/amd64 镜像及哈希、GreenCloud Compose/Docker/运行时变量存在性回读、备份 `SHA256SUMS`、本机和公网状态接口、公开套餐 API | 已确认：测试运行时未配置 PMC，正式运行时已配置非空 Live PMC，两个应用均使用同一新镜像并保持 `running/healthy`、重启次数 `0`；正式 PostgreSQL/Redis 未重建。新 Checkout 的微信展示与支付闭环没有在本次发布中创建或验证。 |

## 待解决事实冲突

| 主张 | 冲突证据 | 当前处理 |
| --- | --- | --- |
| Task 主键是否遵循项目数据库约束 | 根 `AGENTS.md` 要求让 GORM 处理主键且不得直接使用 `AUTO_INCREMENT`；`model/task.go` 的 `Task.ID` 当前仍含 `gorm:"primary_key;AUTO_INCREMENT"` | 冲突：本次只记录现状，未修改 schema；需要在单独的数据库兼容性任务中核验迁移影响后处理。 |

## 已消解的历史冲突

| 主张 | 当前证据 | 处理 |
| --- | --- | --- |
| `api.tryvalo.com` 与 `new.tryvalo.com` 的当前边缘路径 | 2026-09-01 当前 DNS、响应头以及两台主机 Caddy 配置一致证明：`api.tryvalo.com` 指向 Zgo `64.83.30.150`，再以 `origin-api.tryvalo.com` 的 Host/SNI 固定回源 GreenCloud `173.249.203.66`；`new.tryvalo.com` 直接指向 GreenCloud | 已确认：采用当前现场读回；旧的“两域名都直连 GreenCloud”描述只保留为历史状态，不再作为当前冲突。 |
