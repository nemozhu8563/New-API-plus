# 架构事实

本文件记录系统结构、技术栈、模块边界、运行方式、共享契约位置和主要数据流。不要写未来计划。

## 技术栈

| 范围 | 技术及版本 | 用途 | 确认来源 | 最近确认 |
| --- | --- | --- | --- | --- |
| 根服务 | Go `1.25.1`、Gin `v1.9.1`、GORM `v1.25.2` | HTTP API、认证、计费、渠道分发、持久化和后台任务 | `go.mod`、`main.go` | 2026-08-31 |
| Relay 转换契约 | Go `1.25.1` 独立 module | 多协议 DTO、请求/响应转换、媒体与 reasoning 映射 | `relaykit/go.mod`、`relaykit/relayconvert/` | 2026-08-31 |
| 前端 | React `^19.2.7`、Rsbuild `^2.1.4`、Base UI `^1.6.0`、Tailwind CSS `^4.3.2`、Bun | 管理后台、用户界面、聊天与计费相关页面 | `web/package.json`、`web/src/` | 2026-08-31 |
| 持久化与缓存 | SQLite、MySQL、PostgreSQL；日志可单独使用 ClickHouse；Redis 或内存缓存 | 主数据、日志、配额和渠道缓存 | `model/main.go`、`common/redis.go`、`go.mod` | 2026-08-31 |

## 模块边界

- `router/` 组装 `/api`、Dashboard、Relay 和 Video 路由；`controller/` 处理 HTTP 请求；`service/` 承载业务逻辑；`model/` 负责 GORM 持久化、迁移和缓存一致性。
- `relay/` 负责认证后的渠道分发、协议适配、上游请求和异步任务；具体供应商适配位于 `relay/channel/`。
- `setting/` 管理定价、模型、运营、系统和性能配置；`common/` 提供 JSON、数据库类型、Redis、限流和配额数学等共享能力。
- `relaykit/` 是独立 Go module；它不导入根 module 包，根服务可以依赖它的 DTO、类型和转换能力。
- `web/` 是独立前端范围；生产构建产物为 `web/dist`，由根程序通过 `go:embed` 嵌入。

## 代码目录

- HTTP 与业务主链：`router/`、`controller/`、`service/`、`model/`、`middleware/`。
- Relay 主链：`relay/`、`relay/channel/`、`relay/helper/`、`relay/common/`。
- 共享契约与配置：`dto/`、`types/`、`constant/`、`setting/`、`common/`、`oauth/`、`pkg/`。
- 独立转换模块：`relaykit/dto/`、`relaykit/types/`、`relaykit/relayconvert/`、`relaykit/reasonmap/`。
- 前端：`web/src/routes/`、`web/src/features/`、`web/src/components/`、`web/src/lib/`、`web/src/stores/`、`web/src/i18n/`。

## 运行方式

- `main.main` 先调用 `InitResources`，再初始化缓存与动态配置，启动授权同步、配额看板、凭据刷新、订阅重置、实例上报和系统任务，最后组装 Gin 路由并监听 `PORT`。
- `router.SetRouter` 依次注册 API、Dashboard、Relay 和 Video 路由。`FRONTEND_BASE_URL` 为空时提供嵌入前端；非主节点且该变量存在时，将未匹配请求重定向到外部前端；主节点忽略该变量并提供内置前端。
- `Dockerfile` 使用 Bun 构建 `web/dist`，再构建 Go 可执行文件，最终镜像以 `/new-api` 为入口并暴露 3000 端口。
- 本地开发入口由 `makefile` 提供：后端依赖使用 `docker-compose.dev.yml`，前端由 Rsbuild 默认运行在 5173 并代理到 3000。

## 共享契约

- 根服务公共请求/响应结构位于 `dto/` 和 `types/`，渠道与上下文常量位于 `constant/`。
- 跨上游协议转换契约位于 `relaykit/dto/`、`relaykit/types/` 和 `relaykit/relayconvert/`。
- 后端国际化位于 `i18n/`；前端国际化位于 `web/src/i18n/`，两者独立维护。
- 分层计费表达式的当前实现与边界由 `pkg/billingexpr/`、`relay/helper/price.go` 和 `service/tiered_settle.go` 共同承载。

## 主要数据流

1. Dashboard 请求：浏览器 → `/api` → API 中间件 → `controller/` → `service/`/`model/` → 主数据库或缓存。
2. 同步 Relay 请求：客户端 Token → `/v1` 或 `/v1beta` → 性能检查、Token 鉴权、模型限流与渠道分发 → `controller.Relay` → `relay/` 适配器 → 上游 → 结算与消费日志。
3. 异步任务：任务提交路由 → `model.Task` → 渠道任务适配器 → 周期轮询 → 状态、计费和结果持久化。
4. 前端交付：Bun/Rsbuild → `web/dist` → Go `embed.FS` → Web 路由；配置外部前端时由 NoRoute 重定向代替。

## 渠道选择链路

- 候选渠道先按 group、model、enabled 和可选 tag 过滤，再按当前 retry 对应的最高可用 priority 选层，最后在该 priority 内按 weight 随机选择；数据库路径和内存缓存路径都保持这一顺序。
- 已解析 route tag 且存在候选时，先只在 tagged pool 中选择。显式 override 产生 strict tag，候选缺失或选择失败都不回退；自动推导的非 strict tag 只有在 tagged 选择没有结果时才回退一般池。实现与测试位于 `service/channel_select.go`、`model/ability.go`、`model/channel_cache.go` 和 `service/channel_select_test.go`。

## 已知约束

- 根 module 当前依赖 `relaykit`，但 `relaykit` 必须保持对根 module 的单向独立，并用 `GOWORK=off` 独立构建。
- 主数据库代码面向 SQLite、MySQL 和 PostgreSQL；日志数据库可复用主库或单独使用 ClickHouse。实际跨数据库迁移矩阵本次未运行。
- 当前代码中的 JSON 编解码必须经 `common/json.go` 封装；该约束来自根 `AGENTS.md`，本次未对全仓逐调用审计。
- Go module directive 为 `1.25.1`，容器构建器当前为 Go `1.26.1`；CI 从 `go.mod` 读取版本。三者是不同构建上下文，不代表统一锁定的单一工具链版本。
- `Task.ID` 的 `AUTO_INCREMENT` tag 与根 `AGENTS.md` 数据库约束存在冲突，详见 `docs/facts/current-system.md`。

## 待确认事项

- 待定：所有渠道、模型与 Relay format 的完整能力矩阵。
- 待定：`gorm:"type:json"` 字段在 SQLite、MySQL 和 PostgreSQL 的实际迁移兼容结果。
- 待定：生产环境的主从/多 master、代理、数据库、缓存和前端分离拓扑。
