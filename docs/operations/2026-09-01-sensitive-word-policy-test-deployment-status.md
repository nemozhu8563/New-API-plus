# Tryvalo 敏感词分级策略测试发布状态

## 结果

精确提交 `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75` 已于 2026-09-01 发布到 GreenCloud 独立测试实例。测试环境确认高风险词和 NSFW 词会在本地返回 HTTP `403 content_policy_violation`，审计词不会被本地策略阻断并会记录 `category=audit action=audit`。测试渠道的上游凭据当前返回 HTTP `401 Invalid API key`，因此本次没有取得普通请求或审计词请求的成功模型响应；这不影响本地策略分流结论，但意味着“允许路径成功生成”仍未验收。

生产环境未发布本次提交，生产容器、镜像、启动时间和公网入口在测试前后保持不变。

| 阶段 | 状态 | 证据 |
| --- | --- | --- |
| Planned | 已完成 | 发布提交、不可变镜像、测试服务、备份、接口用例和回滚边界均在执行前确定。 |
| Test execution | 已完成 | 仅更新 `/srv/new-api-test/compose.yaml` 并重建 `new-api-test`；PostgreSQL、Redis 和生产服务未重建。 |
| Policy validation | 已完成 | `high_risk`、`nsfw` 阻断响应及三类策略日志均符合预期；`audit` 请求通过本地策略并到达上游。 |
| Allowed-path generation | 待定 | 普通请求和审计词请求均被上游以 HTTP `401 Invalid API key` 拒绝，没有取得 HTTP `200` 模型响应。 |
| Temporary channel cleanup | 已完成 | 渠道 `1` 从禁用临时改为启用，验收后恢复为禁用，并再次只重建 `new-api-test`。 |
| Production execution | 未执行 | 生产仍运行 `f96bf33b80dfeca9b025a94651fb68db492dc8a7` 对应镜像。 |
| Rollback | 未执行 | 当前测试镜像健康；发布前 Compose 和测试库备份保留在下述路径。 |

## 发布身份

- 应用提交：`fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`
- 提交意图：`Reduce sensitive-word false positives with tiered policy`
- 本地分支状态：`main` 比 `origin/main` ahead `5`；本次没有 push。
- 构建来源：精确提交归档；工作树中无关的 `.gitignore` 和运维状态文档改动未进入镜像。
- 平台：`linux/amd64`
- OCI revision：`fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`
- 测试镜像：`new-api:new-api-test-20260901T064123Z-fc6ebe122e`
- 镜像 ID：`sha256:268d7cff740cfdbb7b97d9f3f3d8b651c8633d2eeb60c4bc92a869b246a34876`
- 传输包：`new-api-test-20260901T064123Z-fc6ebe122e.tar.gz`
- 传输包大小：`71,820,648` bytes
- 传输包 SHA-256：`c3e3d60bb97eb4899d0aba1c0a43520b258404cd40bbb04a95a560d8505c0215`
- GreenCloud 导入目录：`/opt/new-api/import/new-api-test-20260901T064123Z-fc6ebe122e/`

GreenCloud 在加载镜像前直接回读了相同 SHA-256，且 `gzip -t` 通过。远端镜像的 ID、`amd64/linux` 平台和 OCI revision 与上表一致。

## 变更范围

- 将提示词敏感词策略拆分为 `high_risk` 阻断、`nsfw` 阻断和 `audit` 仅审计三层，阻断优先于审计。
- 将高歧义、易误杀的词移入仅审计列表；`淫威` 在本次真实接口验收中用于确认不再被本地阻断。
- 保留兼容选项 `SensitiveWords` 作为 NSFW 阻断列表，新增 `SensitiveWordsHighRisk` 和 `SensitiveWordsAudit`。
- 管理前端展示三份独立列表，并按高风险、审计、NSFW、Prompt 开关、总开关的顺序保存。

本次没有数据库 schema、Stripe、DNS、Cloudflare、Caddy、Zgo、CPA、防火墙或生产配置变更。测试库没有写入三项敏感词 option，运行时继续使用镜像内置词表。

## 发布前验证

- `go test ./setting ./service ./model ./controller -count=1`：通过。
- `bun run typecheck`：通过。
- `bun run test`：通过，`80` files / `314` tests。
- `bun run build`：通过。
- 三份默认词表共有 `2,094` 个互斥有效词：`475` 个高风险、`548` 个 NSFW、`1,071` 个仅审计。

## 测试部署

- Compose：`/srv/new-api-test/compose.yaml`
- 服务：`new-api-test`
- 发布前镜像：`new-api:new-api-test-20260901T062743Z-89e4d3a911`
- 发布前镜像 ID：`sha256:7cc64698a07c88021fa93b0d58a9d6f50c0a35745e279f000c826758f6f8439c`
- 当前镜像：`new-api:new-api-test-20260901T064123Z-fc6ebe122e`
- 当前 Compose SHA-256：`bdf10662ac194507808d158e7226caf6fac9815ca688567f42e16f0d4942f115`
- 初次发布后的应用启动时间：`2026-09-01 15:06:02 CST`（`2026-09-01T07:06:02Z`）
- 接口验收清理后的应用启动时间：`2026-09-01 15:24:25 CST`（`2026-09-01T07:24:25Z`）
- 最终容器 ID：`fbb54eabfc5eb77e55414c74443565743df813c2a3d88eadbf1fea29f1504580`
- 最终状态：`running`、`healthy`、重启次数 `0`

只重建测试应用服务：

```bash
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api-test
```

## 备份与回滚材料

备份目录：`/srv/new-api-test/backups/sensitive-policy-20260901T064123Z-fc6ebe122e`

| 文件 | SHA-256 | 校验 |
| --- | --- | --- |
| `compose.yaml.pre-sensitive` | `1c507ff09cc5922cf88c8b2a2f3376f5aeae6ee5151345f1176f2e2dd67ef433` | 发布前 Compose；解析后指向 `new-api-test-20260901T062743Z-89e4d3a911`。 |
| `newapi_test.pre-sensitive.dump` | `53b8d18204d8a671850cc24f86078a8d7be6d825bd31e7cf239ca20e62f49cd4` | PostgreSQL custom-format 测试库备份。 |
| `restore.pre-sensitive.list` | `1212b0eee9d286e84f535f8c88f4e6a8f8238bff0423f7c28a5658a0e8f8eb95` | `pg_restore --list` 通过，共 `534` 行。 |

发布前 `SensitiveWords`、`SensitiveWordsHighRisk`、`SensitiveWordsAudit` 在测试库中均没有持久化行，因此本次未执行词表数据迁移，也没有词表值需要回写。

## 接口验收

为让 distributor 选择到唯一目标渠道，测试库渠道 `1`（`my-cpa-codex`）从 `status=2` 临时改为 `status=1`，只重建 `new-api-test` 后发出四次 `gpt-5.4-mini`、`max_tokens=1` 请求。请求完成后，无论验收结果如何都执行清理：渠道恢复为 `status=2`，并再次只重建测试应用。令牌和渠道密钥未写入日志或本文档。

| 用例 | HTTP / 错误码 | 策略日志 | 结论 |
| --- | --- | --- | --- |
| 普通文本 | `401 bad_response_status_code` | 无策略命中 | 请求到达上游，但上游返回 `Invalid API key`；不能证明成功生成。 |
| 审计词 `淫威` | `401 bad_response_status_code` | `category=audit action=audit` | 本地策略已放行并到达上游；误杀回归通过，但成功生成待定。 |
| NSFW 词 `成人色情` | `403 content_policy_violation` | `category=nsfw action=block` | 本地硬阻断通过。 |
| 高风险词 `炸弹制作` | `403 content_policy_violation` | `category=high_risk action=block` | 本地硬阻断通过。 |

四次请求对应的策略日志计数为 `audit=1`、`nsfw=1`、`high_risk=1`。两条允许路径均产生渠道 `1`、模型 `gpt-5.4-mini` 的错误日志，错误摘要为 `status_code=401, Invalid API key`；这把失败边界定位在上游认证，而不是本地敏感词阻断。

最终回读：

- 渠道 `1`：`status=2`。
- `http://127.0.0.1:3001/api/status`：HTTP `200`。
- `https://test.tryvalo.com/api/status`：HTTP `200`。
- 测试容器镜像和镜像 ID 与发布身份一致，状态 `running/healthy`、重启次数 `0`。

## 生产不变性

验收结束后的生产回读与发布前基线一致：

- 容器 ID：`ed9f0919b4d91a66ed4068c4e9e94ddceb6deb2b269b1179b4bee881e4fc8438`
- 镜像：`new-api:new-api-release-20260830T025223Z-f96bf33b80`
- 镜像 ID：`sha256:032ba62c4df47fe53bb43e5b8894f0ecca0c9d23ae09d6c79e123aa42c820f1a`
- 启动时间：`2026-08-30T03:09:03.380151452Z`
- 状态：`running/healthy`、重启次数 `0`
- `http://127.0.0.1:3000/api/status`：HTTP `200`
- `https://api.tryvalo.com/api/status`：HTTP `200`

生产 Compose、PostgreSQL、Redis、DNS、Cloudflare、Caddy、Zgo、CPA 和防火墙均未修改。生产仍运行旧敏感词实现，生产 option 未迁移。

## 已知限制

- 测试渠道 `1` 的上游凭据无效，因此本次只确认审计词越过本地策略边界，未确认它能成功得到模型响应。
- 测试环境所有渠道在验收后仍为禁用状态；恢复一个可用测试渠道前，允许路径 HTTP `200` E2E 保持待定。
- 本次是测试环境发布，不是生产发布。生产敏感词行为没有改变。
- 健康接口 HTTP `200` 只证明应用与入口健康，不扩展为全部模型、渠道或生产策略已验收。

## 回滚

如需回滚测试应用，恢复发布前 Compose 并只重建 `new-api-test`：

```bash
cp -p \
  /srv/new-api-test/backups/sensitive-policy-20260901T064123Z-fc6ebe122e/compose.yaml.pre-sensitive \
  /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api-test
```

该回滚会恢复应用镜像，但不会恢复数据库。当前发布没有 schema 或持久化敏感词 option 变更；完整测试库恢复必须停止测试写入并单独审核，不能把应用镜像回滚当作数据库恢复。
