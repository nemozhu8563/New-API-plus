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
- 用户、Token、Channel、Log、Task、TopUp 和订阅对象由 `model/` 持久化并通过 service/controller 暴露。

## 接口、页面、集合或模块

- Dashboard/API：`/api`。
- 同步 Relay：`/v1`、`/v1beta`、`/pg`。
- 异步任务：`/mj`、`/suno`、视频与 Kling/Jimeng 路由。
- 主要模块：`router`、`controller`、`service`、`model`、`middleware`、`relay`、`setting`、`common`。

## 验证状态

已确认：2026-08-31 执行 `make test`、`GOWORK=off go vet ./...` 和 `GOWORK=off go build ./...` 均通过。2026-09-01 现场确认生产和测试应用、生产 PostgreSQL/Redis 健康，公开与本机状态接口成功，且生产库近 24 小时存在部分渠道的消费成功记录；未主动验证全部上游模型。

## 已知约束

- 根模块编译依赖已存在的 `web/dist`，CI 会先创建 embed placeholder，生产构建会先构建真实前端。
- 主数据库必须保持 SQLite、MySQL、PostgreSQL 兼容；日志数据库另支持 ClickHouse。
- `Task.ID` 当前 tag 与根 `AGENTS.md` 的主键规则存在冲突，见 `docs/facts/current-system.md`。

## 待确认事项

- 待定：数据库 dialect 兼容矩阵、Redis 降级与多节点行为，以及生产数据库/缓存的完整业务正确性。
- 待定：所有 Relay provider 的端到端能力和生产可用性。
