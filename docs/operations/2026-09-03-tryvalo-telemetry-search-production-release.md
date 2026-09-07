# Tryvalo telemetry 与搜索入口正式发布状态

## 结果

2026-09-03 09:44:47（Asia/Shanghai），Tryvalo 的站点运行时配置、GA4/Clarity consent gate、`robots.txt` 与 `sitemap.xml` 已发布到 GreenCloud 正式应用。发布使用基线 `e40d88d1535fd315f820af25ce1a2a43ab297e5e` 上的本地 telemetry/SEO 工作树内容；本次没有创建 Git commit 或 push。

正式应用运行 `new-api:new-api-release-20260903T094447Z-e40d88d1535`，镜像 ID 为 `sha256:95f4f247b4e46e275ccdccd7521ca8f3021cf78c18447edcbecff17894d9515d`。仅重建了 `new-api`；PostgreSQL 与 Redis 的容器 ID、启动时间均保持不变。

| 阶段 | 状态 | 证据与边界 |
| --- | --- | --- |
| Planned | 已完成 | 生产 canonical、公开 measurement/project ID、consent 默认值、sitemap 目标、备份与应用级回滚范围在发布前固定。 |
| Production execution | 已完成 | 已切换到目标不可变镜像并仅重建 `new-api`。 |
| Runtime validation | 已完成 | 应用健康、两个 API 入口、首页运行时 payload、canonical、robots 与 sitemap 均已只读验证。 |
| Pre-consent isolation | 已完成 | 初始生产页未观察到 GA4/Clarity 远程资源；analytics 默认拒绝。 |
| Provider receipt | 不属于本次发布健康证明 | GA4/Clarity provider 回读、GSC/Bing provider 状态必须各自独立读取，不能由发布或客户端请求替代。 |
| Rollback | 未执行 | 当前应用健康；保留镜像配置与 Compose 备份。 |

## 发布身份与材料

- 发布 ID：`new-api-release-20260903T094447Z-e40d88d1535`
- 源基线：`e40d88d1535fd315f820af25ce1a2a43ab297e5e`
- 平台：`linux/amd64`
- 正式镜像：`new-api:new-api-release-20260903T094447Z-e40d88d1535`
- 镜像 ID：`sha256:95f4f247b4e46e275ccdccd7521ca8f3021cf78c18447edcbecff17894d9515d`
- 镜像包 SHA-256：`8fbbcbb01bd9ecfb4ab3818c040a2a1a1fd15cc636e7334c86ff9a67193ede7e`
- 备份目录：`/srv/new-api/backups/new-api-release-20260903T094447Z-e40d88d1535`
- 当前 `images.env` SHA-256：`32606ae75a7ac866b6f062d6ce1a8f19b0f84a95bb1170b98f590e86c8ce0284`
- 当前 `new-api.env` SHA-256：`0954b8e2356e442a2dcc29f0d56f4328bd40ba0349cb2ce527abf7e9a6e19ffa`

## 范围与拓扑

- GreenCloud 主机：`nemo-Phoenix`（`173.249.203.66`）
- Compose：`/srv/new-api/compose.yaml`
- 镜像变量：`/srv/new-api/env/images.env`
- 应用配置：`/srv/new-api/env/new-api.env`
- 服务：`new-api`
- 本次没有修改 DNS、Cloudflare、Caddy、Zgo、PostgreSQL、Redis 或防火墙配置。

运行时已精确匹配以下公开配置：

- `SITE_CANONICAL_ORIGIN=https://tryvalo.com`
- `SITE_TELEMETRY_ORIGIN=https://tryvalo.com`
- `GOOGLE_ANALYTICS_ID=G-T2LD0R73QD`
- `CLARITY_PROJECT_ID=ycgor9smow`

`UMAMI_WEBSITE_ID` 在当前正式应用配置中不存在，因此本次没有旧 Umami 脚本绕过新的 analytics consent gate。

## 验证

发布后只读验证结果：

- `new-api`：`running/healthy`，重启次数 `0`。
- `new-api-postgres`：`running/healthy`，容器 ID `068ed5334fd00c628a10387cd53d66048f162b568785c282987996325d8bbefa`，启动时间仍为 `2026-07-11T01:00:44.13346078Z`。
- `new-api-redis`：`running/healthy`，容器 ID `792cc76235b965e3c3b85480b5185bfea9790d9e3141f8f8c96653522c152b4f`，启动时间仍为 `2026-07-12T00:26:12.673497206Z`。
- `https://api.tryvalo.com/api/status`：HTTP `200`。
- `https://new.tryvalo.com/api/status`：HTTP `200`。
- `https://tryvalo.com/`：存在 canonical 与运行时 telemetry payload；首次加载的 analytics consent 为拒绝，未在同意前加载 GA4 或 Clarity 远程资源。
- `https://tryvalo.com/robots.txt`：HTTP `200`，声明 `Sitemap: https://tryvalo.com/sitemap.xml`。
- `https://tryvalo.com/sitemap.xml`：HTTP `200`、`application/xml`，包含 4 个 canonical URL。

这些结果证明发布、配置加载、入口健康与同意前隔离；不证明 GA4/Clarity 已接收数据、搜索引擎已抓取或页面已经收录。

## 发布后的 provider 回读

2026-09-03（Asia/Shanghai），在不改变 GreenCloud、DNS 或应用配置的前提下，分别从 provider 页面复核以下资源。GA4/Clarity 的独立 analytics consent 策略未改变：注册时同意服务协议或隐私政策不授予 analytics consent，未明确 Allow 前不加载第三方 analytics。

| Provider | 已执行/读回 | 证据边界 |
| --- | --- | --- |
| GA4 | 精确 Tryvalo Web stream 已存在：`https://tryvalo.com`，Measurement ID `G-T2LD0R73QD`；先前 Realtime 回读为没有可用数据。 | 当前未获得用户在生产页的明确 analytics Allow，因此 production transport 与新的 Realtime/DebugView 接收侧证据仍待定。 |
| Microsoft Clarity | 精确 Tryvalo project 已存在，Project ID `ycgor9smow`；provider 页面为 `0 / 7` 安装引导，Dashboard、录制和热度图入口均重定向回该页面。 | 不能据此证明 Dashboard/live users 或 Recordings 中已有数据。 |
| Google Search Console | `sc-domain:tryvalo.com` 为当前 Google 账号 Owner；`https://tryvalo.com/sitemap.xml` 在 GSC 读回为成功，含 4 URL。 | sitemap 成功不证明单个页面已抓取或收录。 |
| Bing Webmaster | 只选择 `https://tryvalo.com/` 进行 GSC Import，未导入同页列出的其他站点；Bing provider 页面随后读回该精确站点。 | 这证明 Bing 站点已导入，不替代 sitemap、抓取或收录证据。 |
| Bing sitemap | 向 `https://tryvalo.com/sitemap.xml` 提交一次；provider 原始状态为 `Submitted / Processing`，当前显示 1 个已知 sitemap、0 errors、0 warnings 和 `0 / -` discovered URLs。 | 这是 receipt/processing 证据。不得重提，也不表示 Bing 已抓取、收录、排名或产生流量。 |
| IndexNow | 未请求，未执行。 | 不属于本次授权范围。 |

观察队列：等待用户主动 Allow analytics 后，再分别只读查看 GA4 Realtime/DebugView 与 Clarity Dashboard/Recordings；在 Bing 的 `Submitted / Processing` 异步状态变化后，只读回读 crawl/indexing。无需重复提交 sitemap。

## 回滚

本次未触发回滚。应用级回滚材料位于上述备份目录；恢复该目录中的 `images.env.before` 后，仅重建 `new-api`。该路径只回退应用镜像与相应配置，不删除或回退 GA4、Clarity、GSC 或 Bing 的 provider 资源，也不等同于数据库恢复演练。
