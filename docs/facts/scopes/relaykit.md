# Relay 转换契约范围事实

## 范围

- 路径：`relaykit/`。
- 技术栈及版本：独立 Go module，module directive `1.25.1`。
- 相关横向事实：`docs/facts/architecture.md`、`docs/facts/product-domain.md`、`docs/facts/verified-commands.md`。

## 当前实现

`relaykit` 提供跨协议 DTO、Relay format 类型、请求与响应转换 registry、媒体处理、reasoning 处理和错误/日志注入点。当前源码导入使用 `github.com/QuantumNous/new-api/relaykit/...` 内部路径，未发现对根 module 包的反向导入。

## 事实来源

`relaykit/go.mod`、`relaykit/dto/`、`relaykit/types/`、`relaykit/relayconvert/`、`relaykit/reasonmap/`、根 `AGENTS.md` 及 2026-08-31 独立测试、vet 和 build 输出。

## 如何承载全局业务规则

该范围只承载协议、DTO 和转换规则。配额、数据库、账号、渠道选择和结算由根服务承载；根服务通过注入 logging/system-error callback 与转换层连接。

## 接口、页面、集合或模块

- `relaykit/dto/`：各协议请求/响应结构。
- `relaykit/types/`：Relay format 与共享类型。
- `relaykit/relayconvert/`：request/response registry、内部格式转换、媒体和 reasoning。
- `relaykit/reasonmap/`：reason 映射。

## 验证状态

已确认：2026-08-31 在 `relaykit/` 执行 `GOWORK=off go test ./...`、`GOWORK=off go vet ./...` 和 `GOWORK=off go build ./...` 均通过。

## 已知约束

- 必须保持 `GOWORK=off` 独立构建，不得依赖根 module 的配置、生成物或包。
- 独立性是单向的：根服务可以导入 `relaykit`，`relaykit` 不能导入根服务。

## 待确认事项

- 待定：该 module 对仓库外消费者是否存在稳定版本化 API；当前证据只覆盖本仓库构建与测试。
- 待定：所有协议组合和上游边界的完整兼容矩阵。
