# new-api 项目代码与发布现状（截至 2026-08-28）

> 文档状态：代码交付与 GreenCloud 测试发布交接快照。
>
> 证明边界：本文件依据本地 Git、远端推送结果、不可变镜像构建记录、GreenCloud 现场回读、测试库只读聚合和浏览器验收。它证明精确提交 `91e861f8c970a0b5c1897eeabaaca17f07fa1613` 已于 2026-08-28 发布到独立测试实例，证明测试库套餐月度迁移、Dashboard API Key 复制修复、全量质量门，以及一笔 CNY 20 Stripe Sandbox 充值 E2E 已完成；OAuth 的上一版发布证据继续保留在下文。正式生产应用与数据库未发布、未迁移，本文也不证明生产 Stripe、Stripe 订阅、付费模型、管理员流程或全部用户路径已经完成 E2E。Creem、Epay、Waffo 不在本次 Stripe 验收范围内。本文件自身是发布后的记录更新，不在该测试镜像内。

## 1. 当前结论

原一轮改动从 `origin/main` 的 `e7d1a14cc` 起按职责拆成五个实现/运维提交，再以 `9c0dde868` 和 `ba5cfebac` 记录发布前、发布后现状。后续订阅、运维、认证文案、GitHub/Google 登录、月度套餐契约、Dashboard API Key 修复和前端质量门修复均已形成审计提交；本次运行记录推送完成后，这些提交均位于 `origin/main`。当前测试部署的精确代码交付点为 `91e861f8c970a0b5c1897eeabaaca17f07fa1613`。

当前可以确认：

- 内容策略、Stripe 当前订阅过滤、前端国际化、Dashboard 订阅剩余额度已经完成代码提交和针对性自动化验证。
- 当前没有安全、完整的订阅升级、按比例计费和权益迁移能力，因此本次不实现升级：用户存在有效订阅时仍可查看所有套餐用于比较，但所有购买入口均禁用，并明确提示暂不支持变更套餐。
- Stripe、Creem、Epay、Waffo Pancake 的订阅下单入口都会在调用支付渠道或写入订单前拒绝已有有效订阅的用户，前后端行为一致。
- 当前订阅、Stripe 账单和发票中的配置套餐名会经过 i18n；截图中的 `Professional` 英文残留已在代码中修复。
- `04aa934d4` 将开发阶段套餐统一为单一 CNY 月度契约：每个已支付 Stripe invoice 创建一个新的账期权益。正式库套餐相关三表在发布前只读计数均为 0，因此没有旧正式套餐需要兼容或迁移；独立测试库原有 4 条套餐已全部规范化为月度契约，6 个迁移字段齐全，无需为了测试数据再删除套餐。
- Dashboard 复制 API Key 曾复制列表接口返回的掩码值，属于确定性功能 Bug；`e15486acc` 已改为按 token ID 调用受保护接口获取完整 Key，并由回归测试保护。
- `91e861f8c` 已恢复全量前端质量门：lint 为 0 error、0 warning，typecheck、76 个文件/309 个 Vitest、生产 build 和 format check 全部通过；全量 Go 测试与 `relaykit` 独立构建也通过。
- Stripe Sandbox 已完成一笔 CNY 20 Hosted Checkout 真实测试支付：真实 webhook 恰好结算一次，本地订单由 pending 变为 success，用户额度由 0 变为 10,000,000 quota units（Wallet 显示 `$20`），且没有重复入账。该结果只证明 Sandbox 充值，不等同于 Stripe 生产支付或 Stripe 订阅验证。
- `relaykit` 仍可在关闭 workspace 的情况下独立构建。
- i18n 同步报告中 7 个 locale 的 `missingCount`、`extrasCount`、`untranslatedCount` 均为 0。
- 上一轮已从精确提交 `00f0f3598` 的隔离归档构建 `linux/amd64` 不可变镜像，并于 2026-08-27 发布到 GreenCloud 独立 `new-api-test` 实例；该镜像后来依次被 `f28cacabdf` 和当前 `91e861f8c` 不可变镜像取代。
- 测试实例持续 `healthy`，本机 `GET /api/status` 和首页均返回 HTTP 200，发布后关键启动错误计数为 0。
- 生产 `new-api` 未重建，发布前后镜像引用、镜像 ID 和健康状态一致；PostgreSQL 与 Redis 均保持 `healthy`。
- 本次应用发布未修改生产容器、DNS、Cloudflare、Caddy、Zgo、CPA、PostgreSQL 或 Redis 的部署配置。
- Zgo 边缘全量切换的实际执行状态已由 `13af0b289` 更新到专门运行手册；本次应用测试发布没有再次执行或改动该切换。
- GitHub 和 OIDC（测试环境用于 Google）登录使用数据库唯一 claim 原子占用外部身份。一个本地账号可以同时绑定多个 provider 下尚未被占用的身份；同一 `provider + subject` 只能归属一个本地账号，同一 provider 的既有绑定不能换成另一个外部身份，也不按邮箱自动合并账号。
- GitHub/OIDC 只有在开关打开且 Client ID、Client Secret 和必要 endpoint 配置完整后才能启用；未启用时登录页不展示对应入口。OIDC 授权和换 Token 统一使用 `ServerAddress + /oauth/oidc`，SPA 再调用 `/api/oauth/oidc` 完成登录；测试环境因已启用而渲染 Google 和 Custom Provider 图标。
- 测试 GitHub/Google OAuth Client 已创建并写入测试配置。Google/OIDC 与 GitHub 的首次登录及重复登录均完成 HTTP 200 callback、会话接口和测试库 owner 一致性验证；重复登录没有创建新用户或新绑定。
- 当前测试实例运行镜像 `new-api:new-api-test-20260828T091835Z-91e861f8c9`，状态 `healthy`；测试 `/api/status` 返回 HTTP 200 且 `success=true`。
- GitHub/Google 用户侧解绑不支持，本轮不实现也不测试；现有 custom OAuth 与管理员维护接口保持原状，不在本次产品边界内。
- 本次只重建 `new-api-test`。生产 `new-api` 的容器 ID、镜像引用、镜像 ID、健康状态和重启次数在发布前后保持一致；PostgreSQL 与 Redis 未重建。

## 2. 本轮提交

| 提交 | 范围 | 已验证结果 | 发布边界 |
| --- | --- | --- | --- |
| `dfed70b42` | 内置中英文敏感词、热更新 matcher、英文边界匹配、403 `content_policy_violation`、禁止重试、第三方许可证 | `setting`、`service`、`controller`、`model` 测试通过；`relaykit` 独立构建通过 | 测试实例已包含；未验证生产配置是否启用内容检查 |
| `3ba312b31` | Stripe 当前订阅只接受 `active`、`trialing`、`past_due`、`unpaid`；历史 invoice 继续保留 | 相关后端测试随上述四个 package 一并通过 | 测试实例已包含；未用真实 Stripe 账号或订单回读 |
| `2cfc6445f` | 前端用户文案、无障碍标签、i18n 检测器及全部 locale | i18n sync 通过；相关 Vitest 通过；前端类型检查与生产构建通过 | 测试实例已包含；未执行真实浏览器 E2E 或人工逐语言审校 |
| `a3ea507e3` | Dashboard 汇总所有有效订阅的剩余额度，并避免跨用户复用订阅缓存 | 指标与卡片组件测试通过；前端类型检查与生产构建通过 | 测试实例已包含；未用多个真实账号验证展示 |
| `430aff7c8` | Zgo 边缘全量切换计划、预签和最终 Caddy 候选配置 | staged diff 检查和敏感信息模式扫描通过 | 该提交自身只交付计划材料；实际执行状态后来由 `13af0b289` 更新 |
| `9c0dde868` | 上一轮发布前项目现状文档 | 文档 diff 与提交范围复核通过 | 作为上一轮 `20260826T143317Z` 测试镜像的精确代码提交 |
| `ba5cfebac` | 记录上一轮 GreenCloud 测试发布结果 | 现场测试健康、生产不变性、staged diff 和敏感信息扫描通过 | 文档提交，不改变已发布镜像 |
| `e5b733c4e` | 已有有效订阅时，Stripe、Creem、Epay、Waffo Pancake 在渠道调用和订单写入前统一拒绝再次购买 | 4 个目标 Go package 测试通过；Stripe 及另外 3 个支付入口均有回归测试 | 当前测试实例已包含；未实现订阅升级或按比例计费 |
| `1e66565f5` | 有效订阅期间保留套餐对比但禁用全部购买按钮；本地化套餐名和禁用原因 | 3 个 Vitest 文件、12 个测试通过；i18n sync、目标 oxlint/oxfmt、前端类型检查与生产构建通过 | 当前测试实例已包含；未执行真实浏览器和 Stripe Sandbox E2E |
| `13af0b289` | 将 Zgo 边缘运行手册由计划态更新为已执行、已验收状态 | 文档 diff 和敏感信息模式扫描通过 | 记录既有生产边缘事实；本次应用测试发布未改动 Zgo |
| `47e458353` | 删除已被运行手册和主机回滚目录取代的临时候选 Caddyfile | 仓库引用搜索和 staged diff 检查通过 | 只清理仓库临时材料，不删除主机回滚材料 |
| `00f0f3598` | 要求生产发布、切流、回滚及重大基础设施变更同步更新真实状态记录 | staged diff 和敏感信息模式扫描通过 | 本次不可变测试镜像的精确代码提交；该规则本身不改变运行时 |
| `6fd43b270` | 记录 Tryvalo 入站邮件路由的当前状态 | 文档 diff 和敏感信息模式扫描通过 | 文档提交；未由本次测试发布修改邮件或 DNS |
| `a01d82228` | 保留 CPA 直连路由的最终仓库配置与运行记录 | 文档及配置检查通过 | 进入 `main` 镜像构建上下文；本次未应用 Caddy、CPA 或边缘配置 |
| `b6b0b49c9` | 登录与注册法律文案组件化并完成全 locale i18n | 目标 Vitest、i18n、构建和格式检查通过 | 当前测试实例已包含；未执行浏览器逐语言人工审校 |
| `abce39b75` | GitHub/OIDC 身份原子归属、完整启用校验、统一 OIDC 回调和 OAuth provider 图标 | 目标 Go 测试、295 个前端测试、前端构建、i18n、格式与 diff 检查通过 | 上一测试镜像的精确 `main` 提交；当时真实 GitHub/Google E2E 待测试凭据 |
| `893e79268` | 将 OIDC 前端授权和后端换 Token 的 redirect URI 临时统一为 API callback `/api/oauth/oidc` | OIDC 回调单测、前端 URL 单测及测试环境 Google/OIDC callback HTTP 200 | 上一测试镜像的精确 `main` 提交；后由 `f28cacabdf` 改为 SPA callback |
| `815e26139` | 记录 `893e79268` 隔离测试发布和初次 Google/OIDC callback 证据 | 文档 diff、现场 HTTP、容器身份和测试库聚合复核通过 | 文档提交；不改变测试或生产运行时 |
| `519748d0e` | 纠正把用户消息误写成 GitHub E2E 通过的证据边界 | 记录搜索、staged diff 和敏感信息扫描通过 | 文档提交；明确以现场 callback 和会话证据为准 |
| `484649c8d` | 记录 GitHub callback、已认证 Dashboard 和测试库聚合闭环 | GitHub callback HTTP 200、Dashboard DOM、聚合 `1|1|0`、容器健康 | 文档提交；不改变测试或生产运行时 |
| `66276df51` | 允许同一本地账号绑定多个 provider 下尚未占用的身份，同时禁止身份复用和同 provider 换绑 | 全量 Go 测试、目标 model/controller/i18n 测试和 diff 检查通过 | 当前测试镜像已包含；不按邮箱合并账号 |
| `f28cacabdf` | OIDC 授权和换 Token 统一通过 SPA `/oauth/oidc` callback，使浏览器会话在进入 Dashboard 前持久化 | 全量 Go 测试、OIDC 前端测试、类型检查、生产构建和 diff 检查通过；Google 重复登录现场通过 | 上一测试镜像的精确 `main` 提交；生产未发布 |
| `04aa934d4` | 将套餐与权益统一为单一 CNY 月度计费契约；每个 Stripe paid invoice 创建独立账期权益，并加入开发阶段套餐数据迁移 | 全量 Go 测试、24 个订阅前端测试、typecheck、build、目标 lint/format、i18n 和 diff 检查通过 | 当前测试镜像已包含；测试库 4 条套餐已迁移，正式库套餐相关三表为 0 且未触碰 |
| `e15486acc` | Dashboard 复制 API Key 时按 ID 获取受保护的完整 Key，不再复制列表接口的掩码值 | 目标 Vitest 回归和受影响文件 lint 通过；后续全量前端测试继续覆盖 | 当前测试镜像已包含；测试账号无真实 Key，浏览器页面已验收，精确剪贴板契约由自动化测试证明 |
| `91e861f8c` | 清理前端既有 lint/format/type 回归并保持 Dashboard API Key 修复 | lint 0 error/0 warning；typecheck、76 文件/309 测试、build、format、全量 Go、`relaykit` 独立构建和 diff 检查通过 | 当前测试镜像的精确代码提交；生产未发布 |

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

本次订阅跟进提交又实际执行并通过：

```text
GOCACHE=/tmp/new-api-go-build go test ./setting ./service ./controller ./model
cd relaykit && GOWORK=off GOCACHE=/tmp/new-api-go-build go build ./...
cd web && bun run build:check
cd web && bun run test <3 个订阅组件测试文件>
cd web && bun run i18n:sync
oxlint <本次目标前端文件>
oxfmt --check <本次目标前端文件及 locale JSON>
git diff --check
```

- Go 测试在受限沙箱内首次因 `httptest` 无权绑定本地端口失败；在具备本地端口权限的执行边界重跑后，4 个目标 package 全部通过。
- 本次 Vitest 为 3 个文件、12 个测试，覆盖有效订阅期间按钮禁用、提示文案和账单套餐名本地化。
- `relaykit` 独立构建、前端类型检查与生产构建、i18n 同步、目标 lint/format、diff 检查均通过。
- 当前测试发布镜像的 Docker 多阶段构建通过，包含 Bun 前端生产构建与 Go 二进制构建；构建上下文来自精确提交 `00f0f3598` 的隔离归档，而非工作树。

本次 GitHub/Google 登录发布前又实际执行并通过：

```text
go test ./model ./oauth ./controller
cd web && bun run test
cd web && bun run build:check
cd web && bun run i18n:sync
cd web && bun run format:check
git diff --check
```

- Go：`model`、`oauth`、`controller` 三个目标 package 全部通过。
- Vitest：76 个测试文件、295 个测试全部通过。
- 前端生产构建、i18n 同步、格式检查与 Git diff 检查全部通过。
- 镜像从精确 `main` 提交 `abce39b75f37a01fb205cab70bf643390c2e5895` 的隔离归档构建为 `linux/amd64`，没有从功能分支或 dirty 工作树构建。

API callback 路径修正发布后在精确提交 `893e79268a691076d41e374c57c248521d803460` 上实际执行并通过：

```text
GOCACHE=/tmp/new-api-go-build go test ./oauth -run '^TestOIDCProvider_ExchangeTokenUsesAPICallback$' -count=1
cd web && bun run test src/lib/__tests__/oauth.test.ts
```

- OIDC：换 Token 时提交的 redirect URI 固定为 `ServerAddress + /api/oauth/oidc`，目标回归测试通过。
- 前端：1 个 Vitest 文件、1 个测试通过，确认 Google/OIDC 授权请求使用 `/api/oauth/oidc` callback。

身份绑定规则调整和浏览器 callback 修正随后在精确提交 `f28cacabdfdf25bc0b35e39fb42329136c62c7d0` 上实际执行并通过：

```text
GOCACHE=/tmp/new-api-go-build go test ./... -count=1
GOCACHE=/tmp/new-api-go-build go test ./model -run '^(TestUserCanBindDistinctOAuthChannelsWhenIdentitiesAreAvailable|TestDifferentUsersCanUseDifferentOAuthChannels|TestUpdateUserBindColumnRejectsExternalIdentityClaimedByAnotherUser|TestUpdateUserBindColumnRejectsSecondIdentityForSameProvider)$' -count=1
cd web && bun run test src/lib/__tests__/oauth.test.ts
cd web && bun run typecheck
cd web && bun run build
git diff --check
```

- Go：全量 package 测试通过；目标 model 用例确认一个本地账号可绑定多个不同且未占用的 provider 身份、已被其他用户占用的身份不能再次 claim、同一 provider 的既有身份不能被替换。
- 前端：OIDC URL 测试、类型检查和生产构建通过；授权与换 Token 使用完全一致的 SPA callback `/oauth/oidc`。
- 解绑：GitHub/Google 用户侧不支持，本轮没有新增或修改解绑代码，也不把解绑列为待测路径。

月度套餐、Dashboard API Key 修复和全量质量门随后在精确提交 `91e861f8c970a0b5c1897eeabaaca17f07fa1613` 上实际执行并通过：

```text
GOCACHE=/tmp/new-api-go-build go test ./...
cd relaykit && GOWORK=off go build ./...
cd web && bun run lint
cd web && bun run typecheck
cd web && bun run test
cd web && bun run build
cd web && bun run format:check
git diff --check
```

- Go：全量 package 测试通过；`relaykit` 在 `GOWORK=off` 下独立构建通过。
- 前端：lint 为 0 error、0 warning；typecheck、76 个文件/309 个 Vitest、生产构建、全量格式检查全部通过。
- Dashboard：目标回归测试确认复制动作获取完整 Key，而不是复制掩码列表值。
- 套餐：开发阶段单向月度迁移测试通过；测试环境运行后 4 条套餐全部为 `month/1` 与 `billing_cycle` 契约，相关迁移字段完整。
- Stripe：上述本地质量门不替代 Sandbox 业务验证；业务 E2E 证据单列在 4.4。

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
| 发布时间 | `2026-08-27 08:35:14`（Asia/Shanghai） |
| 提交 | `00f0f359810cfd0a1e62d57c1bd9e64cfff8e935` |
| Release ID / 镜像 | `new-api-test-20260827T002357Z-00f0f3598` / `new-api:new-api-test-20260827T002357Z-00f0f3598` |
| 平台 | `linux/amd64` |
| 镜像 ID | `sha256:9e657ef349efb8282ccc0e9fe8d35b0a729869e89659da1f2ff3ebfe3f2a1d1b` |
| 镜像包 SHA-256 | `a66652e79e29cf15687d140b4939acde9178bb9bce1e2f41e552bb962f2acf23` |
| GreenCloud 镜像包 | `/srv/new-api-test/releases/new-api-test-20260827T002357Z-00f0f3598.tar.gz` |
| 发布后测试 Compose SHA-256 | `a0b913daef917c0dc6217a638cf14f2b3bc2cc81d89789e12183349a8fa06dff` |
| 旧测试镜像 | `new-api:new-api-test-20260826T143317Z-9c0dde868`，镜像 ID `sha256:e709317dbd9a19b7b5b49862c299945fe4e700241cb1bbe5fd663ac653227fab` |
| Compose 备份 | `/srv/new-api-test/backups/compose.yaml.before-new-api-test-20260827T002357Z-00f0f3598` |
| 测试库备份 | `/srv/new-api-test/backups/pre-new-api-test-20260827T002357Z-00f0f3598.dump` |
| 测试库备份 SHA-256 | `c14f8dc570ff1aa5fa1dc1694acccc137e441398ecd434b80094a9aebcbcc505` |

发布动作只加载新镜像、更新测试 Compose 中唯一的镜像引用，并执行：

```text
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --force-recreate new-api-test
```

2026-08-27 08:35:35（Asia/Shanghai）独立回读结果：

- `new-api-test`：`running`、`healthy`、重启次数 0，运行镜像 ID 与构建镜像一致。
- `GET http://127.0.0.1:3001/api/status`：HTTP 200 且 `success: true`。
- `GET http://127.0.0.1:3001/`：HTTP 200。
- 最近 10 分钟启动日志中的 `panic`、`fatal`、迁移/数据库关键错误计数：0。
- 生产 `new-api`：仍为 `new-api:new-api-release-20260821T083301Z-52055bbf`，镜像 ID 仍为 `sha256:915b85ceef61ef8bb35294d589b6d4a57f07ab49594ea0ba3c071c8b73e0df2d`，状态 `healthy`。
- `new-api-postgres`、`new-api-redis`：均为 `healthy`。
- 生产 `GET http://127.0.0.1:3000/api/status`：HTTP 200；生产容器在发布前后镜像引用和镜像 ID 完全一致，未重建。
- 测试库仍为独立 `newapi_test`，现场大小约 465 MB；测试和生产连接串 SHA-256 不同，未在文档中记录连接串本身。

容器级回滚命令：

```bash
cp -p /srv/new-api-test/backups/compose.yaml.before-new-api-test-20260827T002357Z-00f0f3598 /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --force-recreate new-api-test
```

如需恢复测试数据库，应先停止测试写入并另行制定恢复步骤；本次未执行数据库恢复演练。

### 4.1 GitHub/Google 登录代码测试发布（较早版本）

发布范围为 A 类测试应用发布，并单列 OAuth claim schema 风险检查。正式生产、DNS、Cloudflare、Caddy、Zgo、CPA、PostgreSQL 和 Redis 均不在变更范围内。

不可变发布证据：

| 项目 | 值 |
| --- | --- |
| 测试容器启动时间 | `2026-08-27 17:55:44`（Asia/Shanghai） |
| 提交 | `abce39b75f37a01fb205cab70bf643390c2e5895`，发布前与 `origin/main` 一致 |
| Release ID / 镜像 | `new-api-test-20260827T093802Z-abce39b75` / `new-api:new-api-test-20260827T093802Z-abce39b75` |
| 平台 | `linux/amd64` |
| 镜像 ID | `sha256:926b0a7ee78a5e54223fb6bb125a6dc8dbf4c4933e10405c43cdfcbb1d963b76` |
| 镜像包 SHA-256 | `0900cebf482e62f19bd2ac03624510ca7effdf9b96839671679114f4a3c014f4` |
| 镜像包大小 | `71,696,973` bytes |
| GreenCloud 镜像包 | `/srv/new-api-test/releases/new-api-test-20260827T093802Z-abce39b75.tar.gz` |
| 发布后测试 Compose SHA-256 | `d8ffd66152e355fe33e3e330c31b7b584a1c0b9c0dc974a0a7bcf815e1caad22` |
| 发布前测试镜像 | `new-api:new-api-test-20260827T002357Z-00f0f3598`，镜像 ID `sha256:9e657ef349efb8282ccc0e9fe8d35b0a729869e89659da1f2ff3ebfe3f2a1d1b` |
| Compose 备份 | `/srv/new-api-test/backups/new-api-test-20260827T093802Z-abce39b75/compose.yaml.before-new-api-test-20260827T093802Z-abce39b75` |
| Compose 备份 SHA-256 | `a0b913daef917c0dc6217a638cf14f2b3bc2cc81d89789e12183349a8fa06dff` |
| 测试库备份 | `/srv/new-api-test/backups/new-api-test-20260827T093802Z-abce39b75/newapi_test.before-new-api-test-20260827T093802Z-abce39b75.dump` |
| 测试库备份 SHA-256 | `322907ac64f2c329b7f02fe4f1a24fd9f9de924b4cb5eb57b2a46bcacaf414d0` |

发布前测试库只读检查结果：

- `users.github_id`、`users.oidc_id`、`users.telegram_id` 的非空历史绑定数均为 0，重复 subject 分组数均为 0。
- `external_identity_claims` 已存在，行数为 0；`(provider, subject)` 与 `(provider, user_id)` 两组唯一索引均已存在，未发现 legacy owner 冲突。
- 测试服务固定为 `NODE_TYPE=slave`。从上一测试提交到当前 `main` 的数据库相关变化只有 GitHub/OIDC claim 回填；由于没有历史绑定、表和唯一索引已经就绪，本次没有启动临时 master，也没有执行手写数据迁移。

远端先通过完整 SHA-256 和 `gzip -t` 校验镜像包，再执行 `docker load`。随后只把测试 Compose 的唯一镜像引用从上一测试 tag 替换为当前 tag，`docker compose config -q` 通过，并执行：

```text
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test
```

2026-08-27 17:58（Asia/Shanghai）独立回读结果：

- `new-api-test` 容器 ID 为 `597380c6fce4c094c9c39d554558166d04d289fa7efba629bf96d26e274149fc`，运行镜像 ID 与构建镜像一致，状态 `running/healthy`、重启次数 0。
- 测试内网 `/api/status`、首页、`/sign-in` 均为 HTTP 200；公网 `https://test.tryvalo.com/api/status` 与 `/sign-in` 均为 HTTP 200，TLS 校验通过。
- 最近 10 分钟关键启动日志中 `panic`、`fatal`、迁移/数据库关键错误计数为 0。
- 测试 Compose 与发布前备份的差异只有旧、新两行镜像引用；PostgreSQL、Redis 未重建。
- 生产 `new-api` 容器 ID 仍为 `dd32f52cf926231547112c0c7390a2f456e432fd4b312be9a8fd7388a4a44776`，镜像 ID 仍为 `sha256:915b85ceef61ef8bb35294d589b6d4a57f07ab49594ea0ba3c071c8b73e0df2d`，状态 `running/healthy`、重启次数 0；内网及公网 `api.tryvalo.com/api/status` 均为 HTTP 200。

容器级回滚命令：

```bash
cp -p /srv/new-api-test/backups/new-api-test-20260827T093802Z-abce39b75/compose.yaml.before-new-api-test-20260827T093802Z-abce39b75 /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test
```

如需恢复测试数据库，应先停止测试写入并使用本节记录的 custom-format dump 制定单独恢复步骤；本次未执行恢复演练。

### 4.2 API callback 路径修正与首次 OAuth 验收（上一版）

本次仍为 A 类测试应用发布。发布内容仅是 `main` 上的 OAuth callback 路径修正及测试环境 OAuth 配置；生产应用、生产 OAuth 配置、DNS、Cloudflare、Caddy、Zgo、CPA、PostgreSQL 和 Redis 均未改动。OAuth Client Secret 只保存在受限配置中，没有写入仓库、命令输出或本文。

不可变发布证据：

| 项目 | 值 |
| --- | --- |
| 测试容器启动时间 | `2026-08-28 02:24:12`（Asia/Shanghai） |
| 提交 | `893e79268a691076d41e374c57c248521d803460`，发布前与 `origin/main` 一致 |
| Release ID / 镜像 | `new-api-test-20260827T182015Z-893e79268a` / `new-api:new-api-test-20260827T182015Z-893e79268a` |
| 平台 | `linux/amd64` |
| 镜像 ID | `sha256:9257bbe16e3b6fe9466f29ee638959133bcd177684ddaad0056a987141a97849` |
| 镜像包 SHA-256 | `7a90838a7c913161ea1961a5fae12cf7e4817505b056806d2645ce6e9d2d892b` |
| 镜像包大小 | `71,718,119` bytes |
| GreenCloud 镜像包 | `/srv/new-api-test/releases/new-api-test-20260827T182015Z-893e79268a.tar.gz` |
| 发布后测试 Compose SHA-256 | `bdd84c56cfa0f68809610a790ece57a86151064c54bff39edd391fd9140c189a` |
| 发布前测试镜像 | `new-api:new-api-test-20260827T093802Z-abce39b75`，镜像 ID `sha256:926b0a7ee78a5e54223fb6bb125a6dc8dbf4c4933e10405c43cdfcbb1d963b76` |
| Compose 备份 | `/srv/new-api-test/backups/new-api-test-20260827T182015Z-893e79268a/compose.yaml.before-new-api-test-20260827T182015Z-893e79268a` |
| Compose 备份 SHA-256 | `39fb2d275917cb79021b49ab157b0ab20643e3514a9c96f243d6a4c98f0eb794` |
| 测试库备份 | `/srv/new-api-test/backups/new-api-test-20260827T182015Z-893e79268a/newapi_test.before-new-api-test-20260827T182015Z-893e79268a.dump` |
| 测试库备份 SHA-256 | `e4542f6cf8b1aa56a250fcd0acfdacb26a317cd53f4eb1bc62ef72c5ff74b06b` |

远端先核对镜像包 SHA-256，再加载镜像并仅更新测试 Compose 的镜像引用；随后通过 `docker compose config -q`，并只重建 `new-api-test`：

```text
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test
```

OAuth 现场结果：

- `2026-08-28 03:02:28`（Asia/Shanghai），Google/OIDC 的 `GET /api/oauth/oidc` callback 返回 HTTP 200，服务端耗时约 432 ms。
- `2026-08-28 09:34:13`（Asia/Shanghai），从已登出的测试登录页发起一条全新 GitHub 流程；`GET /api/oauth/github` callback 返回 HTTP 200，服务端耗时约 662 ms，随后浏览器进入 `/dashboard/overview` 并渲染已认证头像控件。
- GitHub callback 后测试库只读聚合仍为：存在 GitHub 绑定的用户 1、存在 OIDC 绑定的用户 1、同时绑定两者的用户 0。GitHub 数量未增加符合既有绑定用户再次登录的预期；查询未输出邮箱、用户名、provider subject 或其他身份字段。
- 登录页已现场观察到 GitHub、Google 两个入口和 Google 图标；Google 与 GitHub callback 均已出现，GitHub callback 后已读取 Dashboard DOM。
- Chrome Connector 曾连续超时，一次早期页面展示被本机扩展标记为 `ERR_BLOCKED_BY_CLIENT`；随后改用本机 CDP Proxy 完成 GitHub 流程和 Dashboard DOM 回读。该版本当时尚未完成 Google 重复登录和已占用身份绑定失败提示的浏览器证据；GitHub/Google 用户侧解绑不属于支持范围。

`2026-08-28 09:38:10`（Asia/Shanghai）最终只读回读：

- `new-api-test`：`running`、`healthy`、重启次数 0，镜像引用和镜像 ID 与上表一致。
- 测试本机 `127.0.0.1:3001/api/status`、公网 `https://test.tryvalo.com/api/status` 和 `/sign-in` 均返回 HTTP 200。
- 生产 `new-api`：`running`、`healthy`、重启次数 0，仍为 `new-api:new-api-release-20260821T083301Z-52055bbf` 和 `sha256:915b85ceef61ef8bb35294d589b6d4a57f07ab49594ea0ba3c071c8b73e0df2d`。
- 生产本机 `127.0.0.1:3000/api/status`、公网 `https://api.tryvalo.com/api/status` 和 `/sign-in` 均返回 HTTP 200；本次没有重建生产容器。

容器级回滚命令：

```bash
cp -p /srv/new-api-test/backups/new-api-test-20260827T182015Z-893e79268a/compose.yaml.before-new-api-test-20260827T182015Z-893e79268a /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test
```

如需恢复测试数据库，应先停止测试写入并使用本节记录的 custom-format dump 制定单独恢复步骤；本次未执行恢复演练。

### 4.3 多 provider 绑定规则与重复登录验收（上一版）

本次仍为 A 类测试应用发布。发布内容是 `main` 上的多 provider 身份绑定规则和 OIDC 浏览器 callback 修正。生产应用、生产 OAuth 配置、DNS、Cloudflare、Caddy、Zgo、CPA、PostgreSQL 和 Redis 均未改动。OAuth Client Secret 仍只保存在受限配置中，没有写入仓库、命令输出或本文。

不可变发布证据：

| 项目 | 值 |
| --- | --- |
| 测试容器启动时间 | `2026-08-28 11:34:27`（Asia/Shanghai） |
| 提交 | `f28cacabdfdf25bc0b35e39fb42329136c62c7d0`，发布前与 `origin/main` 一致 |
| Release ID / 镜像 | `new-api-test-20260828T033014Z-f28cacabdf` / `new-api:new-api-test-20260828T033014Z-f28cacabdf` |
| 平台 | `linux/amd64` |
| 镜像 ID | `sha256:25500e7f851d56b7bb8b71ceea19f3887f1ed9230e718417c6c7a55c81b37d3a` |
| 镜像包 SHA-256 | `7b0a316f727d29d416258b31821fc5919c8e622e8e799999a9be02d19f58f98a` |
| 镜像包大小 | `71,722,601` bytes |
| GreenCloud 镜像包 | `/srv/new-api-test/releases/new-api-test-20260828T033014Z-f28cacabdf.tar.gz` |
| 发布后测试 Compose SHA-256 | `05ade21592c163c070efee06b9aad043532e06d785b34905bf2c04668b96f72c` |
| 发布前测试镜像 | `new-api:new-api-test-20260827T182015Z-893e79268a`，镜像 ID `sha256:9257bbe16e3b6fe9466f29ee638959133bcd177684ddaad0056a987141a97849` |
| Compose 备份 | `/srv/new-api-test/backups/new-api-test-20260828T033014Z-f28cacabdf/compose.yaml.before-new-api-test-20260828T033014Z-f28cacabdf` |
| Compose 备份 SHA-256 | `ac0f294304dfb527d804527c5d20e465301ce0938a1e26dc06565a7a4ebed8c2` |
| 测试库备份 | `/srv/new-api-test/backups/new-api-test-20260828T033014Z-f28cacabdf/newapi_test.before-new-api-test-20260828T033014Z-f28cacabdf.dump` |
| 测试库备份 SHA-256 | `89d5e8f179c90c469f4675fb97ff4c7b365a0683a25c703f20fcda4af763cbba` |

远端核对镜像包后，仅更新测试 Compose 的镜像引用并重建 `new-api-test`；生产容器、PostgreSQL 和 Redis 未重建。

OAuth 现场结果：

- 外部 provider redirect URI 使用浏览器路由 `/oauth/github` 和 `/oauth/oidc`；SPA 再调用 `/api/oauth/github` 或 `/api/oauth/oidc` 完成服务端 callback。OIDC 授权与换 Token 的 `redirect_uri` 都是 `ServerAddress + /oauth/oidc`，逐字一致。
- `2026-08-28 11:51:29`（Asia/Shanghai），Google/OIDC 重复登录的 `GET /api/oauth/oidc` callback 返回 HTTP 200；session refresh 和当前用户接口均为 HTTP 200，`login_method = oauth:oidc`。返回本地用户 ID 的 SHA-256 与数据库 OIDC owner 一致：`86e50149658661312a9e0b35558d84f6c6d3da797f552a9657fe0558ca40cdef`。
- `2026-08-28 12:06:53`（Asia/Shanghai），GitHub 重复登录的 `GET /api/oauth/github` callback 返回 HTTP 200，浏览器返回 `/dashboard/overview`；session refresh 和当前用户接口均为 HTTP 200，`login_method = oauth:github`。返回本地用户 ID 的 SHA-256 与数据库 GitHub owner 一致：`c6f3ac57944a531490cd39902d0f777715fd005efac9a30622d5f5205e7f6894`。
- `2026-08-28 12:14:14`（Asia/Shanghai），测试库只读聚合为：用户总数 34、GitHub 绑定 1、OIDC 绑定 1、同时绑定两者 0。两次重复登录均未新增用户或 provider 绑定。查询未输出邮箱、用户名、provider subject 或原始本地用户 ID。
- 已占用 GitHub 身份的二次绑定尝试在 popup 到达 `/oauth/github` 后被 Chrome 危险网站安全页拦截。`2026-08-28 12:01:48` 的 `/api/oauth/github` 403 来源与浏览器不同，符合安全扫描请求特征，因此不作为真实绑定拒绝证据。服务端规则已有自动化测试，页面提示仍未现场验证。
- GitHub/Google 用户侧解绑明确不支持，本轮未实现、未测试；现有 custom OAuth 与管理员维护接口未修改。

`2026-08-28 12:14:14`（Asia/Shanghai）最终只读回读：

- `new-api-test` 容器 ID 为 `446224475e75283ebafeee81da0d6d7cbf63f548c1135ff9e1e86104e53fb38d`，状态 `running/healthy`、重启次数 0，镜像引用和镜像 ID 与上表一致。
- 测试本机 `127.0.0.1:3001/api/status`、公网 `https://test.tryvalo.com/api/status` 和 `/sign-in` 均返回 HTTP 200；测试 Compose 解析通过。
- 生产 `new-api` 容器 ID 仍为 `dd32f52cf926231547112c0c7390a2f456e432fd4b312be9a8fd7388a4a44776`，状态 `running/healthy`、重启次数 0，仍为 `new-api:new-api-release-20260821T083301Z-52055bbf` 和 `sha256:915b85ceef61ef8bb35294d589b6d4a57f07ab49594ea0ba3c071c8b73e0df2d`。
- 生产本机 `127.0.0.1:3000/api/status` 与公网 `https://api.tryvalo.com/api/status` 均返回 HTTP 200；本次没有重建生产容器。

容器级回滚命令：

```bash
cp -p /srv/new-api-test/backups/new-api-test-20260828T033014Z-f28cacabdf/compose.yaml.before-new-api-test-20260828T033014Z-f28cacabdf /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test
```

如需恢复测试数据库，应先停止测试写入并使用本节记录的 custom-format dump 制定单独恢复步骤；本次未执行恢复演练。

### 4.4 月度套餐、Dashboard 修复与 Stripe Sandbox 充值验收（当前）

本次仍为 A 类测试应用发布。发布范围是 `main` 上的月度套餐契约、Dashboard API Key 复制修复和全量前端质量门修复；随后只在独立测试实例完成一笔 Stripe Sandbox 充值。生产应用、生产数据库、生产 Stripe 配置、DNS、Cloudflare、Caddy、Zgo、CPA、PostgreSQL 和 Redis 均未改动。Creem、Epay、Waffo 明确不在本次验收范围内。

不可变发布证据：

| 项目 | 值 |
| --- | --- |
| 测试容器启动时间 | `2026-08-28 17:35:13`（Asia/Shanghai） |
| 提交 | `91e861f8c970a0b5c1897eeabaaca17f07fa1613` |
| Release ID / 镜像 | `new-api-test-20260828T091835Z-91e861f8c9` / `new-api:new-api-test-20260828T091835Z-91e861f8c9` |
| 平台 | `linux/amd64` |
| 镜像 ID | `sha256:d74398a36f8a7655ce1bff6cb98c0035f554cc0ba0445472b9a8a9fa999e08bc` |
| 镜像包 SHA-256 | `42933cc9f3abf3ae63072ca74376c419b1880c27aaa2caf169ce551f658896b9` |
| 镜像包大小 | `71,832,852` bytes |
| GreenCloud 镜像包 | `/srv/new-api-test/releases/new-api-test-20260828T091835Z-91e861f8c9.tar.gz` |
| 发布后测试 Compose SHA-256 | `964be300832cb295bfb9e669d32cc69b8979163490509424ee61fb94bc29cc7b` |
| 发布前测试镜像 | `new-api:new-api-test-20260828T033014Z-f28cacabdf`，镜像 ID `sha256:25500e7f851d56b7bb8b71ceea19f3887f1ed9230e718417c6c7a55c81b37d3a` |
| Compose 备份 | `/srv/new-api-test/backups/new-api-test-20260828T091835Z-91e861f8c9/compose.yaml.before-new-api-test-20260828T091835Z-91e861f8c9` |
| Compose 备份 SHA-256 | `05ade21592c163c070efee06b9aad043532e06d785b34905bf2c04668b96f72c` |
| 测试库备份 | `/srv/new-api-test/backups/new-api-test-20260828T091835Z-91e861f8c9/newapi_test.before-new-api-test-20260828T091835Z-91e861f8c9.dump` |
| 测试库备份 SHA-256 | `21872da4e82f198150a678d33e5de8d3a9ff3fa1998f0156e752ca6c7ec796ee` |

迁移与运行时回读：

- 测试库原有 4 条套餐全部规范化为 `duration_unit=month`、`duration_value=1`、`custom_seconds=0`、`quota_reset_period=billing_cycle`、`quota_reset_custom_seconds=0`，并完成 `public_visible` 回填；6 个迁移字段齐全，无非月度或缺字段记录。
- 迁移是开发阶段的单向数据规范化。正式库套餐相关三表在发布前只读计数均为 0，且本轮未运行正式迁移，所以不存在需要保留的旧正式套餐；测试库 4 条数据已迁移成功，无需删除。
- 旧迁移包装脚本曾因等待 `database migrated` 日志而报告失败；正常 `migrateDB()` 路径成功时不输出该 marker。最终以迁移后字段和数据不变量回读为准，临时 migration 容器已经删除。
- `new-api-test` 当前镜像与上表一致，状态 `running/healthy`；`/api/status` 返回 HTTP 200 且 `success=true`。
- 生产 `new-api` 仍运行 `new-api:new-api-release-20260821T083301Z-52055bbf`，镜像 ID 仍为 `sha256:915b85ceef61ef8bb35294d589b6d4a57f07ab49594ea0ba3c071c8b73e0df2d`，状态 `running/healthy`，启动时间未变化。

Stripe Sandbox 充值 E2E 于 `2026-08-28 17:41-18:31`（Asia/Shanghai）使用隔离合成用户完成：

- Hosted Checkout 提交一笔 CNY 20 测试支付；只使用 Stripe 公共测试卡和虚构联系资料，未保存 Link 信息，并勾选 AI agent 代理披露。
- 该用户最终恰好只有 1 条 `top_ups`：`pending=0`、`success=1`、`amount=20`、`expected_amount_minor=2000`、`expected_currency=CNY`、`provider_livemode=false`；Checkout Session、PaymentIntent、Charge 引用均存在。
- 最新 `checkout.session.completed` 为 Sandbox 事件，状态 `succeeded`、`attempts=1`、`last_error` 为空，事件时间与订单完成时间一致。
- `credited_quota=10000000`；用户 `quota=10000000`、`billing_debt=0`。Wallet 显示 `$20`，Billing History 恰好 1 条 Stripe `Success`，Usage Logs 恰好 1 条 Top-up，没有重复入账。
- Top-up 日志的 `logs.quota=0` 是现有记录语义：真实充值额度以 `top_ups.credited_quota` 和 `users.quota` 为准，不是结算失败。
- 清除旧控制台记录后，Wallet 与付款后客户路径没有新的应用 console error。

该闭环证明的是测试实例的 Stripe Sandbox 单次充值；它不证明 Stripe Live、生产 webhook、Stripe 订阅续费/退款/争议、其他支付渠道或管理员流程。

容器级回滚命令：

```bash
cp -p /srv/new-api-test/backups/new-api-test-20260828T091835Z-91e861f8c9/compose.yaml.before-new-api-test-20260828T091835Z-91e861f8c9 /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test
```

上述 Compose 备份精确恢复到 `new-api-test-20260828T033014Z-f28cacabdf`。测试库的单向套餐迁移如需恢复，应先停止测试写入，再使用本节 custom-format dump 制定单独恢复步骤；本次没有执行数据库恢复演练，正式库不需要此回滚。

## 5. 已知验证缺口

- 当前 `91e861f8c` 已通过全量 Go 测试、`relaykit` 独立构建，以及前端 lint、typecheck、76 个文件/309 个 Vitest、生产 build 和 format check；此前“未跑全量 Vitest”和“lint 仍有 17 error/3 warning”两个缺口已经关闭。
- Dashboard API Key 复制的精确剪贴板契约由回归测试证明；隔离浏览器账号没有真实 API Key，因此现场只验证了部署后的 Dashboard 页面，没有再次创建并复制一枚真实 Key。
- `setting/data/google_profanity_en.txt` 为第三方上游的精确快照，保留了 43 行上游尾随空格；运行时解析会 `TrimSpace`，相关测试已通过。
- Stripe Sandbox 单次充值的 Hosted Checkout、真实 webhook、数据库结算和客户页面 E2E 已完成；仍未验证 Stripe 订阅续费/退款/争议、Stripe Live 或生产支付。Creem、Epay、Waffo 是本次明确排除项，不作为 Stripe 发布失败。
- 仍未执行付费模型请求、管理员登录、多个真实账号 Dashboard 展示或全站完整浏览器 E2E。
- 测试 GitHub/Google OAuth Client 已创建并配置。Google/OIDC 与 GitHub 的首次和重复登录均已通过 callback、会话接口及数据库 owner 一致性验证。已占用外部身份的绑定拒绝由自动化测试覆盖，但浏览器提示被 Chrome 安全页阻断，仍未形成现场证据。GitHub/Google 用户侧解绑不支持，不是验证缺口。
- 没有实现或验证订阅升级、按比例计费、原订阅取消和权益迁移；当前明确采用“有效订阅期间可比较套餐但禁止再次购买”的产品边界。
- `/api/status` 当前返回的 `version` 为空，不能单独证明运行提交；本次以不可变镜像 tag、镜像 ID、构建提交和包哈希建立对应关系。
- 测试套餐迁移已完成字段与数据回读；正式库套餐相关三表为空且未触碰。仍没有执行数据库恢复演练、完整 schema diff 或真实 MySQL 实例迁移验证。
- 没有完成产品文案和各语言翻译的人工语义审校；同步报告为结构和未翻译检测结果，不等同于人工质量验收。

## 6. 发布与运维状态

| 项目 | 当前状态 |
| --- | --- |
| 本地代码 | 月度套餐、Dashboard API Key 修复和全量质量门已在精确提交 `91e861f8c` 完成验证，并从干净提交归档构建镜像 |
| 远端仓库 | 本次运行记录推送完成后的状态：测试交付点 `91e861f8c` 及本文均位于 `origin/main`，本地 `HEAD` 与远端一致 |
| 测试部署 | 已完成；GreenCloud `new-api-test` 运行不可变镜像 `new-api-test-20260828T091835Z-91e861f8c9`；4 条测试套餐已迁移，Stripe Sandbox 单次充值 E2E 已通过 |
| 生产部署 | 本轮未执行；生产容器身份与健康状态已回读为不变 |
| 生产业务回读 | 未执行管理员、Stripe Live、模型请求或账单 E2E；Sandbox 结果和容器健康不能替代生产业务证明 |
| Zgo / DNS / Cloudflare | Zgo 全量切换已在专门运行手册中记录为已执行和已验收；本次应用测试发布未触碰边缘配置 |

此前未提交的 Zgo 执行手册、`.codex-cutover` 状态和 `AGENTS.md` 已分别由 `13af0b289`、`47e458353`、`00f0f3598` 提交。当前镜像从干净的精确提交 `91e861f8c970a0b5c1897eeabaaca17f07fa1613` 归档构建；镜像加载和 Compose 重建全程通过 SSH 与 Docker Compose 执行。浏览器只用于 OAuth 和 Stripe Sandbox 用户路径验收，生产未部署。

## 7. 后续门槛

1. 测试 OAuth Client 已创建，provider callback 分别为 `https://test.tryvalo.com/oauth/github` 和 `https://test.tryvalo.com/oauth/oidc`；凭据继续只保存在本地受限文件和测试配置，生产配置保持不变。两种 provider 的重复登录已验收；如要消除当前 concern，只需在不触发 Chrome 安全拦截的受控浏览器里补充“已被另一用户占用的外部身份再次绑定”失败提示证据。GitHub/Google 用户侧解绑不支持，不列为后续工作。
2. Stripe Sandbox 充值 E2E 已完成；后续 Stripe 范围如需继续，优先补订阅续费/退款/争议，而不是重复本次充值路径。管理员登录、无订阅/有有效订阅两种套餐界面和内容策略拒绝路径仍需单独验收。
3. Creem、Epay、Waffo 不在本次 Stripe 范围；只有另行确定要发布这些渠道时，才建立各自的 Sandbox/回调/结算验收门槛。
4. 如测试需要真实上游模型或长连接/SSE，先明确 token、费用与观察范围，再执行并核对请求日志及单次计费。
5. 正式生产发布必须作为单独动作，以固定提交重新构建或复用已审计镜像，执行生产备份、迁移前计数、健康与业务回读，并记录独立回滚窗口。
6. 若未来要支持升级，必须先明确各支付渠道的订阅变更 API、proration、失败回滚、原订阅取消时序和本地权益迁移规则，再补后端原子性与真实 Sandbox E2E；当前不得把再次购买当作升级。
7. Zgo、DNS、Cloudflare、Caddy 和防火墙后续变更继续使用各自的执行门槛，不能由本次测试实例成功推导为可直接变更。
