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
| 运行前端测试 | `web/`、本地/CI | `bun run test` | `web/package.json`、`.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过；79 files、311 tests |
| 根模块静态检查 | 根目录、本地/CI | `GOWORK=off go vet ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：宿主可见边界重跑通过；首次沙箱运行因无权读取 Go build cache 失败 |
| relaykit 静态检查 | `relaykit/`、本地/CI | `GOWORK=off go vet ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 前端类型检查 | `web/`、本地/CI | `bun run typecheck` | `web/package.json`、`.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 根模块构建 | 根目录、本地/CI | `GOWORK=off go build ./...` | `.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| relaykit 独立构建 | `relaykit/`、本地/CI | `GOWORK=off go build ./...` | 根 `AGENTS.md`、`.github/workflows/ci.yml` | 2026-08-31（Asia/Shanghai） | 已确认：通过 |
| 前端构建 | `web/`、本地/CI | `bun run build` | `web/package.json`、`Dockerfile`、release workflows | 2026-08-31（Asia/Shanghai） | 已确认：通过；Rsbuild 产出 `web/dist` |
| 部署 Compose | 根目录、目标环境 | `docker compose up -d` | `README.md`、`docker-compose.yml` | 待定 | 待定：本次未部署 |

## 命令确认规则

- 命令必须来自仓库脚本、持续集成配置、项目文档或官方工具文档。
- 未确认前写“待定”，不要套用某个技术栈的默认命令。
- 只确认来源但未实际运行时，最近运行时间和最近结果仍写“待定”。
- 每次运行后记录适用范围、时间和结果；命令或环境变化后重新验证。
