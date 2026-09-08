# 已确认命令

本文件记录已确认命令、命令来源、适用范围、最近验证结果和待确认命令。未知命令必须保持“待定”。

## 命令表

| 用途 | 适用目录/服务/环境 | 命令 | 确认来源 | 最近运行时间 | 最近结果 |
| --- | --- | --- | --- | --- | --- |
| 安装前端依赖 | `web/`、本地/CI | `bun install --frozen-lockfile` | `.github/workflows/ci.yml`、`makefile`、`Dockerfile` | 待定 | 待定：本次复用已有 `web/node_modules`，未运行安装 |
| 启动本地后端依赖 | 根目录、本地 | `make dev-api` | `makefile`、`docker-compose.dev.yml` | 待定 | 待定：本次未启动容器 |
| 启动本地前端 | 根目录、本地 | `make dev-web` | `makefile` | 待定 | 待定：本次未启动开发服务器 |
| 运行全部检查 | 全仓 | 待定 | 当前没有单一仓库脚本覆盖 CI 的全部 backend/frontend 检查 | 待定 | 待定 |
| 运行根模块与 relaykit 测试 | 根目录、本地/CI | `make test` | `makefile`、`.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过；root packages 与 `relaykit` 独立 tests 全部通过，部分 Go 结果来自缓存 |
| 运行前端测试 | `web/`、本地/CI | `bun run test`（本次附加 `--reporter=dot`） | `web/package.json`、`.github/workflows/ci.yml` | 2026-09-07（Asia/Shanghai） | 已确认：通过；81 files、322 tests，含套餐文案与后台取消交互 |
| 运行根模块测试 | 根目录、本地 | `GOWORK=off go test ./...` | 根 `AGENTS.md`、本次 telemetry 变更涉及的 Go 路由和嵌入资源测试 | 2026-09-03（Asia/Shanghai） | 已确认：通过；包含首页运行时注入、SEO 路由及其他根模块包测试 |
| 运行敏感词相关后端测试 | 根目录、本地 | `go test ./setting ./service ./model ./controller -count=1` | 本次变更涉及包及根 `AGENTS.md` 验证规则 | 2026-09-01（Asia/Shanghai） | 已确认：通过 |
| 运行 Stripe 订阅账期相关包测试 | 根目录、本地 | `go test ./model ./controller -count=1` | Stripe 账期变更涉及包及根 `AGENTS.md` 验证规则 | 2026-09-01（Asia/Shanghai） | 已确认：宿主可见边界重跑通过；`model` 与 `controller` 均为 `ok`。沙箱内同命令会因禁止 miniredis/httptest 绑定 loopback 端口而出现环境性失败，不作为代码失败结论 |
| 运行一次性付款及关联包测试 | 根目录、本地 | `GOWORK=off go test ./controller ./model ./router ./common . -count=1` | 根 `AGENTS.md` 与本次变更涉及包 | 2026-09-07（Asia/Shanghai） | 已确认：宿主可见边界通过，包含 recurring 拒绝、订单快照、内部取消、退款与争议回归；不是远端交易验收 |
| 一次性付款及关联包静态检查 | 根目录、本地 | `GOWORK=off go vet ./controller ./model ./router ./common .` | 根 `AGENTS.md` 与本次变更涉及包 | 2026-09-07（Asia/Shanghai） | 已确认：通过 |
| 根模块静态检查 | 根目录、本地/CI | `GOWORK=off go vet ./...` | `.github/workflows/ci.yml` | 2026-09-03（Asia/Shanghai） | 已确认：通过 |
| relaykit 静态检查 | `relaykit/`、本地/CI | `GOWORK=off go vet ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 前端类型检查 | `web/`、本地/CI | `bun run typecheck` | `web/package.json`、`.github/workflows/ci.yml` | 2026-09-07（Asia/Shanghai） | 已确认：通过 |
| 前端 lint | `web/`、本地 | `bun run lint` | `web/package.json`、`web/AGENTS.md` | 2026-09-07（Asia/Shanghai） | 已确认：通过；只有既有无关 warning |
| 前端格式检查 | `web/`、本地 | `bun run format:check` | `web/package.json`、`web/AGENTS.md` | 2026-09-07（Asia/Shanghai） | 已确认：全量检查未通过，唯一问题为原有 `src/features/dashboard/components/overview/__tests__/summary-cards.test.tsx`；本次修改文件已精确格式化 |
| 根模块构建 | 根目录、本地/CI | `GOWORK=off go build ./...` | `.github/workflows/ci.yml` | 2026-09-07（Asia/Shanghai） | 已确认：通过 |
| relaykit 独立构建 | `relaykit/`、本地/CI | `GOWORK=off go build ./...` | 根 `AGENTS.md`、`.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 前端构建 | `web/`、本地/CI | `bun run build` | `web/package.json`、`Dockerfile`、release workflows | 2026-09-07（Asia/Shanghai） | 已确认：通过；Rsbuild 产出 `web/dist` |
| 验证 GreenCloud 正式基础 Compose | GreenCloud `/srv/new-api/compose.yaml` | `docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml config -q` | `ops/greencloud/compose.yaml`、GreenCloud Compose 发布记录 | 2026-09-08（Asia/Shanghai） | 已确认：通过；基础发布不再解析可选 keeper 服务 |
| 验证 GreenCloud 可选 keeper overlay | GreenCloud `/srv/new-api/compose.yaml`、`/srv/new-api/compose.keeper.yaml` | `docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml -f /srv/new-api/compose.keeper.yaml --profile keeper config -q` | `ops/greencloud/compose.keeper.yaml`、GreenCloud Compose 发布记录 | 2026-09-08（Asia/Shanghai） | 已确认：通过；只渲染配置，不启动 keeper |
| 重建 GreenCloud 测试应用 | GreenCloud `/srv/new-api-test/compose.yaml`、`new-api-test` | `docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test` | `docs/operations/greencloud-service-migration-sop.md`、2026-09-08 PMC 发布记录 | 2026-09-08（Asia/Shanghai） | 已确认：执行成功；仅重建测试应用为 `new-api:new-api-test-20260908T012416Z-369141e27`，最终 `running/healthy`、重启次数 `0`，运行时不含订阅 Checkout PMC 变量；测试本机、`test.tryvalo.com` 状态接口与本机公开套餐 API 回读正常 |
| 重建 GreenCloud 正式应用 | GreenCloud `/srv/new-api/compose.yaml`、`new-api` | `docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api` | `docs/operations/greencloud-service-migration-sop.md`、2026-09-08 PMC 发布记录 | 2026-09-08（Asia/Shanghai） | 已确认：执行成功；仅重建正式应用为 `new-api:new-api-release-20260908T012416Z-369141e27`，最终 `running/healthy`、重启次数 `0`，运行时 PMC 已配置但未输出；PostgreSQL/Redis 未重建；本机、`api.tryvalo.com`、`new.tryvalo.com` 的 `/api/status` 均为 HTTP `200` |

## 命令确认规则

- 命令必须来自仓库脚本、持续集成配置、项目文档或官方工具文档。
- 未确认前写“待定”，不要套用某个技术栈的默认命令。
- 只确认来源但未实际运行时，最近运行时间和最近结果仍写“待定”。
- 每次运行后记录适用范围、时间和结果；命令或环境变化后重新验证。
