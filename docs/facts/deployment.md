# 部署和运行事实

本文件记录部署方式、环境、数据库、迁移、备份、运维和回滚约束。不要写尚未验证的部署方案为当前事实。

## 环境

| 环境 | 当前状态 | 入口或地址 | 确认来源 | 最近确认 |
| --- | --- | --- | --- | --- |
| 本地 | 待定：仓库提供 Compose 后端依赖和 Rsbuild 前端启动入口，本次未启动服务 | 后端默认 `http://localhost:3000`；前端开发默认 `http://localhost:5173` | `makefile`、`docker-compose.dev.yml` | 2026-08-31 |
| CI | 已确认：PR workflow 配置 root/relaykit vet、build、test 与前端 typecheck、test；远端最近一次运行状态待定 | GitHub Actions | `.github/workflows/ci.yml` | 2026-08-31 |
| 预览 | 不适用：当前仓库未发现独立 preview 环境配置或 preview 部署 workflow | 不适用 | `.github/workflows/`、Docker 与项目配置扫描 | 2026-08-31 |
| 测试 | 已确认（2026-09-01 15:24～15:27，Asia/Shanghai）：GreenCloud 测试应用运行敏感词分级提交 `fc6ebe122e` 的不可变 `linux/amd64` 镜像，容器 `running/healthy`、重启次数 `0`；策略分级接口验收已完成，允许路径成功生成仍待定 | `test.tryvalo.com` | GreenCloud Docker/PostgreSQL 回读；测试接口验收；当前公网 HTTP | 2026-09-01 |
| 生产 | 已确认（2026-09-01 21:28～21:40，Asia/Shanghai）：GreenCloud 正式应用运行敏感词分级提交 `fc6ebe122e` 的不可变 `linux/amd64` 镜像，容器 `running/healthy`、重启次数 `0`；PostgreSQL、Redis 未重建 | `api.tryvalo.com`、`new.tryvalo.com` | GreenCloud Docker/PostgreSQL 回读；备份校验；当前公网 HTTP | 2026-09-01 |

## 当前远端快照

- 2026-09-01 21:40（Asia/Shanghai）在 GreenCloud 主机 `nemo-Phoenix` 现场回读：正式 `new-api` 运行 `new-api-release-20260901T132333Z-fc6ebe122e`，镜像 ID `sha256:268d7cff740cfdbb7b97d9f3f3d8b651c8633d2eeb60c4bc92a869b246a34876`，OCI revision 为 `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75`。该镜像 ID 与测试 `new-api-test-20260901T064123Z-fc6ebe122e` 相同。
- Stripe 订阅账期修复曾以不可变测试镜像 `new-api:new-api-test-20260901T062743Z-89e4d3a911` 完成 Sandbox E2E；镜像 ID 为 `sha256:7cc64698a07c88021fa93b0d58a9d6f50c0a35745e279f000c826758f6f8439c`，传输包 SHA-256 为 `03a091d122fa681c05e374b84f20226fa262e911582dd2ca6468b4dfc87f3e72`。当前测试镜像 `fc6ebe122e` 是该提交的后继版本并包含同一修复，但支付 E2E 的证据归属于 `89e4d3a911` 镜像。
- 测试和正式本机 `127.0.0.1:3001`、`127.0.0.1:3000` 的 `/api/status` 均返回 HTTP `200`。测试容器因临时渠道启用和清理各重建一次，最终启动时间为 `2026-09-01T07:24:25.939107805Z`；正式应用于 `2026-09-01T13:28:19.177904701Z` 启动，最终为 `running/healthy`、重启次数 `0`。
- 生产 `new-api-postgres` 与 `new-api-redis` 均为 `running/healthy`、health failing streak `0`、重启次数 `0`。PostgreSQL `pg_isready` 返回 accepting connections，并完成了生产库只读聚合；Redis 最近五次容器健康检查均以 exit `0` 完成。以上证明容器和所执行的依赖检查，不等于数据库全量数据正确或缓存业务语义完整。
- 当前公网 `api.tryvalo.com`、`new.tryvalo.com` 和此前测试验收的 `test.tryvalo.com` 的 `/api/status` 均返回 HTTP `200`；本次正式发布没有修改既有边缘路径。
- 当前边缘路径已现场确认：`api.tryvalo.com` 解析到 Zgo `64.83.30.150`，Zgo Caddy `v2.11.4` 为 active，并固定反代到 GreenCloud `173.249.203.66`，Host 与 TLS SNI 均为 `origin-api.tryvalo.com`；GreenCloud 只允许该 Zgo 地址访问 `origin-api.tryvalo.com`，再转发到 `127.0.0.1:3000`。`new.tryvalo.com` 直接解析到 GreenCloud，并由同一 GreenCloud Caddy 转发到生产应用。
- 本次正式发布仅重建 `new-api`，`new-api-postgres` 与 `new-api-redis` 的容器 ID 和启动时间保持不变。正式库发布前仅存在旧 `SensitiveWords` option，其值等于旧默认词表减去 `代理`；备份后删除三项敏感词 option，当前总计 `0` 行，因此正式运行时使用镜像内置三层词表。启动日志错误模式匹配数为 `0`。生产业务请求 E2E 未执行，没有读取或使用已有生产 token。
- 正式发布身份、option 处理、备份、健康证据和业务验证边界见 [2026-09-01 敏感词分级策略正式发布状态](../operations/2026-09-01-sensitive-word-policy-production-deployment-status.md)；测试策略接口用例与临时渠道清理见 [2026-09-01 敏感词分级策略测试发布状态](../operations/2026-09-01-sensitive-word-policy-test-deployment-status.md)。
- Stripe 账期代码、Sandbox 首购、历史账单日期恢复、Automatic Tax 对象回读及未验证边界见 [2026-09-01 Stripe 订阅账期测试发布记录](../operations/2026-09-01-stripe-subscription-period-test-deployment.md)。

## 数据库和持久化

- 主库在未配置 `SQL_DSN` 时使用 SQLite；配置 PostgreSQL DSN 时使用 PostgreSQL，其他非 local DSN 按 MySQL 处理。主库不接受 ClickHouse。
- `LOG_SQL_DSN` 未配置时日志复用主库；配置后可使用 SQLite、MySQL、PostgreSQL 或 ClickHouse。相关选择逻辑位于 `model/main.go`。
- 默认 `docker-compose.yml` 启用 PostgreSQL 15 与 Redis，挂载应用数据、日志和 PostgreSQL volume；MySQL、独立日志库和 ClickHouse 是注释示例，不代表默认启用。
- Compose 文件包含演示凭据并明确要求生产更换；Facts 不复制其值，当前生产是否已更换为待定。

## 迁移

- master 节点初始化主库时执行 `migrateDB`，包含显式兼容迁移和 GORM `AutoMigrate`；非 master 节点在连接配置完成后跳过迁移。
- 独立日志库由 `migrateLOGDB` 迁移；ClickHouse 使用专门建表与 TTL 同步逻辑。
- 当前迁移代码和相关单元测试通过，但本次未在真实 MySQL、PostgreSQL 或 ClickHouse 实例上运行迁移。

## 备份和恢复

- 已确认（文档边界）：2026-08-30 应用发布保留了上一镜像配置和 Compose 备份，应用回滚路径已写明，但该次发布的 `Rollback` 为 `Not executed`。
- 已确认：2026-09-01 测试发布在 `/srv/new-api-test/backups/sensitive-policy-20260901T064123Z-fc6ebe122e` 保留发布前 Compose、测试库 custom-format dump 和 `pg_restore --list` 清单；三个文件的 SHA-256 与 `534` 行 restore 清单已回读。该次测试回滚未执行。
- 已确认：2026-09-01 正式发布在 `/srv/new-api/backups/new-api-release-20260901T132333Z-fc6ebe122e` 保留发布前 `images.env`、Compose、渲染配置、运行身份、敏感词 option CSV/摘要、PostgreSQL custom-format dump、`pg_restore --list` 和 `SHA256SUMS`；发布后全部校验通过，数据库 dump SHA-256 为 `7b9c6b2e23e7612ca903e83479094cc77b2491baa9c877ab1535eff480d8084c`。该次正式回滚未执行。
- 已确认：Stripe 账期测试发布在 `/srv/new-api-test/backups/new-api-test-20260901T062743Z-89e4d3a911/` 保留 `compose.yaml.before` 和 PostgreSQL custom-format `newapi_test.before.dump`；两者 SHA-256 分别为 `62fd95c4269a1ad53a76fc5f792514e731b55a4fef2ca5ac9f3a3d5212c3b76c` 与 `2ea2551b739bfa878c2fe626c3baffb13d7145cd8419c37ca6a6bb19e8afd43a`。dump 大小为 `22,582,167` bytes、mode `600`，`pg_restore --list` 为 `534` 行；回滚未执行。
- 待定：本次正式发布已有 PostgreSQL 单次 dump 和配置/option 备份，但生产 Redis、`/data` 和日志 volume 的完整备份计划、保留期、恢复命令及最近恢复演练结果仍未确认；应用镜像回滚不能替代数据库恢复。

## 运维和监控

- Compose 健康检查访问 `http://localhost:3000/api/status` 并要求响应包含成功标记。
- 容器可将日志写入 `/app/logs`；主程序支持可选 pprof、Pyroscope、错误日志、节点实例上报和计划任务历史。
- Docker 发布 workflow 针对 amd64/arm64 构建并推送多架构镜像，生成 provenance/SBOM 并用 cosign 签名；Release workflow 生成多平台二进制及 SHA256 checksums。
- 上述为仓库自动化事实；远端 workflow 最近运行、镜像存在性、签名读取和生产监控告警均为待定。

## 回滚或降级

- 已确认（截至 2026-09-01 的文档边界）：敏感词正式发布可恢复 `/srv/new-api/backups/new-api-release-20260901T132333Z-fc6ebe122e/images.env.before` 并只重建 `new-api`；Stripe 账期测试发布和敏感词测试发布分别可恢复其 Compose 备份并只重建 `new-api-test`。
- 已确认：本次正式发布删除了持久化敏感词 option。仅回滚镜像不会恢复发布前的 option 覆盖，若要恢复完整策略语义，必须单独审核并从 `sensitive-options.before.csv` 恢复三项 option；完整数据库恢复则使用 custom-format dump 制定方案。
- 待定：上述应用与数据库回滚均未执行，不能写成已演练；数据库或数据完整性事故必须走单独审核的恢复方案。

## 已知约束

- 最终容器暴露 3000 端口并以 `/new-api` 为入口；前端必须先构建到 `web/dist` 才能嵌入根二进制。
- README 声明容器部署支持 amd64/arm64，远端数据库要求 MySQL >= 5.7.8 或 PostgreSQL >= 9.6；本次未在这些最低版本上运行兼容测试。
- 主库和日志库的密钥/连接串只能由运行环境提供，不能写入 Facts 或提交真实 `.env`。

## 待确认事项

- 待定：生产数据库的全量数据完整性、Redis 缓存业务正确性、证书自动续期、容量和防火墙完整规则；当前检查只覆盖上述只读探针。
- 待定：生产备份、数据库恢复演练、告警、容量、日志保留与应用回滚演练。
- 待定：GitHub Actions 最近运行、Docker Hub 镜像、cosign 签名和 release 产物的远端读回。
