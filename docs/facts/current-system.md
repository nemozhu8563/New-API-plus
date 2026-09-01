# 当前系统事实

本文件记录系统现在已知是什么。不要把未来计划写进这里。

`current-system.md` 是事实总览和索引，不承载分类事实正文。写入 `docs/facts/**` 前先读 `docs/facts/AGENTS.md` 和 `docs/facts/STANDARD.md`。

## 状态

已确认：当前仓库由根 Go API 网关服务、独立 Go 模块 `relaykit/` 和 React 前端 `web/` 三个稳定范围组成。根服务将 `web/dist` 嵌入可执行文件，也可以通过 `FRONTEND_BASE_URL` 将未匹配请求重定向到独立前端。2026-08-31 在以提交 `bdbc07608167` 为基线的当前工作树上完成了根模块与 `relaykit` 测试、静态检查和构建，以及前端类型检查、测试和构建。

已确认（2026-09-01 09:27～09:33，Asia/Shanghai）：GreenCloud 生产与测试应用仍运行提交 `f96bf33b80dfeca9b025a94651fb68db492dc8a7` 的同一镜像，两个应用容器、生产 PostgreSQL 和 Redis 均为 `running/healthy`、重启次数 `0`，本机与三个公开 `/api/status` 入口均返回 HTTP `200`。生产库近 24 小时存在来自两个不同渠道的消费成功记录，所查询条件下没有渠道错误记录；这只证明部分真实上游链路近期成功，不证明所有渠道和模型可用。测试库当前配置的渠道均为禁用状态。

## 事实文件索引

| 事实文件 | 状态 | 用途 |
| --- | --- | --- |
| `docs/facts/architecture.md` | 已确认 | 技术栈、模块边界、运行方式、共享契约位置。 |
| `docs/facts/product-domain.md` | 已确认 | 跨范围共同成立的业务对象、业务规则、状态语义和业务不变量。 |
| `docs/facts/ui-style.md` | 已确认 | 全局界面风格、交互原则、视觉约束和组件库使用边界；浏览器视觉验收仍为待定。 |
| `docs/facts/integrations.md` | 已确认 | 外部服务代码边界、协议兼容、回调入口、部分真实上游活动及带日期的 Stripe/邮件快照；未执行的端到端场景保持待定。 |
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
- 待定：Stripe Sandbox 月度订阅 Checkout、真实 `invoice.paid` 权益入账、续费、退款和争议；单次充值闭环已经完成，不能代表这些场景。
- 待定：Stripe Live 真实充值、订阅、签名回调、结算、续费、退款和争议闭环。
- 待定：生产备份、恢复演练、监控告警和可执行回滚流程。

## 最近事实刷新

| 刷新范围 | 日期 | 依据 | 结果 |
| --- | --- | --- | --- |
| 全部 Facts 与三个范围文件 | 2026-08-31（Asia/Shanghai） | 当前代码、测试、配置、`makefile`、GitHub Actions、Docker 文件和本次命令输出 | 已确认：建立当前事实索引；未能由仓库和本地运行证明的外部状态保持待定。 |
| 既有文档可复用事实 | 2026-09-01（Asia/Shanghai） | `docs/authentication.md`、渠道/计费 solutions、运维状态记录，并以当前代码和定向测试交叉复核 | 已确认：纳入稳定实现契约和带日期的远端快照；旧流程、计划、环境实例值及未复核结论未纳入。 |
| GreenCloud、Zgo、公网、生产聚合和 Stripe 测试边界 | 2026-09-01 09:21～09:33（Asia/Shanghai） | 当前 DNS/HTTP 响应头；GreenCloud 与 Zgo SSH 只读回读；Docker、PostgreSQL 聚合和既有 Stripe 验收记录 | 已确认：当前应用与依赖健康、生产边缘路径和部分真实上游活动；Stripe Sandbox 未完成的生命周期继续保持待定。 |

## 待解决事实冲突

| 主张 | 冲突证据 | 当前处理 |
| --- | --- | --- |
| Task 主键是否遵循项目数据库约束 | 根 `AGENTS.md` 要求让 GORM 处理主键且不得直接使用 `AUTO_INCREMENT`；`model/task.go` 的 `Task.ID` 当前仍含 `gorm:"primary_key;AUTO_INCREMENT"` | 冲突：本次只记录现状，未修改 schema；需要在单独的数据库兼容性任务中核验迁移影响后处理。 |

## 已消解的历史冲突

| 主张 | 当前证据 | 处理 |
| --- | --- | --- |
| `api.tryvalo.com` 与 `new.tryvalo.com` 的当前边缘路径 | 2026-09-01 当前 DNS、响应头以及两台主机 Caddy 配置一致证明：`api.tryvalo.com` 指向 Zgo `64.83.30.150`，再以 `origin-api.tryvalo.com` 的 Host/SNI 固定回源 GreenCloud `173.249.203.66`；`new.tryvalo.com` 直接指向 GreenCloud | 已确认：采用当前现场读回；旧的“两域名都直连 GreenCloud”描述只保留为历史状态，不再作为当前冲突。 |
