# Upstream 合并与模块验收

更新时间：2026-09-12（Asia/Shanghai）

## 已完成

- 通过代理 `HTTP(S)_PROXY=http://127.0.0.1:10808` 更新官方远端 `upstream`。
- 官方 `upstream/main` 当前提交为 `385d2dfd1`（safe multi-RP ID passkey support）。
- 本地分支 `codex/upstream-integration-20260911` 已完成合并，提交为 `510a0445e`。
- 冲突文件已逐项处理，未解决冲突为空；本地 Tryvalo、钱包、Stripe、订阅、计费、敏感词、affiliate、OAuth、站点运行时及渠道路由逻辑保留，同时吸收 upstream 的 passkey、插件、定价和渠道能力改动。

## 已验证

- 根模块 `go test ./...`：代码测试通过；一次并行运行中的 OAuth 用例出现状态竞争，单独重跑已通过。
- `model`、`setting/ratio_setting` 定向测试通过。
- `relaykit` 使用 `GOWORK=off` 独立构建与测试通过。
- 前端 `bun run typecheck`、`bun run lint`、`bun run format:check`、`bun run build` 通过；lint 仅保留 warning。
- SQLite、MySQL 8.4、PostgreSQL 16 的既有定向迁移/审计/模型矩阵已通过。

## 保留的不确定项

- 前端全量测试仍有旧断言与 upstream 新 UI 结构不一致，集中于 setup guide、pricing model cards、task price display、model mapping editor；未用回滚产品功能的方式掩盖这些差异。
- 本次是本地代码合并和验证，没有进行生产部署、真实 Stripe 付款、Webhook、退款、争议或权益 E2E。
- 未完成所有数据库迁移场景的完整 fresh/upgrade 双轮矩阵；已有矩阵仅覆盖本轮涉及的定向路径。
- Task 主键上的历史 schema 约束冲突仍待单独数据库兼容性任务处理。
