# 部署和运行事实

本文件记录部署方式、环境、数据库、迁移、备份、运维和回滚约束。不要写尚未验证的部署方案为当前事实。

## 环境

| 环境 | 当前状态 | 入口或地址 | 确认来源 | 最近确认 |
| --- | --- | --- | --- | --- |
| 本地 | 待定：仓库提供 Compose 后端依赖和 Rsbuild 前端启动入口，本次未启动服务 | 后端默认 `http://localhost:3000`；前端开发默认 `http://localhost:5173` | `makefile`、`docker-compose.dev.yml` | 2026-08-31 |
| CI | 已确认：PR workflow 配置 root/relaykit vet、build、test 与前端 typecheck、test；远端最近一次运行状态待定 | GitHub Actions | `.github/workflows/ci.yml` | 2026-08-31 |
| 预览 | 不适用：当前仓库未发现独立 preview 环境配置或 preview 部署 workflow | 不适用 | `.github/workflows/`、Docker 与项目配置扫描 | 2026-08-31 |
| 测试 | 已确认（2026-09-08 09:35:58，Asia/Shanghai）：GreenCloud 测试应用运行提交 `369141e27` 的不可变 `linux/amd64` 镜像 `new-api:new-api-test-20260908T012416Z-369141e27`，容器 `running/healthy`、重启次数 `0`；运行时不含订阅 Checkout PMC 变量，继续使用 Stripe 默认动态支付方式。测试本机与 `test.tryvalo.com` 状态接口均为 HTTP `200`；本机公开套餐接口返回 Sandbox Plan `2/3/4`，均标记 Stripe Checkout 可用。真实 Sandbox 付款 E2E 待定 | `test.tryvalo.com` | GreenCloud Docker/Compose、运行时环境变量存在性检查、本机和公网 HTTP、公开套餐 API、备份校验 | 2026-09-08 |
| 生产 | 已确认（2026-09-08 10:25:42 发布，Asia/Shanghai）：GreenCloud 正式应用运行提交 `3186b5c8d` 的 `linux/amd64` 镜像 `new-api:new-api-release-20260908-managed-off-3186b5c8d`，容器 `running/healthy`、重启次数 `0`。充值及套餐 Checkout 显式退出 Managed Payments；订阅 PMC 环境配置保持不变。PostgreSQL、Redis 未重建；本机及两个正式状态入口均 HTTP `200`。两笔新 Live Checkout 已显示微信，付款闭环仍待定 | `api.tryvalo.com`、`new.tryvalo.com`、`tryvalo.com` | GreenCloud Docker/Compose、HTTP、Stripe Live Session API 与 Checkout 页面；[发布记录](../operations/2026-09-08-stripe-managed-payments-opt-out-release.md) | 2026-09-08 |

## 当前远端快照

- 最新发布为 2026-09-08 10:25:42（Asia/Shanghai）的正式 Managed Payments opt-out，镜像 ID `sha256:55bf2045ba37042bcf9880556a307a1009e7a379717600213604442387025303`。测试未更新。备份、配置摘要、健康、未付款 Checkout 及回滚边界见 [Managed Payments 退出发布记录](../operations/2026-09-08-stripe-managed-payments-opt-out-release.md)；下列较早记录不表示测试与正式目前仍运行同一镜像。

- 2026-09-07 在 GreenCloud 主机 `nemo-Phoenix` 现场回读：测试 `new-api-test` 运行 `new-api:new-api-test-20260907T135216Z-22a1b1d82`，正式 `new-api` 运行 `new-api:new-api-release-20260907T135216Z-22a1b1d82`；两者镜像 ID 均为 `sha256:44fe064881dc38b72405976d72e8646fb1ff0ed6ce62ba909e253be4c5f388f6`，状态 `running/healthy`、重启次数 `0`。传输包 SHA-256 为 `8ae2ea1165cac2fbfa7e25278b198f1d416a80b9bf93d54da389986f05ec89d4`。
- 当前回读时测试和正式本机状态端点，以及 `test.tryvalo.com`、`tryvalo.com`、`api.tryvalo.com`、`new.tryvalo.com` 的 `/api/status` 均为 HTTP `200`；测试和正式的 `/wallet`、`/login` 均为 HTTP `200`。直接 HTTPS 首请求均新建连接并返回 `200`；此前同次发布验收中的复用连接同样返回 `200`。
- 正式 `new-api-postgres` 与 `new-api-redis` 当前均为 `running/healthy`、重启次数 `0`，其容器 ID 与启动时间保持 2026-07 的原值，证明 2026-09-07 正式发布仅重建 `new-api`。以上证明容器和所执行的依赖检查，不等于数据库全量数据正确或缓存业务语义完整。
- 2026-09-07 数据库只读回读确认：测试保留禁用且不公开的历史 Plan `1`，公开 Plan `2/3/4` 映射 Sandbox one-time Price；正式 Plan `1/2/3` 映射 Live one-time Price，`subscription_orders=0`、active `user_subscriptions=0`。完整 Price 映射与交易边界见 `docs/operations/2026-09-07-stripe-one-time-cutover.md`。
- 当前边缘路径已现场确认：`api.tryvalo.com` 解析到 Zgo `64.83.30.150`，Zgo Caddy `v2.11.4` 为 active，并固定反代到 GreenCloud `173.249.203.66`，Host 与 TLS SNI 均为 `origin-api.tryvalo.com`；GreenCloud 只允许该 Zgo 地址访问 `origin-api.tryvalo.com`，再转发到 `127.0.0.1:3000`。`new.tryvalo.com` 直接解析到 GreenCloud，并由同一 GreenCloud Caddy 转发到生产应用。
- 2026-09-07 发布仅重建 `new-api-test` 与 `new-api`，没有重建生产 PostgreSQL/Redis，也没有改变 DNS、代理、防火墙、API 密钥、webhook secret、Stripe Tax 或支付方式配置。
- 已确认（2026-09-08 00:48，Asia/Shanghai）：GreenCloud 正式基础 Compose 已更新到源码提交 `7eee48288` 的配置摘要（基础文件 SHA-256 `639aff8994bef03e69bafde58093fe013ac9ed0b01d73586f8e89a274e44d279`）。可选的 `cpacodexkeeper` 已移至显式的 `compose.keeper.yaml`（SHA-256 `5cbcefe4a9f085e3d92e0006954d34afcb2dcc3de3849a0cf547d3d3161c3d67`），使基础应用发布不再依赖 keeper 镜像变量。测试和正式基础 Compose 均已通过 `config -q`；正式显式 keeper overlay 亦通过配置校验，但未启动 keeper。两个应用的 `up --no-deps --no-build --pull never` 复核均保持已有容器，运行镜像、启动时间和重启次数未变。
- 已确认（2026-09-08 09:35:58～09:40，Asia/Shanghai）：提交 `369141e27` 的同一不可变镜像 ID `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b` 已先后发布测试与正式。测试仅更新 Compose 镜像 tag，运行时 PMC 变量保持不存在；正式仅更新 `images.env` 的应用镜像和 `new-api.env` 的订阅 Checkout PMC，未记录 PMC 标识。两次均只重建应用；正式 PostgreSQL、Redis 的启动时间继续为 2026-07。测试、`api.tryvalo.com` 与 `new.tryvalo.com` 的状态接口均为 HTTP `200`。完整执行、配置摘要和回滚路径见 [2026-09-08 Stripe Payment Method Configuration 发布记录](../operations/2026-09-08-stripe-payment-method-configuration-release.md)。
- 当前一次性套餐发布的身份、备份、健康证据与回滚边界见 [2026-09-07 Stripe 一次性付款预配置与切换记录](../operations/2026-09-07-stripe-one-time-cutover.md)。较早的 telemetry、敏感词策略和 Stripe 账期发布记录仍分别由对应操作文档承载。
- Stripe 账期代码、Sandbox 首购、历史账单日期恢复、Automatic Tax 对象回读及未验证边界见 [2026-09-01 Stripe 订阅账期测试发布记录](../operations/2026-09-01-stripe-subscription-period-test-deployment.md)。

## 2026-09-08 GreenCloud Compose 配置发布

- 范围：只修复 GreenCloud `new-api` 基础 Compose 对未启用 `cpacodexkeeper` 的不必要依赖；测试 Compose 与应用镜像未变。涉及主机为 `173.249.203.66`，入口仍为 `test.tryvalo.com`、`api.tryvalo.com`、`new.tryvalo.com`。未修改 DNS、Caddy、代理、防火墙、数据库、Redis、应用环境变量、Stripe 或支付配置。
- 已执行：先在 `/srv/new-api/backups/new-api-compose-config-20260907T164800Z-7eee48288` 保存发布前 `compose.yaml`，再写入基础 Compose 和显式 keeper overlay；旧基础配置的 SHA-256 为 `aa8ec0bca4e11b135a62f5d626509efd874c91379244d994aa0b47cd901646d5`。源码提交已推送到 `origin/main`。
- 已验证：测试与正式基础 Compose 的 `config -q` 均通过；正式基础与 keeper overlay 的显式 `--profile keeper config -q` 通过。`new-api` 与 `new-api-test` 均为 `running/healthy`、重启 `0`，继续使用同一镜像 ID `sha256:44fe064881dc38b72405976d72e8646fb1ff0ed6ce62ba909e253be4c5f388f6`。两个本机状态端点及三个公网状态端点均返回 HTTP `200` 和 `success: true`；测试与正式公开套餐 API 均返回三档 CNY `259/599/1099`、一个月有效期及 `stripe_checkout_available=true`。
- 未回滚：发布前基础 Compose 保留在上述备份目录；如需回退配置，先将 `compose.yaml.before` 恢复为 `/srv/new-api/compose.yaml`，重新执行基础 Compose 配置校验，再仅重建 `new-api`。本次未执行回滚。
- 已知边界：这次只证明配置、容器、状态接口和公开套餐可用性；Stripe Sandbox/Live 真实付款、Webhook、权益、退款和争议 E2E 仍待定。

## 2026-09-08 Stripe Payment Method Configuration 发布

- 范围：提交 `369141e27` 将订阅 Checkout 的可选 Payment Method Configuration 限定为环境变量控制。测试环境保持该变量不存在；正式环境配置 Live 账户本地 PMC。测试与正式均升级到同一不可变镜像；没有更改充值 Checkout、Stripe Dashboard 支付方式、Webhook、Tax、DNS、Caddy、代理、防火墙、数据库或 Redis。
- 已执行：测试应用仅替换镜像 tag 并重建 `new-api-test`；正式先备份 `images.env`、`new-api.env` 和 Compose，再替换镜像 tag 并添加正式 PMC 变量，随后仅重建 `new-api`。两套 Compose 均在应用重建前通过 `config -q`。
- 已验证：两个应用均为 `running/healthy`、重启次数 `0`、镜像 ID 为 `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b`；测试运行时变量不存在，正式运行时变量为已配置但未输出；测试和正式本机状态接口，以及 `test.tryvalo.com`、`api.tryvalo.com`、`new.tryvalo.com` 均为 HTTP `200`。正式 PostgreSQL/Redis 继续运行且未重建。
- 未回滚：测试和正式发布前配置备份均已校验。回退时分别恢复备份中的测试 Compose，或正式 `images.env.before`、`new-api.env.before` 和 `compose.yaml.before`，重新执行 Compose 配置校验后只重建对应应用；不得重建 PostgreSQL、Redis 或 keeper。真实新 Checkout 的微信展示、付款、Webhook、权益、退款和争议不由本次健康检查证明。

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
- 已确认：2026-09-03 telemetry/SEO 正式发布在 `/srv/new-api/backups/new-api-release-20260903T094447Z-e40d88d1535` 保留发布前配置与 Compose 材料；应用镜像包 SHA-256 为 `8fbbcbb01bd9ecfb4ab3818c040a2a1a1fd15cc636e7334c86ff9a67193ede7e`。该次正式回滚未执行，详见上述 telemetry 发布记录。
- 已确认：Stripe 账期测试发布在 `/srv/new-api-test/backups/new-api-test-20260901T062743Z-89e4d3a911/` 保留 `compose.yaml.before` 和 PostgreSQL custom-format `newapi_test.before.dump`；两者 SHA-256 分别为 `62fd95c4269a1ad53a76fc5f792514e731b55a4fef2ca5ac9f3a3d5212c3b76c` 与 `2ea2551b739bfa878c2fe626c3baffb13d7145cd8419c37ca6a6bb19e8afd43a`。dump 大小为 `22,582,167` bytes、mode `600`，`pg_restore --list` 为 `534` 行；回滚未执行。
- 已确认：2026-09-07 测试和正式切换的备份目录分别为 `/srv/new-api-test/backups/new-api-test-20260907T135216Z-22a1b1d82` 与 `/srv/new-api/backups/new-api-release-20260907T135216Z-22a1b1d82`；两处 `SHA256SUMS` 均已重新校验通过。该次回滚未执行。
- 已确认：2026-09-08 PMC 发布在正式 `/srv/new-api/backups/new-api-release-20260908T012416Z-369141e27` 保存发布前 `images.env`、`new-api.env` 和 `compose.yaml`，其 `SHA256SUMS` 已通过校验；测试 `/srv/new-api-test/backups/new-api-release-20260908T012416Z-369141e27` 保存发布前 Compose。两次回滚均未执行。
- 待定：上述发布虽有已校验备份目录，但生产 Redis、`/data` 和日志 volume 的完整备份计划、保留期、恢复命令及最近恢复演练结果仍未确认；应用镜像回滚不能替代数据库恢复。

## 运维和监控

- Compose 健康检查访问 `http://localhost:3000/api/status` 并要求响应包含成功标记。
- 容器可将日志写入 `/app/logs`；主程序支持可选 pprof、Pyroscope、错误日志、节点实例上报和计划任务历史。
- Docker 发布 workflow 针对 amd64/arm64 构建并推送多架构镜像，生成 provenance/SBOM 并用 cosign 签名；Release workflow 生成多平台二进制及 SHA256 checksums。
- 上述为仓库自动化事实；远端 workflow 最近运行、镜像存在性、签名读取和生产监控告警均为待定。

## 回滚或降级

- 已确认（截至 2026-09-03 的文档边界）：telemetry/SEO 正式发布可恢复 `/srv/new-api/backups/new-api-release-20260903T094447Z-e40d88d1535/images.env.before` 并只重建 `new-api`；敏感词正式发布与 Stripe 账期/敏感词测试发布分别保留各自的历史回滚材料。
- 已确认：本次正式发布删除了持久化敏感词 option。仅回滚镜像不会恢复发布前的 option 覆盖，若要恢复完整策略语义，必须单独审核并从 `sensitive-options.before.csv` 恢复三项 option；完整数据库恢复则使用 custom-format dump 制定方案。
- 已确认：2026-09-07 一次性套餐发布如需回滚，必须在暂停购买后同时恢复对应备份中的应用镜像配置与旧 recurring Price 映射，并只重建应用；旧代码不得接入新 one-time Price，新代码不得接入旧 recurring Price。
- 已确认：2026-09-08 PMC 发布如需回滚，必须恢复同次备份中的应用镜像配置和正式 `new-api.env`，然后只重建对应应用；仅回滚镜像而保留 PMC 配置会继续将旧代码的订阅 Checkout 请求传给该 PMC，故不是完整回退。
- 待定：上述应用与数据库回滚均未执行，不能写成已演练；数据库或数据完整性事故必须走单独审核的恢复方案。

## 已知约束

- 最终容器暴露 3000 端口并以 `/new-api` 为入口；前端必须先构建到 `web/dist` 才能嵌入根二进制。
- README 声明容器部署支持 amd64/arm64，远端数据库要求 MySQL >= 5.7.8 或 PostgreSQL >= 9.6；本次未在这些最低版本上运行兼容测试。
- 主库和日志库的密钥/连接串只能由运行环境提供，不能写入 Facts 或提交真实 `.env`。

## 待确认事项

- 待定：生产数据库的全量数据完整性、Redis 缓存业务正确性、证书自动续期、容量和防火墙完整规则；当前检查只覆盖上述只读探针。
- 待定：生产备份、数据库恢复演练、告警、容量、日志保留与应用回滚演练。
- 待定：GitHub Actions 最近运行、Docker Hub 镜像、cosign 签名和 release 产物的远端读回。
