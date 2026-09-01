# 部署和运行事实

本文件记录部署方式、环境、数据库、迁移、备份、运维和回滚约束。不要写尚未验证的部署方案为当前事实。

## 环境

| 环境 | 当前状态 | 入口或地址 | 确认来源 | 最近确认 |
| --- | --- | --- | --- | --- |
| 本地 | 待定：仓库提供 Compose 后端依赖和 Rsbuild 前端启动入口，本次未启动服务 | 后端默认 `http://localhost:3000`；前端开发默认 `http://localhost:5173` | `makefile`、`docker-compose.dev.yml` | 2026-08-31 |
| CI | 已确认：PR workflow 配置 root/relaykit vet、build、test 与前端 typecheck、test；远端最近一次运行状态待定 | GitHub Actions | `.github/workflows/ci.yml` | 2026-08-31 |
| 预览 | 不适用：当前仓库未发现独立 preview 环境配置或 preview 部署 workflow | 不适用 | `.github/workflows/`、Docker 与项目配置扫描 | 2026-08-31 |
| 生产 | 已确认（2026-09-01 09:27～09:33，Asia/Shanghai）：GreenCloud 测试与生产仍运行同一不可变 `linux/amd64` 应用镜像；两个应用容器、生产 PostgreSQL/Redis 均为 `running/healthy`、重启次数 `0` | `test.tryvalo.com`、`api.tryvalo.com`、`new.tryvalo.com` | GreenCloud Docker/PostgreSQL 只读回读；当前公网 HTTP/DNS；Zgo 与 GreenCloud Caddy 只读回读 | 2026-09-01 |

## 当前远端快照

- 2026-09-01 09:27～09:33（Asia/Shanghai）在 GreenCloud 主机 `nemo-Phoenix` 现场只读回读：生产 `new-api` 与测试 `new-api-test` 仍分别运行 `new-api-release-20260830T025223Z-f96bf33b80` 和 `new-api-test-20260830T025223Z-f96bf33b80`，二者镜像 ID 都是 `sha256:032ba62c4df47fe53bb43e5b8894f0ecca0c9d23ae09d6c79e123aa42c820f1a`，OCI revision 均为 `f96bf33b80dfeca9b025a94651fb68db492dc8a7`。
- 两个应用容器均为 `running/healthy`、health failing streak `0`、重启次数 `0`；本机 `127.0.0.1:3000` 和 `127.0.0.1:3001` 的 `/api/status` 均返回 HTTP `200`。
- 生产 `new-api-postgres` 与 `new-api-redis` 均为 `running/healthy`、health failing streak `0`、重启次数 `0`。PostgreSQL `pg_isready` 返回 accepting connections，并完成了生产库只读聚合；Redis 最近五次容器健康检查均以 exit `0` 完成。以上证明容器和所执行的依赖检查，不等于数据库全量数据正确或缓存业务语义完整。
- 当前公网 `api.tryvalo.com`、`new.tryvalo.com`、`test.tryvalo.com` 的 `/api/status` 均返回 HTTP `200`。`test.tryvalo.com` 当前解析到 Cloudflare edge 并返回 Cloudflare 响应头。
- 当前边缘路径已现场确认：`api.tryvalo.com` 解析到 Zgo `64.83.30.150`，Zgo Caddy `v2.11.4` 为 active，并固定反代到 GreenCloud `173.249.203.66`，Host 与 TLS SNI 均为 `origin-api.tryvalo.com`；GreenCloud 只允许该 Zgo 地址访问 `origin-api.tryvalo.com`，再转发到 `127.0.0.1:3000`。`new.tryvalo.com` 直接解析到 GreenCloud，并由同一 GreenCloud Caddy 转发到生产应用。
- 本次没有更改容器、DNS、Caddy、数据库、Redis 或渠道配置，也没有执行真实支付、退款或主动付费模型探测。

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
- 待定：生产数据库、Redis、`/data` 和日志 volume 的完整备份计划、保留期、恢复命令及最近恢复演练结果；应用镜像回滚不能替代数据库恢复。

## 运维和监控

- Compose 健康检查访问 `http://localhost:3000/api/status` 并要求响应包含成功标记。
- 容器可将日志写入 `/app/logs`；主程序支持可选 pprof、Pyroscope、错误日志、节点实例上报和计划任务历史。
- Docker 发布 workflow 针对 amd64/arm64 构建并推送多架构镜像，生成 provenance/SBOM 并用 cosign 签名；Release workflow 生成多平台二进制及 SHA256 checksums。
- 上述为仓库自动化事实；远端 workflow 最近运行、镜像存在性、签名读取和生产监控告警均为待定。

## 回滚或降级

- 已确认（截至 2026-08-30 的文档边界）：普通应用回滚恢复上一镜像配置并只重建 `new-api`；测试环境恢复上一 Compose 配置并只重建 `new-api-test`。该次发布无 schema 或生产数据变更。
- 待定：上述回滚未执行，不能写成已演练；数据库或数据完整性事故必须走单独审核的恢复方案。

## 已知约束

- 最终容器暴露 3000 端口并以 `/new-api` 为入口；前端必须先构建到 `web/dist` 才能嵌入根二进制。
- README 声明容器部署支持 amd64/arm64，远端数据库要求 MySQL >= 5.7.8 或 PostgreSQL >= 9.6；本次未在这些最低版本上运行兼容测试。
- 主库和日志库的密钥/连接串只能由运行环境提供，不能写入 Facts 或提交真实 `.env`。

## 待确认事项

- 待定：生产数据库的全量数据完整性、Redis 缓存业务正确性、证书自动续期、容量和防火墙完整规则；当前检查只覆盖上述只读探针。
- 待定：生产备份、数据库恢复演练、告警、容量、日志保留与应用回滚演练。
- 待定：GitHub Actions 最近运行、Docker Hub 镜像、cosign 签名和 release 产物的远端读回。
