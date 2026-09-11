# Tryvalo 敏感词分级策略正式发布状态

## 结果

精确提交 `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75` 已于 2026-09-01 21:28（Asia/Shanghai）发布到 GreenCloud 正式应用。正式环境运行不可变 `linux/amd64` 镜像 `new-api:new-api-release-20260901T132333Z-fc6ebe122e`，其镜像 ID 与测试环境已验收镜像完全相同。应用容器最终为 `running/healthy`、重启次数 `0`，本机及 `api.tryvalo.com`、`new.tryvalo.com` 的 `/api/status` 均返回 HTTP `200`。

正式库旧 `SensitiveWords` 覆盖值已在发布备份完成后删除；`SensitiveWords`、`SensitiveWordsHighRisk`、`SensitiveWordsAudit` 当前均无持久化行，因此运行时使用当前镜像内置的高风险阻断、NSFW 阻断和仅审计放行三层默认词表。

本次没有读取或使用已有生产 token，因此没有执行生产策略业务请求 E2E。测试环境对同一镜像 ID 的真实接口验收已确认 `成人色情` 与 `炸弹制作` 分别按 NSFW 和高风险策略返回 `403 content_policy_violation`，`淫威` 记录 `category=audit action=audit` 后越过本地策略；这些证据证明镜像内策略行为，不证明生产允许路径已经成功取得模型响应。

| 阶段 | 状态 | 证据 |
| --- | --- | --- |
| Planned | 已完成 | 精确提交、复用测试镜像、正式服务、option 处理、备份、健康门和回滚边界均在执行前固定。 |
| Production execution | 已完成 | 仅重建正式 `new-api` 应用；PostgreSQL 和 Redis 的容器 ID 与启动时间未变化。 |
| Option migration | 已完成 | 删除旧 `SensitiveWords` 覆盖；三项敏感词 option 当前均为 `0` 行，镜像默认三层词表生效。 |
| Health validation | 已完成 | 容器健康、重启次数、启动错误模式、两个正式公网入口及本机状态接口均通过。 |
| Business E2E | 待定 | 未读取或使用生产 token，未发起普通、audit、NSFW 或 high-risk 的生产业务请求。 |
| Rollback | 未执行 | 截至 2026-09-01 21:40（Asia/Shanghai）正式镜像健康；镜像与数据库 option 的回滚材料均已保留。 |

## 发布身份

- 应用提交：`fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`
- 提交意图：`Reduce sensitive-word false positives with tiered policy`
- 远端仓库边界：该提交尚未 push；正式镜像固定来自本地精确提交归档，不包含后续提交或工作树改动。
- 平台：`linux/amd64`
- OCI revision：`fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`
- 测试验收镜像：`new-api:new-api-test-20260901T064123Z-fc6ebe122e`
- 正式镜像：`new-api:new-api-release-20260901T132333Z-fc6ebe122e`
- 镜像 ID：`sha256:268d7cff740cfdbb7b97d9f3f3d8b651c8633d2eeb60c4bc92a869b246a34876`
- 正式应用启动时间：`2026-09-01T13:28:19.177904701Z`（2026-09-01 21:28:19，Asia/Shanghai）
- 最终状态：`running`、`healthy`、重启次数 `0`

正式 tag 与测试 tag 指向相同镜像 ID，且远端镜像的 `linux/amd64` 平台和 OCI revision 与上表一致。本次固定复用已完成测试环境策略验收的镜像，没有从当前工作树或后续提交重新构建。

## 变更范围

- 正式应用从 `new-api:new-api-release-20260830T025223Z-f96bf33b80` 切换到上述敏感词分级镜像。
- 正式库删除三项敏感词 option 中唯一存在的旧 `SensitiveWords` 行，使镜像内三层默认词表成为运行时来源。
- 只重建 `new-api`；`new-api-postgres` 与 `new-api-redis` 未重建，容器 ID、启动时间和重启次数保持不变。
- 没有数据库 schema、Stripe、DNS、Cloudflare、Caddy、Zgo、CPA 或防火墙变更。

## 发布前验证

- `go test ./setting ./service ./model ./controller -count=1`：通过。
- `bun run typecheck`：通过。
- `bun run test`：通过，`80` files / `314` tests。
- `bun run build`：通过。
- 三份默认词表共有 `2,094` 个互斥有效词：`475` 个高风险、`548` 个 NSFW、`1,071` 个仅审计。
- 同一镜像 ID 的测试接口验收：NSFW 和 high-risk 均返回 `403 content_policy_violation`；audit 命中记录 `category=audit action=audit` 后越过本地策略。

测试上游凭据返回 `401 Invalid API key`，因此普通与 audit 允许路径的成功模型生成在测试和正式环境都仍为待定。

## 正式部署

- Compose：`/srv/new-api/compose.yaml`
- 镜像变量：`/srv/new-api/env/images.env`
- 服务：`new-api`
- 发布前镜像：`new-api:new-api-release-20260830T025223Z-f96bf33b80`
- 当前镜像：`new-api:new-api-release-20260901T132333Z-fc6ebe122e`
- 当前 Compose SHA-256：`aa8ec0bca4e11b135a62f5d626509efd874c91379244d994aa0b47cd901646d5`
- 当前 `images.env` SHA-256：`0f2344e8aed496c66fa1181c64638b329b99cb4fca18efadc6fb1850c2300c32`

只重建正式应用服务：

```bash
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml config -q
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api
```

## 数据库 option 处理

发布前正式库三项 key 中仅存在 `SensitiveWords`：

- 值大小：`23,297` bytes
- 行数：`2,093`
- MD5：`f21cc6d05662149fe0b6958d158d1eb0`
- 与旧默认词表比较：仅缺少 `代理`，没有额外词

这说明该行会覆盖新镜像的 NSFW 默认表，却不会把旧词自动拆分到 high-risk、NSFW 和 audit 三类。完成备份后，事务删除 `SensitiveWords`、`SensitiveWordsHighRisk`、`SensitiveWordsAudit` 三个 key；发布后回读总数为 `0`。缺少持久化行时，应用启动会加载镜像内置默认值；显式空字符串会清空列表，不等同于删除，本次没有写入空字符串。

## 备份与恢复材料

备份目录：`/srv/new-api/backups/new-api-release-20260901T132333Z-fc6ebe122e`

目录包含：

- `images.env.before`
- `compose.yaml.before`
- `compose.rendered.before.yaml`
- `runtime.before.txt`
- PostgreSQL custom-format `newapi.before-release.dump`
- `newapi.before-release.restore-list.txt`
- `sensitive-options.before.csv`
- `sensitive-options.summary.before.txt`
- `SHA256SUMS`

发布后再次执行 `sha256sum -c`，所有文件均通过。数据库 dump SHA-256 为 `7b9c6b2e23e7612ca903e83479094cc77b2491baa9c877ab1535eff480d8084c`。

## 正式入口与健康验证

2026-09-01 21:40（Asia/Shanghai）最终只读回读：

- `http://127.0.0.1:3000/api/status`：HTTP `200`
- `https://api.tryvalo.com/api/status`：HTTP `200`
- `https://new.tryvalo.com/api/status`：HTTP `200`
- 正式应用：目标镜像与镜像 ID 一致，`running/healthy`、重启次数 `0`
- 正式应用自 `2026-09-01T13:28:00Z` 起的启动日志中，`panic`、`fatal`、migration failure 和 database failure 模式匹配数为 `0`
- `new-api-postgres`：`running/healthy`、重启次数 `0`，启动时间仍为 `2026-07-11T01:00:44.13346078Z`
- `new-api-redis`：`running/healthy`、重启次数 `0`，启动时间仍为 `2026-07-12T00:26:12.673497206Z`
- 三项敏感词 option：总计 `0` 行
- 备份清单：全部 SHA-256 校验通过

这些结果证明应用、依赖容器、配置加载和入口健康，不扩展为所有渠道、模型、计费或用户业务路径均已验收。

## 业务验证边界

本次没有从正式库提取 token，也没有使用已有生产凭据发起模型请求。生产普通文本、audit 文本、NSFW 文本和 high-risk 文本四类业务请求均未执行。

可以确认的是正式环境运行了测试验收过的同一镜像 ID，且正式库没有持久化词表覆盖。不能据此宣称生产普通或 audit 请求已取得成功模型响应；这项验收需要单独授权可控凭据、费用和观察范围。

## 回滚

本次未触发自动回滚。应用镜像回滚材料位于上述备份目录；恢复 `images.env.before` 后，仅重建 `new-api`：

```bash
cp -p \
  /srv/new-api/backups/new-api-release-20260901T132333Z-fc6ebe122e/images.env.before \
  /srv/new-api/env/images.env
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml config -q
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api
```

该命令只恢复应用镜像，不会自动恢复已删除的敏感词 option。若需要恢复发布前的完整策略语义，必须先停止或控制正式写入，再单独审核并从 `sensitive-options.before.csv` 恢复三项 option；若需要完整数据库恢复，则使用 custom-format dump 制定恢复计划并核对对象和行数。当前没有执行数据库恢复演练，不能把镜像回滚描述为完整数据回滚。
