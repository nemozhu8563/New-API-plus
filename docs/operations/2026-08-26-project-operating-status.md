# new-api 项目代码与发布现状（截至 2026-08-26）

> 文档状态：本地代码交接快照。
>
> 证明边界：本文件只依据本地 Git 历史、工作树和本次实际执行的验证命令。它不证明远端已收到提交，也不证明测试、预发布或生产环境已经部署、切流或完成回读。

## 1. 当前结论

本轮改动已从 `origin/main` 的 `e7d1a14cc` 起按职责拆成五个本地提交。创建本文件前，代码提交后的本地实现头为 `430aff7c8`，分支为 `main`，相对 `origin/main` 超前 5 个提交，工作树无剩余代码改动。

当前可以确认：

- 内容策略、Stripe 当前订阅过滤、前端国际化、Dashboard 订阅剩余额度已经完成本地代码提交和针对性自动化验证。
- `relaykit` 仍可在关闭 workspace 的情况下独立构建。
- i18n 同步报告中 7 个 locale 的 `missingCount`、`extrasCount`、`untranslatedCount` 均为 0。
- Zgo 边缘切换只有候选 Caddy 配置和执行手册；尚未执行任何服务器、DNS、Cloudflare、Caddy、sing-box 或生产配置变更。
- 本轮没有执行 `git push`、部署、数据库迁移、生产流量切换或生产回读。

## 2. 本轮提交

| 提交 | 范围 | 已验证结果 | 发布边界 |
| --- | --- | --- | --- |
| `dfed70b42` | 内置中英文敏感词、热更新 matcher、英文边界匹配、403 `content_policy_violation`、禁止重试、第三方许可证 | `setting`、`service`、`controller`、`model` 测试通过；`relaykit` 独立构建通过 | 只证明本地代码行为，未验证生产配置是否启用内容检查 |
| `3ba312b31` | Stripe 当前订阅只接受 `active`、`trialing`、`past_due`、`unpaid`；历史 invoice 继续保留 | 相关后端测试随上述四个 package 一并通过 | 未用真实 Stripe 账号或生产订单回读 |
| `2cfc6445f` | 前端用户文案、无障碍标签、i18n 检测器及全部 locale | i18n sync 通过；相关 Vitest 通过；前端类型检查与生产构建通过 | 未执行真实浏览器 E2E 或人工逐语言审校 |
| `a3ea507e3` | Dashboard 汇总所有有效订阅的剩余额度，并避免跨用户复用订阅缓存 | 指标与卡片组件测试通过；前端类型检查与生产构建通过 | 未在已部署环境用多个真实账号验证展示 |
| `430aff7c8` | Zgo 边缘全量切换计划、预签和最终 Caddy 候选配置 | staged diff 检查和敏感信息模式扫描通过 | **计划未执行**，不能据此声称已切流或已升级 Caddy |

## 3. 本地验证记录

本轮实际执行并通过：

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

## 4. 已知验证缺口

- 没有运行整个仓库的 Go 和前端全量测试套件；当前证据是与本轮改动对应的定向测试、构建和格式检查。
- 对本轮变更的前端文件运行 `oxlint` 时仍报告 17 个 error 和 3 个 warning；核对零上下文 diff 后，这些位置位于本轮改动行之外，集中在 4 个已有文件，未在本轮顺带清理。
- `setting/data/google_profanity_en.txt` 为第三方上游的精确快照，保留了 43 行上游尾随空格；因此该批 staged diff 的 whitespace 检查只报告这些数据行。运行时解析会 `TrimSpace`，相关测试已通过。
- 没有进行真实 Stripe、浏览器、数据库迁移、服务器、DNS 或生产环境验证。
- 没有完成产品文案和各语言翻译的人工语义审校；同步报告为结构和未翻译检测结果，不等同于人工质量验收。

## 5. 发布与运维状态

| 项目 | 当前状态 |
| --- | --- |
| 本地代码 | 五批提交完成并完成上述定向验证 |
| 远端仓库 | 未推送；`origin/main` 仍停留在 `e7d1a14cc` |
| 测试或预发布部署 | 本轮未执行 |
| 生产部署 | 本轮未执行 |
| 生产回读 | 未执行，因此生产实际版本和行为属于未知 |
| Zgo 边缘切换 | 计划已编写，尚未执行 |

## 6. 后续门槛

1. 推送前复核五个提交的范围和基础设施信息暴露边界，再决定远端交付方式。
2. 发布前补足所需的全量测试、浏览器 E2E、人工文案审校和目标环境验证。
3. 部署后记录实际提交、镜像 digest、迁移结果、入口健康检查和生产回读证据。
4. Zgo 切换必须以单独的明确“开始”指令进入执行阶段，并严格按执行手册的检查点、验收和回滚门槛进行；不能与应用代码发布状态混为一谈。
