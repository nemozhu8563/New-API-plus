# Tryvalo Zgo / GreenCloud 边缘路由执行及状态记录（2026-08-26 起）

> 当前状态（2026-08-27）：**拓扑修正、CPA v7.2.143 升级与 CPA/CPAMP 双域名拆分均已执行，并通过当前可执行范围内的验收，未触发回滚**。
>
> 当前拓扑：`api.tryvalo.com -> Zgo 64.83.30.150 -> GreenCloud 173.249.203.66`；`cpa.tryvalo.com`、`cpamp.tryvalo.com` 与 `new.tryvalo.com` 均直接指向 GreenCloud `173.249.203.66`。
>
> 最新 Release ID：`cpamp-two-domains-20260827T113329+0800`。原始全量 Zgo 切换记录：`tryvalo-zgo-cutover-20260826T205747+0800`。
>
> 路由原则：**仅 API 入口必须经过 Zgo；CPA、CPAMP 与 Web 入口不经过 Zgo**。线路改善仍需用 TTFT、断流和重连数据与发布前基线对比。

## 0. 最终部署状态

以下状态为本次发布窗口内的最终核验结果，不包含任何 API Key、Cookie、证书私钥或其他 secret。

### 0.1 当前拓扑和服务状态

| 项目 | 最终状态 |
| --- | --- |
| 正式 DNS | Cloudflare DNS-only（灰云）A 记录：`api.tryvalo.com -> 64.83.30.150`；`cpa.tryvalo.com`、`cpamp.tryvalo.com`、`new.tryvalo.com -> 173.249.203.66`；TTL 为 Cloudflare 自动 |
| DNS 验证 | `1.1.1.1`、`8.8.8.8` 均已返回上述最终地址 |
| Zgo Caddy | `v2.11.4`，active，监听 `80/tcp`、`443/tcp`；正式业务仅承接 `api.tryvalo.com`，并保留 `edge-api.tryvalo.com` 运维预检入口 |
| Zgo sing-box | `sing-box-10443` active/enabled；原占用 `443/tcp` 的 `sing-box` inactive/disabled |
| Hysteria2 | 已删除，不再保留原 UDP 服务 |
| GreenCloud Caddy | `v2.11.4`，active；直接承接 `new.tryvalo.com`、`cpa.tryvalo.com` 和 `cpamp.tryvalo.com`，并继续接收 Zgo 的 API 回源 |
| 固定 API 回源 | Zgo 连接 `173.249.203.66:443`，SNI 和 HTTP Host 均为 `origin-api.tryvalo.com`，保持严格证书校验，不依赖回源 DNS |
| GreenCloud 防火墙 | 公网 `80/tcp`、`443/tcp` 保留；`8317/tcp`、`18317/tcp` 不直接暴露公网。由于 CPA/CPAMP/new 直连，不能再把公网 `443/tcp` 收紧为仅 Zgo |
| GreenCloud 主机级隔离 | `api.tryvalo.com`、`origin-api.tryvalo.com` 对非 Zgo 来源返回 HTTP 404；`new.tryvalo.com`、`cpa.tryvalo.com`、`cpamp.tryvalo.com` 正常直接服务 |
| CPA | CLIProxyAPI `7.2.143`，Commit `4b5f1eab`，BuiltAt `2026-08-26T21:32:30Z`；`cliproxyapi` active/enabled，仍只在本机 `8317` 提供后端监听 |
| CPA-Manager-Plus | `v1.12.5`，镜像固定到 digest `sha256:02d16100da1dd3d717061cc314e30b430d3eea44e1019730ecaa9536593e975e`；容器 healthy，`127.0.0.1:18317` 仅供本机 Caddy 访问；入口为 `https://cpamp.tryvalo.com/management.html` |
| 业务依赖 | `new-api`、PostgreSQL、Redis 状态未因本次路由变更或 CPA 二进制升级而迁移；CPA PostgreSQL store 连接正常 |
| CPA 插件和统计面板 | 未安装动态库插件；CPA 官方 Management Center 保留在 `cpa.tryvalo.com`；CPA-Manager-Plus 独立消费统计面板已部署在 `cpamp.tryvalo.com` |
| 本次未部署 | 内容拦截、自定义敏感词逻辑和 CPA 自定义并发控制均未在本次切流中上线 |

### 0.2 配置指纹和回滚材料

| 项目 | 值 |
| --- | --- |
| 当前 Zgo Caddyfile SHA-256 | `3390e52ae4a1a6e8efb8dd3d428186bf41e177b4f57d817f126051cf14602667` |
| 当前 GreenCloud Caddyfile SHA-256 | `fa1e1afd873bdad3ab7f32f2409baa9534ca30de983502cc1a3dc6436642fa52` |
| 当前 CPA-Manager-Plus Compose SHA-256 | `7a00f76ef943a7dd5515e86dd9ca240ae57c20e2cebf4e47322279ed6986ab8a` |
| CPA/CPAMP 双域名拆分回滚目录 | `/root/tryvalo-releases/cpamp-two-domains-20260827T113329+0800`；包含拆分前 Caddyfile 与 Compose |
| 2026-08-27 两台主机回滚目录 | `/root/tryvalo-cutovers/cpa-direct-greencloud-20260827T090049+0800/` |
| Zgo 过渡配置 SHA-256 | `c0f5c3e61694790923820b9756137130d05606891c8f1f9ae145df9c9ac94f39`，已保存为 `zgo.Caddyfile.intermediate` |
| 2026-08-26 Zgo / GreenCloud 原始回滚目录 | `/root/tryvalo-cutovers/tryvalo-zgo-cutover-20260826T205747+0800/zgo-edge` / `/root/tryvalo-cutovers/tryvalo-zgo-cutover-20260826T205747+0800/greencloud-caddy` |
| CPA v7.2.143 官方 Linux amd64 资产 SHA-256 | `9154f460a5684ae82d74f3643d7b3f9c8961659d33058458c9edc044f5f761ba`，与 GitHub Release API digest 及官方 `checksums.txt` 一致 |
| CPA 二进制 SHA-256 | 升级前 `b04f1451df94ff22d848ee468824222863884f64d7d77bec6e78adc1aa29e89e`；升级后 `04e5a1d5397ef06ed3e629d6da51c87986ea8d9ab0062c8c22d76a5c24363cca` |
| CPA 配置 SHA-256 | `64227d50add8ccedb3f313f3786a37487f5e1decb065347804cb1817c57e3211`，升级前后不变 |
| CPA v7.2.143 回滚目录 | `/root/tryvalo-releases/cpa-v7.2.143-20260827T100501+0800`；包含旧二进制、配置、systemd、环境文件、PostgreSQL custom dump 与 pgstore runtime |

### 0.3 发布验收结果

2026-08-27 拓扑修正验收：

- `1.1.1.1` 和 `8.8.8.8` 均返回最终路由：API 为 Zgo，CPA/new 为 GreenCloud。
- `api.tryvalo.com/api/status` 通过 Zgo 返回 HTTP 200；TLS 校验结果为 0。
- `new.tryvalo.com/api/status` 直接从 GreenCloud 返回 HTTP 200；TLS 校验结果为 0。
- `cpa.tryvalo.com/healthz`、`/` 均返回 HTTP 200；无凭据访问 `/v1/models` 返回预期 HTTP 401；TLS 校验结果为 0。
- 将 `api.tryvalo.com` 或 `origin-api.tryvalo.com` 强制解析到 GreenCloud，从非 Zgo 来源访问均返回 HTTP 404，绕过保护有效。
- 公网访问 `173.249.203.66:8317` 超时，CPA 后端端口未暴露；两台 Caddy 均 validate 通过且 active。

2026-08-27 CPA v7.2.143 升级验收：

- `cliproxyapi` 为 active/enabled，进程自 10:05:02（Asia/Shanghai）起运行；loopback `/healthz` 正常。
- `https://cpa.tryvalo.com/healthz` 返回 HTTP 200，TLS 校验结果为 0；`/management.html` 完整返回 HTTP 200。
- Management Center 标题为 `CLI Proxy API Management Center`，HTML SHA-256 为 `68981cdc33ff6293371d186cf9f60fab892c01051cd77974ecde4de0ed1238bd`。
- 无凭据访问 `/v1/models` 与 `/v0/management/config` 均返回预期 HTTP 401；公网 `8317/tcp` 仍不可达。
- PostgreSQL store 可连接；升级前备份与升级后的 `auth_store` 均为 0 行，证明账号库原本就是空库，并非升级清空。
- 启动后日志中 panic/fatal、error level 和插件加载失败均为 0；生产插件目录未发现动态库插件。
- 因 `auth_store` 为空，已配置 API Key 的 `/v1/models` 只能返回空数组，`gpt-5.4` SSE 请求返回 `model_not_found`。真实模型、流式完整性和 usage 结算尚未完成验收，需添加上游账号后补测。

2026-08-27 CPA/CPAMP 双域名拆分验收：

- Cloudflare 新增 DNS-only A 记录 `cpamp.tryvalo.com -> 173.249.203.66`，TTL 自动；`1.1.1.1`、`8.8.8.8` 均已回读到 GreenCloud。
- `cpa.tryvalo.com` 已恢复为纯 CPA：`/healthz` 与官方 `/management.html` 返回 HTTP 200，无凭据 `/v1/models` 返回预期 HTTP 401；持有 CPA Management Key 的本机管理接口返回 HTTP 200。
- `cpamp.tryvalo.com` 全路径反代到 CPA-Manager-Plus：`/health` 与 `/management.html` 返回 HTTP 200，无凭据 `/status` 返回预期 HTTP 401，持有 CPAMP Admin Key 的 `/status` 返回 HTTP 200。
- 两个面板已通过标题区分：CPA 为 `CLI Proxy API Management Center`，CPAMP 为 `CPA Manager Plus`；不再使用路径或 `Authorization` Header 进行同域分流。
- CPAMP Collector 状态为 `running`，mode 为 `auto`，transport 为 `subscribe`，queue 为 `usage`，`lastError=null`，dead letters 为 0。
- CPAMP 容器 healthy、restart count 为 0；CORS 只允许 `https://cpamp.tryvalo.com`，旧 `https://cpa.tryvalo.com` 不再获得 allow-origin 响应头。
- 正式 Let's Encrypt 证书于 11:44:05（Asia/Shanghai）签发成功；两个域名的 HTTPS 校验结果均为 0。`18317` 只监听 `127.0.0.1`，未直接暴露公网。
- 初次签发前因 DNS 尚未创建出现的 NXDOMAIN 已在 DNS 生效后通过强制平滑 reload 恢复；11:45 的两条 `use of closed network connection` 来自本次验收客户端主动超时中断大文件下载，不是上游进程故障。随后完整面板 GET 均成功。

2026-08-26 原始 Zgo 全量切换的业务验收（历史）：

- 当时两个正式域名的 `/api/status` 均返回 HTTP 200；无效 Token 均返回预期 HTTP 401。
- `gpt-5.4` 共完成 4 次真实请求：2 次非流式、2 次 SSE，全部返回 HTTP 200。
- 两次 SSE 均只出现一次 `response.completed`，没有失败事件、重复完成或提前断流。
- 4 次请求均各自只有一条消费记录，没有重复结算。
- 已从两个不同网络来源验证客户端 IP 正常传递，没有全部退化为 Zgo 地址。
- 发布观察窗口最近 15 分钟的 Zgo access log 中 `5xx=0`；两端 Caddy 均 active，未发现新 OOM 或持续告警。
- 发布过程中 Zgo Caddy 曾因 access log 目录权限导致一次 reload 失败，已在正式切流前修复；21:20（Asia/Shanghai）后未见同类新告警。

历史验收请求 ID：

```text
202608261402030717113508268d9d6GAqLOIGb
202608261402066298397548268d9d6q3YMzqdA
202608261402104144185868268d9d6zhNoPhTA
202608261402124690527318268d9d6poLnEflX
```

### 0.4 已知问题和待复核项

- 渠道 29 虽声明支持 `gpt-5.4-mini`，但其上游账号实际不支持。该问题与本次 Zgo 线路切换无关，本次未修改渠道配置。
- “线路是否改善”尚未由发布窗口内的少量请求证明。需要按同模型、同渠道对比发布前后 24 小时的 TTFT、SSE 完整率、异常断流和重连数据。
- 内容拦截、自定义敏感词词库和 CPA 自定义并发控制尚未上线；双域名拆分只解决 CPA 与统计管理面的入口隔离，不等同于危险请求拦截。
- CPA 官方管理面板和 CPA-Manager-Plus 独立消费统计面板均已可用；当前仍没有动态库插件。
- CPA 账号库为空，暂时无法把进程健康、管理面板和鉴权验收等同于真实模型业务验收；需添加上游账号后补做非流式、SSE 和 usage 验证。
- 本次临时 API Key 未写入文档，发布完成后必须在凭据管理端吊销。

### 0.5 当前拓扑约束与回滚顺序

- 只有 `api.tryvalo.com` 允许依赖 Zgo；`cpa.tryvalo.com`、`cpamp.tryvalo.com` 和 `new.tryvalo.com` 的正常路径必须直达 GreenCloud。
- GreenCloud 公网 `443/tcp` 需要为 CPA/CPAMP/new 保持可达。API 隔离由 Caddy 的 hostname + Zgo 来源限制完成，不能再用“公网 443 仅允许 Zgo”的旧规则。
- CPA 后端 `8317/tcp` 必须继续拒绝公网，只允许现有本机/容器桥接访问。
- CPA-Manager-Plus 后端 `18317/tcp` 必须继续只监听 `127.0.0.1`；公网入口只能经过 `cpamp.tryvalo.com:443`。
- 如果 CPA/new 需要回退到 Zgo，必须先恢复 Zgo 回滚目录中的 `zgo.Caddyfile.intermediate` 并验证证书与代理，再修改 DNS；不能先把 DNS 指回已经删除 CPA/new 站点的 Zgo 最终配置。
- API 路由本次未切换；应用、数据库和 Redis 均未迁移，回滚不涉及数据恢复或双写处理。

## 1. 2026-08-26 原始 Zgo 全量切换的目标与边界（历史）

> 本节至第 13 节保留 2026-08-26 原始全量切换的计划、验收和回滚过程，用于审计与历史回退，不代表当前路由。当前生产事实以第 0 节、第 15 节和第 16 节为准；不得直接按历史步骤再次把 CPA/new 接入 Zgo。

本次在同一个维护窗口完成两类变更，但必须按检查点串行执行：

1. GreenCloud Caddy 从现场版本升级到固定目标版本 `v2.11.4`。
2. Zgo 释放 `80/443` 给 Caddy，作为 `api.tryvalo.com` 与 `new.tryvalo.com` 的无状态边缘入口。
3. Zgo 通过固定 IP `173.249.203.66:443` 回源 GreenCloud，TLS SNI 和 HTTP Host 使用 `origin-api.tryvalo.com`，保持证书校验开启。
4. Cloudflare 只提供 DNS-only（灰云）解析；API 业务流量不经过 Cloudflare HTTP 代理。

目标拓扑：

```text
客户端
  -> Cloudflare 灰云 DNS
  -> Zgo Caddy :443
  -> HTTPS 173.249.203.66:443
     SNI/Host = origin-api.tryvalo.com
  -> GreenCloud Caddy
  -> new-api 127.0.0.1:3000
  -> PostgreSQL / Redis / 私有 CPA
```

本次明确不做：

- 不迁移或重建 `new-api`、PostgreSQL、Redis、CPA。
- 不更新 `new-api` 镜像、数据库 schema、渠道配置、重试次数或熔断参数。
- 不修改根域、`www`、MX、NS、Tunnel、CPA/test hostname。
- 不把 `3000`、`3001`、`5432`、`6379`、`8317` 暴露公网。
- 不复制数据库、Redis 或业务写入源，因此不存在数据迁移或双写步骤。
- 不使用公网 IP 证书，不关闭 TLS 证书校验。

## 2. 固定版本和角色（2026-08-26 历史状态）

| 项目 | 最终状态 |
| --- | --- |
| GreenCloud | `nemo-Phoenix` / `173.249.203.66` |
| GreenCloud Caddy | 已从 `2.6.2` 升级到 `2.11.4` |
| Zgo Caddy | `2.11.4` |
| 正式入口 | `api.tryvalo.com`、`new.tryvalo.com` -> `64.83.30.150`，`proxied=false` |
| 回源身份 | `origin-api.tryvalo.com` -> `173.249.203.66`，`proxied=false` |
| 预检入口 | `edge-api.tryvalo.com` -> Zgo，`proxied=false`，只用于运维预检，不承接业务灰度 |
| Zgo Caddy 监听 | `80/tcp`、`443/tcp` |
| sing-box VLESS Reality | 已迁至 `10443/tcp`，active/enabled |
| Hysteria2 | 已删除 |
| DNS TTL | `300s` |

`v2.11.4` 是计划编写日官方最新稳定版本。执行时必须再次核对官方 stable release 和 APT 候选版本；如果版本已变化，不能自动追随最新版本，仍使用 `2.11.4`，除非单独修改并记录本计划。

## 3. 维护窗口和检查点

建议预留 90 分钟操作窗口，保留 24 小时 DNS/配置回滚材料：

| 时间 | 操作 | 检查点 |
| --- | --- | --- |
| T-60 ~ T-45 | 两台主机、DNS、端口、证书、版本只读盘点 | G0：实际状态与计划相符 |
| T-45 ~ T-30 | 备份 GreenCloud Caddy；准备目标/回退二进制或包 | G1：旧版本可以恢复 |
| T-30 ~ T-20 | 升级 GreenCloud Caddy 到 `2.11.4` | G2：原直连 API 完整可用 |
| T-20 ~ T-10 | 迁移 sing-box 端口；安装和配置 Zgo Caddy | G3：代理与 VPN 两条路径都可用 |
| T-10 ~ T0 | 添加 origin/edge hostname；预签正式证书；验证 Zgo -> GreenCloud | G4：全链路预检通过 |
| T0 | `api`、`new` 两条灰云记录连续切向 Zgo | 全量切换 |
| T0 ~ T+15 | TLS、GET、鉴权、非流式、SSE、日志和账单验收 | G5：正式入口通过 |
| T+15 ~ T+60 | 观察错误率、断流、资源和证书 | G6：满足旧 TTL 条件后才允许关闭 GreenCloud 公网 443 |
| T+24h | 复核流量和稳定性，结束快速回滚窗口 | G7：允许清理临时材料 |

这是一次全量切换。`edge-api` 仅用于操作者在切流前验证配置和链路，不做用户流量比例灰度。

## 4. G0：执行前只读盘点

建立 cutover ID，例如：

```text
tryvalo-zgo-cutover-20260826T<HHMMSS>+0800
```

在 GreenCloud 记录但不输出 secret：

```bash
date -Is
hostnamectl --static
uptime
free -h
df -h /
caddy version
dpkg-query -W -f='${Package} ${Version}\n' caddy 2>/dev/null || true
apt-cache policy caddy
systemctl is-active caddy cliproxyapi cloudflared
systemctl cat caddy
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
ss -lntp | grep -E ':(80|443|3000|3001|5432|6379|8317)\b' || true
ufw status numbered
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3000/api/status
```

在 Zgo 记录：

```bash
date -Is
hostnamectl --static
uptime
free -h
df -h /
uname -m
systemctl status sing-box --no-pager
ss -lntup | grep -E ':(22|80|443|8443|10443)\b' || true
ufw status numbered
```

同时记录：

- Zgo 公网入口 IP、Zgo 到 GreenCloud 时实际使用的固定出口 IP。
- Cloudflare 中 `api`、`new` 的记录 ID、A/AAAA 值、TTL、灰云状态；导出或截图留档。
- `origin-api`、`edge-api` 是否已存在；禁止覆盖用途不明的既有记录。
- `dig @1.1.1.1`、`dig @8.8.8.8` 与权威 DNS 的当前 A/AAAA 结果。
- 两个正式域名当前证书主体、签发者和到期时间。
- 当前 5 分钟的请求数、5xx、SSE 请求和异常断开基线。

停止条件：现场版本、安装来源、Zgo IP、DNS 权限、端口占用或 Caddy storage 路径任一无法确认，不进入升级或切流。

不要使用 `ops/greencloud/scripts/host-preflight.sh` 作为本次通过条件。该脚本针对旧的宿主机 PostgreSQL/预部署阶段，与当前 Docker 生产基线冲突。

## 5. G1：备份与 Caddy 回退材料

GreenCloud 上创建仅 root 可读的本次目录，具体路径包含 cutover ID：

```text
/root/tryvalo-cutovers/<cutover-id>/greencloud-caddy/
```

必须保存：

- `caddy version`、`dpkg-query`、`apt-cache policy caddy` 输出。
- 当前 `/usr/bin/caddy` 二进制及 SHA-256。
- `/etc/caddy/`、systemd unit/drop-in、服务环境变量文件的 root-only 备份。
- 从 `systemctl cat caddy` 和运行用户确认后的实际 Caddy storage；不要凭文档假定路径。
- 当前 Caddyfile 的 `caddy validate` 输出。
- 当前包若仍能从仓库下载，保存精确旧版本 `.deb`；否则当前二进制备份是强制回退材料。
- 官方 APT 仓库下载的目标 `.deb`、包版本和 SHA-256。

禁止将证书私钥、环境变量或 storage 内容打印到终端、聊天、Git 或普通日志。

升级前从目标 `.deb` 解包出新二进制，使用新二进制验证当前 Caddyfile。只有目标二进制验证成功，才允许安装：

```bash
<target-caddy-binary> validate --config /etc/caddy/Caddyfile --adapter caddyfile
```

APT 中必须记录完整 Debian 包版本，不能把 Caddy 的上游版本 `2.11.4` 直接猜成安装参数。执行时先用 `apt-cache madison caddy` 找到且只找到一个上游版本为 `2.11.4` 的完整版本字符串，再依次执行：

```text
apt-get download caddy=<完整 Debian 包版本>
dpkg-deb -x <目标 deb 文件> <mktemp 创建的临时目录>
<临时目录>/usr/bin/caddy version
<临时目录>/usr/bin/caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
apt-get install caddy=<同一个完整 Debian 包版本>
```

旧 `.deb` 的文件名、版本和 SHA-256 必须在安装前写入执行记录。若只能使用备份二进制紧急恢复，需同时记录“dpkg 数据库仍显示新版本”的漂移，并在服务恢复后补做旧包重装；不能把复制旧二进制当成已完成的包级回退。

## 6. G2：先升级 GreenCloud Caddy

顺序不能改变：

1. 确认 G1 回退材料完整且有 SHA-256。
2. 使用官方 stable APT 包安装执行时解析出的精确 `2.11.4` 包版本，不运行实验性的 `caddy upgrade`。
3. 核对 `caddy version` 必须为 `v2.11.4`。
4. 再次执行 `caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile`。
5. 确认 `systemctl is-active caddy`、`journalctl -u caddy` 无启动/证书错误。
6. 验证当前尚未切流的两个正式域名仍然返回 HTTP 200。
7. 完成一个经批准的非流式请求和一条完整 SSE 请求，核对请求 ID 与消费日志。

二进制升级需要新进程加载，不能把配置 `caddy reload` 的无中断语义误认为二进制升级也绝对无中断。窗口内接受短暂的服务重启，但持续不可用超过 60 秒立即恢复旧二进制/包、旧配置和旧 storage，并验证直连入口。

升级通过后才进入 Zgo 变更。GreenCloud Caddy 升级失败时，DNS 尚未变化，不得继续切流。

## 7. G3：迁移 sing-box 并安装 Zgo Caddy

1. 备份 sing-box 配置、systemd unit、当前监听和 UFW 规则。
2. 先放行并配置 `10443/tcp`，验证 VLESS Reality 新端口可用。
3. 更新受控客户端配置并完成至少一次真实连接验证。
4. 再释放 sing-box 的 `443/tcp`；确认旧进程没有继续占用 `80/443`。
5. 按本次最终决定删除 Hysteria2，并确认原 UDP 监听和对应服务已清理。
6. 从官方 stable APT 仓库安装固定版本 Caddy `2.11.4`，使用 systemd 原生运行，不安装完整 Docker 业务栈。
7. 为 Caddy access log 配置轮转和磁盘上限；不得记录 Authorization、Cookie、请求体或敏感查询参数。

Zgo 资源通过线：

- Caddy 和 sing-box 均为 active，10 分钟内无重启。
- `available` 内存至少 `200MiB`，无 OOM、持续 swap storm 或磁盘告警。
- 只有预期的 `80/443`、sing-box/Hysteria2、管理端口监听公网。

任一不通过时停止；DNS 保持 GreenCloud，不进入正式切换。

## 8. G4：配置 origin 与边缘回源

### 8.1 GreenCloud origin

1. 创建/确认 `origin-api.tryvalo.com` 灰云 A 记录指向 `173.249.203.66`。
2. GreenCloud Caddy 增加独立 `origin-api.tryvalo.com` 站点，复用当前 `new_api_proxy`，仍只反代 `127.0.0.1:3000`。
3. 因后续会把 GreenCloud `443/tcp` 限制为仅 Zgo 可达，`origin-api` 的 ACME issuer 必须显式设置 `disable_tlsalpn_challenge`，保留 HTTP-01；不能只假设 Caddy 会自动选中 HTTP-01。
4. 在 GreenCloud Caddy 的全局 `servers` 中只信任现场确认的 Zgo 固定出口 IP/CIDR，并启用 `trusted_proxies_strict`。禁止填写 Zgo 入口 IP、任意公网段或 `private_ranges` 作为替代。
5. GreenCloud 转给 `new-api` 时把 `X-Forwarded-For` 重写为 Caddy 已验证的 `{client_ip}`，不要把客户端提供的原始转发链直接传给应用。这样应用继续只需信任同机 Caddy/现有私网 hop，不必在本窗口重启容器扩大 `TRUSTED_PROXIES`。
6. 使用 `caddy validate` 后执行平滑 `caddy reload`，不要为普通配置变更 stop/start。
7. 等待普通域名证书签发，验证证书链、hostname，并确认实际使用的是 HTTP-01。
8. GreenCloud `80/tcp` 保持公网可达，供 `origin-api` 自动续期；`443/tcp` 在 DNS 收敛前仍保持原公网规则。

执行前只读确认容器中的 `TRUSTED_PROXIES`。若现场值为 `none`，或 Caddy 到容器的实际 peer 不在其信任范围，真实客户端 IP 无法在“不重启 new-api”的边界内恢复，此时停止切流并单独制定应用环境变更，不能带病上线。

### 8.2 Zgo edge

Zgo Caddy 的最终配置必须满足：

- `api.tryvalo.com`、`new.tryvalo.com`、`edge-api.tryvalo.com` 使用同一反代片段。
- 上游连接地址固定为 `https://173.249.203.66`，不依赖回源 DNS 解析。
- `tls_server_name origin-api.tryvalo.com`。
- 上游 HTTP `Host` 显式设为 `origin-api.tryvalo.com`；Caddy `v2.11` 对 HTTPS upstream 会自动改写 Host 为 upstream host，而这里 upstream 是 IP，不能依赖默认值。原始 hostname 继续由默认的 `X-Forwarded-Host` 传递。
- 证书校验保持开启；禁止 `tls_insecure_skip_verify`。
- 请求体上限保持 `100MB`。
- Zgo 外层读写超时不得短于 GreenCloud 当前 `600s`，计划值为 `660s`；连接超时 `30s`。
- 不配置 `lb_retries`、POST/SSE 透明重试、缓存或响应压缩。
- 保持当前已验证的 SSE 低延迟配置 `flush_interval -1`；本次不同时改变其取消语义。
- 覆盖或安全构造转发头，GreenCloud 只把 Zgo 固定出口视为可信上游；从一条已知来源请求验证日志中的客户端 IP 没有全部退化为 Zgo IP。

先为 `edge-api.tryvalo.com` 签发普通域名证书，并通过它验证：

```text
客户端 -> Zgo Caddy -> 173.249.203.66:443
TLS SNI/Host origin-api.tryvalo.com -> GreenCloud Caddy -> new-api
```

至少完成：

- `GET /api/status` 返回 HTTP 200。
- 无效 token 返回预期鉴权错误，而不是代理/TLS 错误。
- 一个经批准的有效 token 非流式请求成功。
- 一条 SSE 请求收到完整结束事件。
- GreenCloud/new-api 日志可找到对应 `X-Oneapi-Request-Id`，且一条测试请求只有一条消费/结算记录。

`edge-api` 预检不等于正式流量灰度，不能省略正式域名切换后的再次验收。

### 8.3 正式证书预签，不把签发等待留到切流后

不允许在 DNS 已切到 Zgo 后再等待 `api`、`new` 首次签发证书。采用短时 HTTP-01 challenge handoff：

1. 确认 GreenCloud 当前 `api`、`new` 证书不在续期动作中，且证书到期时间已经留档。
2. GreenCloud 仅对 `/.well-known/acme-challenge/*` 临时反代到 `http://<ZGO_PUBLIC_IP>:80`，其他路径继续进入原 `new_api_proxy`。
3. Zgo 为 `api.tryvalo.com`、`new.tryvalo.com` 加载最终站点配置，并在这两个站点的 ACME issuer 中显式 `disable_tlsalpn_challenge`，强制使用 HTTP-01。
4. 此时公网 DNS 仍指向 GreenCloud；CA 的 HTTP-01 请求由 GreenCloud 转发给 Zgo，业务请求不进入 Zgo。
5. 两张证书签发成功后，用 `curl --resolve` 将两个正式 hostname 直接固定到 Zgo，逐项完成 TLS、GET、鉴权、非流式和 SSE 预检。
6. 从 GreenCloud 删除临时 challenge handoff，validate + reload，并再次确认原直连业务正常；Zgo 保留已签证书和最终配置等待 T0。

任一正式证书无法预签、`curl --resolve` 仍拿不到正确证书，或 challenge handoff 影响 GreenCloud 现有入口时，立即撤销临时路由并停止发布。不得退回“先切 DNS，再等最多 10 分钟签证书”的做法。

## 9. G5：两条正式域名一次性全量切换（2026-08-26 历史步骤）

切换前最后检查：

- GreenCloud `api`、`new` 直连仍为 HTTP 200。
- Zgo `edge-api` 的 GET、鉴权、非流式、SSE 全部通过。
- 两台 Caddy 都是 `v2.11.4`，配置 validate 通过。
- `origin-api` 证书有效，Zgo 回源严格校验成功。
- Zgo 已持有 `api`、`new` 的有效正式证书，两个 hostname 的 `curl --resolve` 全量预检通过。
- Cloudflare 旧记录值和记录 ID 已留档，回滚写入方式已就绪。
- 没有遗留 AAAA 指向错误主机。

然后连续修改：

```text
api.tryvalo.com  A  <GreenCloud-old> -> <ZGO_PUBLIC_IP>  proxied=false
new.tryvalo.com  A  <GreenCloud-old> -> <ZGO_PUBLIC_IP>  proxied=false
```

两条记录应在同一操作批次内连续完成，不做百分比路由。禁止同时改动根域、`www`、Tunnel、MX、NS、其他测试或 CPA hostname。

DNS 修改后：

1. 检查权威 DNS、`1.1.1.1`、`8.8.8.8` 均返回 Zgo。
2. 确认 Zgo 已直接提供 G4 预签的 `api`、`new` 证书；不在此时 reload 未经预检的新配置。
3. 旧 TTL 尚未过期期间，GreenCloud 公网 `443` 暂不关闭，让缓存旧 IP 的客户端继续完成请求；这属于 DNS 自然收敛，不是人为灰度。
4. DNS 已解析到 Zgo 后，任一正式域名连续两次 TLS/GET 失败就立即回滚，不再为首次签发额外等待。

## 10. 正式验收与观察（2026-08-26 历史步骤）

两个域名分别执行，不得只测其中一个：

1. A/AAAA、TLS hostname、证书链和有效期正确。
2. 公网 GET `/api/status` HTTP 200；不能只用 `HEAD`。
3. 管理员原账号可以登录，不重置管理员。
4. CPA channel 解密和 `/v1/models` 授权行为正常。
5. 一个经批准的有效 token 非流式请求成功。
6. 一条 SSE 请求完整结束，无代理缓存、提前 600 秒断流或重复输出。
7. 日志存在应用请求 ID 和上游请求 ID；测试请求只有一次消费/结算。
8. Zgo、GreenCloud Caddy 日志没有持续 TLS、dial、502/503/504 或 reload 错误。
9. Zgo 资源保持 G3 门槛，GreenCloud `new-api`、PostgreSQL、Redis、CPA 状态不变。
10. 从至少两个不同网络/服务器验证，客户端 IP 没有全部被记录为 Zgo。

建议自动观察 60 分钟，按 1 分钟窗口统计：

- 请求总量、2xx/4xx/429/5xx。
- 502/503/504、TLS 和连接 reset。
- 流式请求数、完整结束数、客户端取消与异常断开。
- TTFT P50/P95 和总耗时 P50/P95。
- Zgo CPU、内存、重启、磁盘、RX/TX。

立即回滚触发条件：

- 任一正式域名 TLS 失败或连续两次 GET `/api/status` 失败。
- 5 分钟窗口 5xx 超过 `2%`，且不是已确认的单一上游渠道故障。
- 两条连续 SSE 验收请求提前断开、重复输出或结算异常。
- 管理登录、CPA 私有通路、授权或计费行为异常。
- Zgo Caddy/sing-box 反复重启、OOM、磁盘不足或可用内存持续低于 `200MiB`。
- 客户端来源 IP 全部退化为 Zgo，导致审计或限流语义错误。

本次上线通过条件是“功能与稳定性不退化”。线路是否真正改善，需要次日按同模型、同渠道对比 TTFT、断流和重连数据，不能用单个请求判断。

## 11. G6：收紧 GreenCloud origin（2026-08-26 历史步骤）

至少等待一个切换前的旧 TTL，并且自切换后已经连续观察满 60 分钟，确认两个正式域名均稳定走 Zgo 后：

1. 先增加 `Zgo 固定出口 IP -> GreenCloud 443/tcp` 的 allow 规则。
2. 从 Zgo 验证 `origin-api` HTTPS 回源正常。
3. 再删除 GreenCloud 公网通用 `443/tcp` allow 规则。
4. 保留公网 `80/tcp` 仅用于已强制为 HTTP-01 的 `origin-api` 自动续期。
5. 从非 Zgo 服务器确认 GreenCloud `443` 被拒绝，从 Zgo 确认仍为 HTTP 200。

如果以后需要关闭公网 `80`，另开变更切换 DNS-01；不在本窗口临时加入 Cloudflare DNS 插件和新的 API token。

## 12. 回滚计划（2026-08-26 历史步骤）

### 12.1 DNS/边缘回滚

按以下顺序执行，不能先改 DNS 再恢复 GreenCloud 防火墙：

1. 在 GreenCloud 恢复公网通用 `443/tcp` allow。
2. 用 `curl --resolve` 从外部确认 GreenCloud 上 `api`、`new` 仍可直接提供有效 TLS 和 HTTP 200。
3. 将两条 Cloudflare 灰云 A 记录恢复为 `173.249.203.66`，恢复原 TTL 和原有 AAAA 状态。
4. 用权威 DNS、`1.1.1.1`、`8.8.8.8` 验证解析回退。
5. 验证 GET、管理员登录、CPA、非流式、SSE 和账单日志。
6. 保留 Zgo Caddy、日志和配置用于调查，不在故障中删除证据。

应用和数据库始终在 GreenCloud，因此 DNS 回滚不需要停写、恢复数据库或回灌数据。

### 12.2 GreenCloud Caddy 升级回滚

如果故障可定位到 Caddy `2.11.4`：

1. 先让正式流量回到已确认可服务的入口；必要时进入维护状态。
2. 恢复 G1 保存的旧 Caddy 包或原 `/usr/bin/caddy`，以及旧 `/etc/caddy`、systemd unit/drop-in。
3. 仅在证书/storage 确实因升级损坏时恢复 storage；正常情况下不覆盖更新后的证书状态。
4. 使用旧二进制 validate 旧配置，再启动服务。
5. 验证版本、systemd、80/443、证书、GET、非流式和 SSE。

如果 GreenCloud `2.11.4` 在 DNS 切换前已通过完整直连验收，后续仅 Zgo/DNS 故障时不必自动降级 GreenCloud Caddy。先恢复用户入口，再单独定位版本问题。

### 12.3 sing-box 回滚

如果迁移 `10443/tcp` 后 VPN 路径不可用：

1. 只有在 Zgo Caddy 已停止且 `443` 已释放时，才可将 sing-box 恢复到旧 `443/tcp`。
2. 恢复备份配置、UFW 和客户端端口，验证 Reality；Hysteria2 已删除，不属于默认回滚范围。
3. 若正式 API 已切到 Zgo，不能直接抢占 Caddy `443`；应先完成 DNS 回滚再恢复 sing-box。

## 13. 收尾和次日复核（2026-08-26 历史步骤）

切换后 24 小时内保留：

- Cloudflare 旧/新 DNS 快照和记录 ID。
- 两台 Caddy 的旧/新版本、配置、二进制/包 SHA-256、systemd 状态和有限日志。
- sing-box 旧配置与端口回滚材料。
- cutover 时点前后 60 分钟的状态码、SSE、TTFT、重连和资源统计。

GreenCloud 上 `api`、`new` 的旧证书只能作为其有效期内的直接 DNS 回滚材料；DNS 切到 Zgo 后它们无法继续独立完成域名验证。每日记录剩余有效期，在证书进入续期窗口前另行决定是否建设 DNS-01/ACME challenge handoff 的长期回滚能力，不能把“旧配置还在”误认为“永久可回滚”。

测试结束后吊销本次使用的临时 API Key；不得把它写入本文件、shell history、Caddyfile 或日志。

次日再决定：

- 重连率、SSE 完整率和 TTFT 是否改善。
- Zgo 500GB 套餐按单向还是双向流量计费，是否足够长期承载。
- 是否保留 Zgo 边缘、增加第二边缘节点，或回退为 GreenCloud 直连。
- 是否另开变更处理 `flush_interval -1` 的断开取消语义、渠道权重和短窗熔断；不把这些结论混入本次入口切换。

清理备份、旧二进制、临时证书或日志前需要另行确认。

## 14. 2026-08-26 原始切换执行记录（历史）

```text
Cutover ID: tryvalo-zgo-cutover-20260826T205747+0800
变更批次建立时间: 2026-08-26T20:57:47+08:00（来自 Cutover ID）
执行日期: 2026-08-26（Asia/Shanghai）
GreenCloud Caddy old -> new: 2.6.2 -> 2.11.4
Zgo Caddy version: 2.11.4
GreenCloud Caddy config SHA-256: 500de444ed28642a0fa06a47e48c2588f23d11e30e324338f995ccf93c82c636
Zgo Caddy config SHA-256: 9037672af45256d1e0eef5a15f396cda404243b0d6bf98ff25984682dd2c98e8
sing-box old -> new port: 443/tcp -> 10443/tcp
Hysteria2: deleted
DNS new: api.tryvalo.com / new.tryvalo.com -> 64.83.30.150, proxied=false, TTL=300
GET /api/status: 两个正式域名均为 HTTP 200
有效模型请求: 2 次非流式 + 2 次 SSE，全部 HTTP 200
账单核对: 4 次请求各一条消费记录
客户端 IP: 两个不同网络来源验证通过
发布观察: 最近 15 分钟 Zgo access log 5xx=0；两次 SSE 完整结束
是否触发回滚: 否
24h 复核结论: 待补充 TTFT、断流和重连对比
```

## 15. 2026-08-27 CPA/new 直连 GreenCloud 执行记录

```text
Cutover ID: cpa-direct-greencloud-20260827T090049+0800
执行日期: 2026-08-27（Asia/Shanghai）
最终 DNS: api.tryvalo.com -> 64.83.30.150（Zgo）
最终 DNS: cpa.tryvalo.com / new.tryvalo.com -> 173.249.203.66（GreenCloud）
Cloudflare: 三条记录均为 DNS-only；CPA/new 更新成功并由 1.1.1.1、8.8.8.8 回读确认
Zgo 最终站点: edge-api.tryvalo.com、api.tryvalo.com
Zgo 已删除: cpa_edge_proxy、CPA HTTP-01 临时路由、cpa.tryvalo.com、new.tryvalo.com
Zgo Caddy config SHA-256: 3390e52ae4a1a6e8efb8dd3d428186bf41e177b4f57d817f126051cf14602667
GreenCloud Caddy config SHA-256: c829155c1371161b95dca623468a38c2e249a695a9de813cc7c766f68c857558
Zgo/GreenCloud Caddy: validate 通过，服务 active
CPA TLS: 正式 Let's Encrypt 证书，客户端校验结果 0
GET api.tryvalo.com/api/status: HTTP 200，经 Zgo
GET new.tryvalo.com/api/status: HTTP 200，直连 GreenCloud
GET cpa.tryvalo.com/healthz: HTTP 200，直连 GreenCloud
GET cpa.tryvalo.com/: HTTP 200，直连 GreenCloud
GET cpa.tryvalo.com/v1/models（无凭据）: HTTP 401
GreenCloud API 绕过保护: api.tryvalo.com / origin-api.tryvalo.com 对非 Zgo 来源均为 HTTP 404
CPA 后端公网保护: 173.249.203.66:8317 连接超时，未暴露公网
两台主机回滚/发布快照: /root/tryvalo-cutovers/cpa-direct-greencloud-20260827T090049+0800/
是否触发回滚: 否
范围: 只变更 DNS、TLS、Caddy 和防火墙边界；未上线内容拦截或自定义并发控制
```

## 16. 2026-08-27 CPA v7.2.143 升级执行记录

```text
Release ID: cpa-v7.2.143-20260827T100501+0800
执行时间: 2026-08-27 10:05:02（Asia/Shanghai）
升级前版本: 7.2.47，Commit 00114bec
升级后版本: 7.2.143，Commit 4b5f1eab，BuiltAt 2026-08-26T21:32:30Z
官方资产: CLIProxyAPI_7.2.143_linux_amd64.tar.gz
官方资产 SHA-256: 9154f460a5684ae82d74f3643d7b3f9c8961659d33058458c9edc044f5f761ba
官方校验: GitHub Release API digest、下载文件和 checksums.txt 三者一致
旧二进制 SHA-256: b04f1451df94ff22d848ee468824222863884f64d7d77bec6e78adc1aa29e89e
新二进制 SHA-256: 04e5a1d5397ef06ed3e629d6da51c87986ea8d9ab0062c8c22d76a5c24363cca
配置 SHA-256: 64227d50add8ccedb3f313f3786a37487f5e1decb065347804cb1817c57e3211（升级前后不变）
服务状态: cliproxyapi active/enabled；loopback /healthz 正常
公网验收: /healthz HTTP 200；/management.html HTTP 200；无凭据模型与管理 API HTTP 401；8317/tcp 不可达
PostgreSQL: 连接正常；升级前后 auth_store 均为 0 行
插件状态: 未安装动态库插件；插件加载失败 0
管理面板: 官方 Management Center 可用；独立消费统计面板未部署
日志验收: 启动后 panic/fatal=0，error level=0
业务验收边界: 上游账号库为空；真实模型、SSE 完整性和 usage 结算待添加账号后补验
回滚目录: /root/tryvalo-releases/cpa-v7.2.143-20260827T100501+0800
PostgreSQL 备份: custom dump 已通过 pg_restore --list 验证
是否触发回滚: 否
```

## 17. 2026-08-27 CPA/CPAMP 双域名拆分执行记录

```text
Release ID: cpamp-two-domains-20260827T113329+0800
执行窗口: 2026-08-27 11:33-11:50（Asia/Shanghai）
最终 DNS: cpa.tryvalo.com -> 173.249.203.66（GreenCloud）
新增 DNS: cpamp.tryvalo.com -> 173.249.203.66（GreenCloud）
Cloudflare: 两条记录均为 DNS-only，TTL 自动；1.1.1.1、8.8.8.8 回读正确
入口分工: cpa.tryvalo.com 全路径进入 CPA；cpamp.tryvalo.com 全路径进入 CPA-Manager-Plus
GreenCloud Caddy config SHA-256: fa1e1afd873bdad3ab7f32f2409baa9534ca30de983502cc1a3dc6436642fa52
CPA-Manager-Plus Compose SHA-256: 7a00f76ef943a7dd5515e86dd9ca240ae57c20e2cebf4e47322279ed6986ab8a
CPA-Manager-Plus version: v1.12.5
CPA-Manager-Plus image digest: sha256:02d16100da1dd3d717061cc314e30b430d3eea44e1019730ecaa9536593e975e
CPA-Manager-Plus runtime: healthy，restart count=0，仅监听 127.0.0.1:18317
CORS: 仅允许 https://cpamp.tryvalo.com
TLS: cpamp.tryvalo.com 的正式 Let's Encrypt 证书于 11:44:05 签发；两个正式入口客户端校验结果均为 0
CPA 验收: /healthz HTTP 200；/management.html HTTP 200；无 Key /v1/models HTTP 401；有 Management Key 的管理接口 HTTP 200
CPAMP 验收: /health HTTP 200；/management.html HTTP 200；无 Admin Key /status HTTP 401；有 Admin Key /status HTTP 200
面板标题: CPA 为 CLI Proxy API Management Center；CPAMP 为 CPA Manager Plus
Collector: running，mode=auto，transport=subscribe，queue=usage，lastError=null，dead letters=0
Caddy 日志说明: 11:45 两条 closed connection 为验收客户端主动超时中断大文件下载；随后两个面板完整 GET 均成功
回滚目录: /root/tryvalo-releases/cpamp-two-domains-20260827T113329+0800
是否触发回滚: 否
未上线范围: 内容拦截、自定义敏感词逻辑、CPA 自定义并发控制
```

## 18. 依据

- [GreenCloud 服务迁移 SOP](greencloud-service-migration-sop.md)
- [GreenCloud 迁移执行手册](2026-07-11-greencloud-migration-runbook.md)
- [`ops/greencloud/caddy/Caddyfile`](../../ops/greencloud/caddy/Caddyfile)
- [`ops/greencloud/cpa-manager-plus/compose.yaml`](../../ops/greencloud/cpa-manager-plus/compose.yaml)
- [Caddy 官方安装说明](https://caddyserver.com/docs/install)
- [Caddy 官方命令行说明](https://caddyserver.com/docs/command-line)
- [Caddy 全局 `trusted_proxies` 说明](https://caddyserver.com/docs/caddyfile/options#trusted-proxies)
- [Caddy `tls` / ACME challenge 说明](https://caddyserver.com/docs/caddyfile/directives/tls)
- [Caddy `reverse_proxy` 说明](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
- [Caddy v2.11.4 release](https://github.com/caddyserver/caddy/releases/tag/v2.11.4)
- [CLIProxyAPI v7.2.143 release](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v7.2.143)
- [CPA-Manager-Plus 官方仓库](https://github.com/seakee/CPA-Manager-Plus)
- [CPA-Manager-Plus v1.12.5 release](https://github.com/seakee/CPA-Manager-Plus/releases/tag/v1.12.5)
