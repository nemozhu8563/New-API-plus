# new-api 项目代码与发布现状（截至 2026-08-26）

> 文档状态：代码交付与 GreenCloud 测试发布交接快照。
>
> 证明边界：本文件依据本地 Git、远端推送结果、不可变镜像构建记录，以及 GreenCloud 现场回读。它证明提交 `9c0dde868` 已发布到独立测试实例；不证明这些改动已经发布到生产，也不证明真实 Stripe、付费模型、管理员流程或全部用户路径已经完成 E2E。

## 1. 当前结论

本轮改动从 `origin/main` 的 `e7d1a14cc` 起按职责拆成五个实现/运维提交，再以 `9c0dde868` 记录发布前现状。上述六个提交均已推送到 `origin/main`，代码交付点为 `9c0dde868f6fa3ba80db0252edcf684129f6f35f`。

当前可以确认：

- 内容策略、Stripe 当前订阅过滤、前端国际化、Dashboard 订阅剩余额度已经完成代码提交和针对性自动化验证。
- `relaykit` 仍可在关闭 workspace 的情况下独立构建。
- i18n 同步报告中 7 个 locale 的 `missingCount`、`extrasCount`、`untranslatedCount` 均为 0。
- 已从精确提交 `9c0dde868` 的隔离归档构建 `linux/amd64` 不可变镜像，并于 2026-08-26 发布到 GreenCloud 独立 `new-api-test` 实例。
- 测试实例持续 `healthy`，本机 `GET /api/status` 和首页均返回 HTTP 200，发布后关键启动错误计数为 0。
- 生产 `new-api` 未重建，发布前后镜像引用、镜像 ID 和健康状态一致；PostgreSQL 与 Redis 均保持 `healthy`。
- 本次应用发布未修改生产容器、DNS、Cloudflare、Caddy、Zgo、CPA、PostgreSQL 或 Redis 的部署配置。

## 2. 本轮提交

| 提交 | 范围 | 已验证结果 | 发布边界 |
| --- | --- | --- | --- |
| `dfed70b42` | 内置中英文敏感词、热更新 matcher、英文边界匹配、403 `content_policy_violation`、禁止重试、第三方许可证 | `setting`、`service`、`controller`、`model` 测试通过；`relaykit` 独立构建通过 | 测试实例已包含；未验证生产配置是否启用内容检查 |
| `3ba312b31` | Stripe 当前订阅只接受 `active`、`trialing`、`past_due`、`unpaid`；历史 invoice 继续保留 | 相关后端测试随上述四个 package 一并通过 | 测试实例已包含；未用真实 Stripe 账号或订单回读 |
| `2cfc6445f` | 前端用户文案、无障碍标签、i18n 检测器及全部 locale | i18n sync 通过；相关 Vitest 通过；前端类型检查与生产构建通过 | 测试实例已包含；未执行真实浏览器 E2E 或人工逐语言审校 |
| `a3ea507e3` | Dashboard 汇总所有有效订阅的剩余额度，并避免跨用户复用订阅缓存 | 指标与卡片组件测试通过；前端类型检查与生产构建通过 | 测试实例已包含；未用多个真实账号验证展示 |
| `430aff7c8` | Zgo 边缘全量切换计划、预签和最终 Caddy 候选配置 | staged diff 检查和敏感信息模式扫描通过 | 该提交只交付计划材料；本次应用测试发布未执行边缘切换 |
| `9c0dde868` | 发布前项目现状文档 | 文档 diff 与提交范围复核通过 | 作为本次不可变测试镜像的精确代码提交 |

## 3. 本地验证记录

本轮代码提交前实际执行并通过：

```text
GOCACHE=/tmp/new-api-go-build go test ./setting ./service ./controller ./model
cd relaykit && GOWORK=off go build ./...
cd web && bun run i18n:sync
cd web && bun run test <9 个本轮新增或修改的测试文件>
cd web && bun run build:check
oxfmt --check <本轮变更的前端 JS/TS/TSX 文件>
git diff --check
```

结果摘要：

- Go：4 个目标 package 全部通过。
- relaykit：独立构建通过。
- Vitest：9 个测试文件、24 个测试全部通过。
- 前端：`tsgo -b` 与 `rsbuild build` 均通过。
- i18n：`en`、`fr`、`ja`、`ru`、`vi`、`zh-TW`、`zh` 均无缺失、额外或未翻译项。
- 格式：本轮变更的前端文件通过 `oxfmt --check`。
- 每批提交前均按 staged diff 扫描常见密钥、令牌、密码和私钥模式，未发现疑似凭据。
- 测试发布镜像的 Docker 多阶段构建通过，包含 Bun 前端生产构建与 Go 二进制构建。

## 4. GreenCloud 测试发布记录

发布目标经过现场只读盘点确认：

- GreenCloud 主机：`nemo-Phoenix` / `173.249.203.66`。
- 测试 Compose：`/srv/new-api-test/compose.yaml`，项目和服务均为 `new-api-test`。
- 测试端口：`127.0.0.1:3001 -> 3000`；生产 `new-api` 独立使用 `127.0.0.1:3000`。
- 测试数据与日志：`/srv/new-api-test/data/new-api`、`/srv/new-api-test/logs/new-api`。
- 测试数据库：独立 `newapi_test`；其连接串哈希与生产不同。

不可变发布证据：

| 项目 | 值 |
| --- | --- |
| 发布时间 | `2026-08-26 22:43`（Asia/Shanghai） |
| 提交 | `9c0dde868f6fa3ba80db0252edcf684129f6f35f` |
| Release ID / 镜像 | `new-api-test-20260826T143317Z-9c0dde868` / `new-api:new-api-test-20260826T143317Z-9c0dde868` |
| 平台 | `linux/amd64` |
| 镜像 ID | `sha256:e709317dbd9a19b7b5b49862c299945fe4e700241cb1bbe5fd663ac653227fab` |
| 镜像包 SHA-256 | `c16d657f7c030793bbe4357795b80214337961c1be07844d1d52f002fba2b431` |
| 旧测试镜像 | `new-api:new-api-test-20260826T024237Z-b1376a8fc` |
| Compose 备份 | `/srv/new-api-test/backups/compose.yaml.before-new-api-test-20260826T143317Z-9c0dde868` |
| 测试库备份 | `/srv/new-api-test/backups/pre-new-api-test-20260826T143317Z-9c0dde868.dump` |
| 测试库备份 SHA-256 | `a25578657e757aee41a6f70b15f93d756efb8e32dbeb33ef979c740ecc06f1ed` |

发布动作只加载新镜像、更新测试 Compose 中唯一的镜像引用，并执行：

```text
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --force-recreate new-api-test
```

2026-08-26 22:44（Asia/Shanghai）回读结果：

- `new-api-test`：`running`、`healthy`、重启次数 0，运行镜像 ID 与构建镜像一致。
- `GET http://127.0.0.1:3001/api/status`：HTTP 200 且 `success: true`。
- `GET http://127.0.0.1:3001/`：HTTP 200。
- 最近 10 分钟启动日志中的 `panic`、`fatal`、迁移/数据库关键错误计数：0。
- 生产 `new-api`：仍为 `new-api:new-api-release-20260821T083301Z-52055bbf`，镜像 ID 仍为 `sha256:915b85ceef61ef8bb35294d589b6d4a57f07ab49594ea0ba3c071c8b73e0df2d`，状态 `healthy`。
- `new-api-postgres`、`new-api-redis`：均为 `healthy`。

容器级回滚命令：

```bash
cp -p /srv/new-api-test/backups/compose.yaml.before-new-api-test-20260826T143317Z-9c0dde868 /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --force-recreate new-api-test
```

如需恢复测试数据库，应先停止测试写入并另行制定恢复步骤；本次未执行数据库恢复演练。

## 5. 已知验证缺口

- 没有运行整个仓库的 Go 和前端全量测试套件；当前代码证据是与本轮改动对应的定向测试、构建和格式检查。
- 对本轮变更的前端文件运行 `oxlint` 时仍报告 17 个 error 和 3 个 warning；核对零上下文 diff 后，这些位置位于本轮改动行之外，集中在 4 个已有文件，未在本轮顺带清理。
- `setting/data/google_profanity_en.txt` 为第三方上游的精确快照，保留了 43 行上游尾随空格；运行时解析会 `TrimSpace`，相关测试已通过。
- 没有执行真实 Stripe 付款/回调、付费模型请求、管理员登录、多个真实账号 Dashboard 展示或完整浏览器 E2E。
- `/api/status` 当前返回的 `version` 为空，不能单独证明运行提交；本次以不可变镜像 tag、镜像 ID、构建提交和包哈希建立对应关系。
- 测试容器启动和健康检查通过，但没有执行数据库 schema diff、恢复演练或 MySQL 实例验证。
- 没有完成产品文案和各语言翻译的人工语义审校；同步报告为结构和未翻译检测结果，不等同于人工质量验收。

## 6. 发布与运维状态

| 项目 | 当前状态 |
| --- | --- |
| 本地代码 | 五批实现/运维提交及发布前现状提交已完成定向验证 |
| 远端仓库 | 代码交付点 `9c0dde868` 已推送到 `origin/main`；本文件为其后的发布记录更新 |
| 测试部署 | 已完成；GreenCloud `new-api-test` 运行不可变镜像 `new-api-test-20260826T143317Z-9c0dde868` |
| 生产部署 | 本轮未执行；生产容器身份与健康状态已回读为不变 |
| 生产业务回读 | 未执行管理员、Stripe、模型请求或账单 E2E，生产业务行为不能由容器健康替代证明 |
| Zgo / DNS / Cloudflare | 本次应用测试发布未触碰；边缘状态以专门执行手册和现场记录为准 |

发布期间工作树中另有未提交的 `AGENTS.md`、Zgo 执行手册和 `.codex-cutover` 文件状态变化；它们没有进入 `9c0dde868` 的镜像，也不得与本次测试发布混为一批。后续操作应继续保留并单独核对这些改动。

## 7. 后续门槛

1. 在测试实例用批准的测试账号完成管理员登录、多个订阅组合的 Dashboard 展示、内容策略拒绝路径和 Stripe Sandbox 业务 E2E。
2. 如测试需要真实上游模型或长连接/SSE，先明确 token、费用与观察范围，再执行并核对请求日志及单次计费。
3. 正式生产发布必须作为单独动作，以固定提交重新构建/复用已审计镜像，执行生产备份、迁移检查、健康与业务回读，并记录独立回滚窗口。
4. Zgo、DNS、Cloudflare、Caddy 和防火墙变更继续使用各自的执行门槛，不能由本次测试实例成功推导为已完成或可直接变更。
