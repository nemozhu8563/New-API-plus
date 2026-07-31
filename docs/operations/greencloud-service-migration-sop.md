# GreenCloud 服务迁移 SOP（new-api / CPA）

> 当前目标（2026-07-31）：GreenCloud 是唯一有效的运行、测试、发布和回滚环境。GCP `sub2api-prod` 及其旧测试部署链路已完全弃用，不得用于部署、回滚或健康检查；历史迁移资料仅作审计记录。

> 版本：v1（2026-07-12）
> 适用范围：将 `new-api`、PostgreSQL、Redis、宿主机 CLIProxyAPI（CPA）及可选 CPA keeper 迁入或迁出 GreenCloud；也适用于 GreenCloud 上的常规 `new-api` 镜像发布。
> 当前生产基线与实际端口请先读：[GreenCloud 迁移执行手册](2026-07-11-greencloud-migration-runbook.md)。本 SOP 是流程，不替代现场核验。

## 0. 不变规则

1. **先盘点、后复制、再切流。** 不从旧文档、容器名或口头记忆直接执行。
2. **数据不走终端输出。** 大库导出先落盘、生成哈希，必要时用 `age` 加密；传输使用可续传文件方式。禁止复制粘贴 dump、token、DSN 或完整配置。
3. **CPA 始终私有。** 它只能绑定宿主机 loopback，必要时只向受控 Docker bridge 暴露；绝不将 `8317`、数据库或 Redis 打开到公网。
4. **镜像只在本地构建。** 目标为 `linux/amd64`，GreenCloud 只做校验、`docker load` 和 Compose 重建，不在生产机编译。
5. **外部动作必须批准。** DNS、Cloudflare、WAF/Access、付费上游验证、停旧机和删除备份均需记录批准、时间和回滚窗口。
6. **避免双写。** 数据迁移或切流时，旧端停止写入后才做最终导出；新端未验收前不得成为正式写入源。

## 1. 迁移分类与完成定义

| 类型 | 范围 | 最小完成定义 | 默认回滚 |
| --- | --- | --- | --- |
| A. 应用发布 | 仅 `new-api` 不可变镜像 | 本机与正式入口 GET `/api/status` 正常，依赖未重建 | 切回上一已验证镜像 |
| B. 服务迁移 | 新主机、镜像、配置、数据库、Redis、CPA | 数据计数/哈希、私有网络、管理登录与端到端验证均通过 | 恢复旧入口和旧写入源 |
| C. 入口切流 | DNS、Tunnel、Caddy、Cloudflare 策略 | 新入口公网验证通过且旧端停止写入 | 还原原 DNS / Tunnel 指向 |
| D. keeper 启用 | CPA keeper 定时任务与通知 | dry-run、通知范围、幂等与重启策略确认 | 停止 keeper，不影响 API/CPA |

一次变更可以包含多种类型，但每一类都要单独通过其完成定义。当前 GreenCloud 基线中，`api.tryvalo.com` 与 `new.tryvalo.com` 是 DNS-only + Caddy 直连；根域和 `www` 仍走 Tunnel。不要将这些入口混为同一切换动作。

## 2. 变更单与决策门

执行前建立 release / cutover ID，例如 `new-api-release-YYYYMMDDTHHMMSSZ`，记录：

- 范围：A/B/C/D；源端与目标端；是否涉及数据迁移。
- 目标代码提交、镜像 tag/digest、配置版本、预计窗口和负责人。
- 备份包清单、SHA-256、存放位置、加密状态和保留期限。
- 验收项、允许的异常范围、回滚触发条件和回滚负责人。
- 已获批准的外部操作：停写、DNS/Cloudflare 修改、付费上游测试、删除旧资源。

| 门 | 必须满足 |
| --- | --- |
| G0：盘点 | 源端和目标端的服务、数据、入口、监听、防火墙及版本均有只读记录 |
| G1：可恢复 | 旧镜像、配置、数据库/Redis/CPA 数据备份及校验结果齐全 |
| G2：目标就绪 | GreenCloud 私有网络、磁盘、Docker/Compose、Caddy、UFW、证书与健康检查就绪 |
| G3：数据一致 | 最终数据包已校验、恢复计数对齐，旧端已停止业务写入 |
| G4：可切流 | 本机、正式域名、管理员登录、CPA 通路和经批准的业务验证都通过 |
| G5：可收尾 | 观察期结束、无账单/数据异常，才可停旧机或清理迁移证据 |

任何一个门未通过，都停止在当前阶段；不以“先切过去看看”代替验收。

## 3. 阶段一：只读盘点（G0）

### 源端

记录但不泄露敏感信息：

- 主机、部署目录、Compose 版本、容器/服务状态、镜像 digest。
- `new-api`、PostgreSQL、Redis、CPA、keeper 的数据目录和恢复方式。
- 监听地址、Docker 网络、UFW/安全组、Caddy / Tunnel / DNS / Cloudflare 策略。
- 数据基线：关键表行数、日志截止点、Redis 数据库键数（不记录键值）。
- 当前 `/api/status`、管理员登录与 CPA 通路结果，只记录状态码、时间、请求 ID 和计数。

### GreenCloud 目标端

在 GreenCloud 上做只读检查：

```bash
hostnamectl --static
uptime
free -h
df -h /
systemctl is-active caddy cliproxyapi cloudflared
docker compose version
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
ss -lntp | grep -E ':(80|443|3000|5432|6379|8317)\b' || true
ufw status numbered
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3000/api/status
```

当前基线要求：`new-api` 仅监听 `127.0.0.1:3000`；CPA 仅监听 `127.0.0.1:8317`，私有 bridge 才可额外使用 `172.17.0.1:8317`；公网只允许 SSH 及 Caddy 必要的 `80/443`。实际 bridge 和子网必须从 `docker network inspect new-api-backend` 读取。

## 4. 阶段二：构建、备份与传输（G1）

### 本地构建不可变镜像

在干净工作区固定代码提交和 release ID 后构建。以下为通用形式；以当前项目 Dockerfile / 构建脚本为准：

```bash
release_id='new-api-release-<timestamp>-<short-sha>'
docker buildx build --platform linux/amd64 -t "new-api:$release_id" --load .
docker image inspect "new-api:$release_id" --format '{{index .RepoDigests 0}}' || true
docker save "new-api:$release_id" | gzip > "$release_id.tar.gz"
shasum -a 256 "$release_id.tar.gz" > "$release_id.tar.gz.sha256"
```

先在本地做与生产相关的构建/测试，再传输。未通过测试的镜像不能借由“先部署到 GreenCloud 测一下”进入生产路径。

### 数据与配置包

按本次范围选择包；每个包都先落到源端磁盘，再生成 SHA-256：

| 包 | 何时需要 | 要点 |
| --- | --- | --- |
| `new-api` PostgreSQL | B 类迁移、schema 风险发布 | 使用一致性逻辑备份；记录关键表计数 |
| Redis RDB/AOF | B 类迁移或缓存状态必须保留 | 先确认持久化完成；不复制运行时临时输出 |
| CPA 数据 / 配置 | 迁移 CPA 时 | 仅导出恢复所需状态，不在日志中打印管理 key |
| keeper runtime / 配置 | 启用或迁移 keeper 时 | 先决定是否需要；当前 keeper 未启用不能默认带入 |
| `new-api` 配置 | B 类迁移 | 密文传输；目标端使用独立密码，不复用旧环境口令 |
| Compose / Caddy / systemd / UFW 摘要 | 每次迁移 | 只记录非敏感版本；含 secret 的文件加密保存 |

大库或不稳定链路使用可续传传输，不让 dump 穿过交互终端：

```bash
shasum -a 256 <artifact> > <artifact>.sha256
age -r '<recipient>' -o <artifact>.age <artifact>
rsync --partial --append-verify <artifact>.age root@173.249.203.66:/opt/new-api/import/<release-id>/
```

`<recipient>`、密码、私钥、DSN、token 只能由受控密钥管理或服务器受限文件提供，不能写进命令历史、仓库或 Obsidian。

## 5. 阶段三：目标端预部署与数据恢复（G2 / G3）

1. 创建受限导入目录，记录目标机磁盘余量和导入包 SHA-256。
2. 先加载镜像并核对 tag/digest，再以现行 Compose 的**唯一配置源**更新镜像引用；不要临时同时改 Compose、环境变量和镜像 tag。
3. 对 B 类迁移：先停止目标端业务写入；恢复 PostgreSQL、Redis、CPA 状态和必要配置；再恢复依赖服务与 `new-api`。
4. 比较源端最终基线与目标端恢复结果：关键表行数、日志截止计数、Options/Channels/Tokens 数量和 Redis 数据库计数。任何差异必须解释并记录。
5. 确认目标端使用自己的数据库/Redis/应用口令；旧环境共享口令不得复制为新常态。

常规 A 类发布不应重建 PostgreSQL 或 Redis。确认依赖均健康后，仅替换 `new-api`：

```bash
cd /opt/new-api
docker compose config -q
docker compose up -d --no-deps --no-build --force-recreate new-api
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
```

如果本次涉及 schema migration、数据修复或需要重建依赖，必须在变更单中单列步骤、备份、影响面和回滚方法；不得将它们藏在普通镜像发布命令中。

## 6. 阶段四：私有网络与入口验收（G4）

### 服务和 CPA 边界

```bash
systemctl is-active caddy cliproxyapi cloudflared
ss -lntp | grep -E ':(80|443|3000|5432|6379|8317)\b' || true
ufw status numbered
docker network inspect new-api-backend
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3000/api/status
docker exec new-api sh -lc 'curl -sS -o /dev/null -w "%{http_code}\n" http://host.docker.internal:8317/'
```

CPA 容器内请求只用来证明私有通路可达；预期可为已认证成功、`401` 或 `403`，取决于现场认证策略。它不能是超时、拒绝连接或公网路由。不要打印 CPA 响应正文。

若 `new-api-backend` 被重建，先重新计算 Docker bridge、子网和网关，再更新 UFW；禁止沿用历史 `br-*` 接口名。任何时候都不能为方便排障把 `8317` 改为 `0.0.0.0`。

### 正式入口

对 API 域名使用 **GET**，不要只用 `curl -I`：该服务的 `HEAD /api/status` 可能与 GET 路由行为不同。

```bash
for host in api.tryvalo.com new.tryvalo.com; do
  dig @1.1.1.1 +short "$host" A
  curl -fsS -o /dev/null -D - "https://$host/api/status" \
    | awk 'BEGIN{code="";server=""} /^HTTP\//{code=$2} tolower($1)=="server:"{server=$2} END{print code, server}'
done
```

当前正式基线：`api.tryvalo.com`、`new.tryvalo.com` 应解析到 `173.249.203.66`，返回 HTTP 200 且 `server: Caddy`；根域与 `www` 仍经 Tunnel。DNS、Cloudflare proxy、Tunnel ingress、WAF/Access/Turnstile 修改必须作为 C 类变更单独批准和验收。

### 业务验收

按风险由低到高执行，并只记录状态、计数和请求 ID：

1. 管理员使用**已迁移的原账号**登录，不创建或重置管理员。
2. 验证 CPA channel 解密和 `/v1/models` 的授权行为。
3. 使用专用测试 token 完成一个无付费或已批准成本的非流式请求。
4. 在成本、上游和观察范围获批准后，验证一条 SSE / 长请求，并检查账单日志。
5. 如 keeper 在范围内，先 dry-run，再验证通知目标与冷却策略；未批准前保持停止。

## 7. 阶段五：切流、观察和回滚

### 切流顺序

1. 记录旧入口、TTL、Tunnel / DNS 记录和源端运行状态。
2. 停止旧端新写入，完成最终导出和 G3 数据一致性核对。
3. 获得 C 类批准后，按变更单更新指定 hostname；不要因切 API 而顺手改 MX、NS、`send.tryvalo.com`、CPA hostname 或无关测试域名。
4. 逐项执行入口和业务验收，记录切换时间与结果。
5. 旧端只保留为回滚源，观察期内不恢复它的业务写入。

### 立即回滚条件

- 数据计数不一致、账单异常或确认存在双写。
- 管理员无法登录、CPA 私有通路失败、授权行为异常。
- 正式域名错误指向、TLS 失败、Caddy 无法服务或持续异常。
- 已批准的关键请求在新端失败，且不能在窗口内安全修复。

回滚顺序：停止 GreenCloud 写入 → 将正式入口恢复到旧端 → 验证旧端健康与写入 → 保留 GreenCloud 日志、镜像、哈希和数据库快照供核对。未经明确数据修复方案，不把 GreenCloud 的数据回灌旧端。

## 8. 收尾（G5）

观察期结束后，才可执行以下需要单独批准的动作：

- 停止或销毁旧环境、撤销旧入口和旧密钥。
- 删除已确认不需要的 rehearsal 容器、临时导入包和明文备份。
- 轮换曾跨环境使用过的数据库、Redis、CPA、应用或通知凭据。

最终记录应包含 release/cutover ID、提交、镜像 digest、包哈希、数据计数、入口状态、验证时间、异常与处理、回滚窗口结束时间。更新项目运行手册和 Obsidian 当前状态页；历史方案只标记为历史，不重写为当前事实。

## 9. 一页执行清单

- [ ] 建立 release / cutover ID、范围、负责人和明确批准。
- [ ] 完成源端/目标端只读盘点（G0）。
- [ ] 本地构建 `linux/amd64` 镜像；测试通过；镜像包和 SHA-256 已生成。
- [ ] 配置与数据包已落盘、加密、可续传传输，并在目标端验证哈希（G1）。
- [ ] GreenCloud 服务、监听、UFW、Caddy、证书和磁盘检查通过（G2）。
- [ ] 如涉及数据：旧端停写、最终导出、恢复计数一致（G3）。
- [ ] `new-api`、PostgreSQL、Redis 健康；CPA 保持私有且 Docker 通路可达。
- [ ] 本机 GET `/api/status`、正式域名 GET `/api/status`、TLS 与入口路径通过。
- [ ] 管理登录、授权行为及经批准的业务/SSE 测试通过（G4）。
- [ ] 切流后持续观察；旧端保持只读回滚源。
- [ ] 观察期结束、凭据轮换与清理获批准后收尾（G5）。
