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
| 运行前端测试 | `web/`、本地/CI | `bun run test` | `web/package.json`、`.github/workflows/ci.yml` | 2026-09-01（Asia/Shanghai） | 已确认：通过；80 files、314 tests |
| 运行敏感词相关后端测试 | 根目录、本地 | `go test ./setting ./service ./model ./controller -count=1` | 本次变更涉及包及根 `AGENTS.md` 验证规则 | 2026-09-01（Asia/Shanghai） | 已确认：通过 |
| 运行 Stripe 订阅账期相关包测试 | 根目录、本地 | `go test ./model ./controller -count=1` | Stripe 账期变更涉及包及根 `AGENTS.md` 验证规则 | 2026-09-01（Asia/Shanghai） | 已确认：宿主可见边界重跑通过；`model` 与 `controller` 均为 `ok`。沙箱内同命令会因禁止 miniredis/httptest 绑定 loopback 端口而出现环境性失败，不作为代码失败结论 |
| 根模块静态检查 | 根目录、本地/CI | `GOWORK=off go vet ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：宿主可见边界重跑通过；首次沙箱运行因无权读取 Go build cache 失败 |
| relaykit 静态检查 | `relaykit/`、本地/CI | `GOWORK=off go vet ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 前端类型检查 | `web/`、本地/CI | `bun run typecheck` | `web/package.json`、`.github/workflows/ci.yml` | 2026-09-01（Asia/Shanghai） | 已确认：通过 |
| 根模块构建 | 根目录、本地/CI | `GOWORK=off go build ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| relaykit 独立构建 | `relaykit/`、本地/CI | `GOWORK=off go build ./...` | 根 `AGENTS.md`、`.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 前端构建 | `web/`、本地/CI | `bun run build` | `web/package.json`、`Dockerfile`、release workflows | 2026-09-01（Asia/Shanghai） | 已确认：通过；Rsbuild 产出 `web/dist` |
| 重建 GreenCloud 测试应用 | GreenCloud `/srv/new-api-test/compose.yaml`、`new-api-test` | `docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test` | `docs/operations/greencloud-service-migration-sop.md`、既有测试发布记录 | 2026-09-01（Asia/Shanghai） | 已确认：执行成功；仅重建测试应用，最终 `running/healthy`、重启次数 `0`，测试本机与公网 `/api/status` 均为 HTTP `200` |
| 重建 GreenCloud 正式应用 | GreenCloud `/srv/new-api/compose.yaml`、`new-api` | `docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api` | `docs/operations/greencloud-service-migration-sop.md`、正式发布记录 | 2026-09-01（Asia/Shanghai） | 已确认：执行成功；仅重建正式应用，最终 `running/healthy`、重启次数 `0`，PostgreSQL/Redis 未重建，本机及两个正式公网 `/api/status` 均为 HTTP `200` |

## 命令确认规则

- 命令必须来自仓库脚本、持续集成配置、项目文档或官方工具文档。
- 未确认前写“待定”，不要套用某个技术栈的默认命令。
- 只确认来源但未实际运行时，最近运行时间和最近结果仍写“待定”。
- 每次运行后记录适用范围、时间和结果；命令或环境变化后重新验证。
