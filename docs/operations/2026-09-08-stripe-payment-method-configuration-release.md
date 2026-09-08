# Stripe Payment Method Configuration 测试与正式发布记录

## 范围与状态

- 发布 ID：`new-api-release-20260908T012416Z-369141e27`
- 代码提交：`369141e2719c40c381577184eb3f526c0b9ac7de`
- 执行时间：2026-09-08 09:35:58～09:40（Asia/Shanghai；容器启动时间以 UTC 记录）
- 目标：GreenCloud `173.249.203.66` 的 `new-api-test` 与 `new-api`；测试入口为 `test.tryvalo.com`，正式入口为 `api.tryvalo.com` 与 `new.tryvalo.com`。
- 变更：订阅 Checkout 读取可选环境变量 `STRIPE_SUBSCRIPTION_PAYMENT_METHOD_CONFIGURATION`。非空时传入 Stripe `payment_method_configuration`；空或未设置时不传，保留 Stripe 默认动态支付方式。变量只影响订阅 Checkout，不影响充值 Checkout。
- 安全边界：正式环境填写 Live 账户本地 PMC；其值没有写入源代码、提交、Facts 或本记录。测试环境保持该变量不存在。

## 计划

1. 在本地对提交 `369141e27` 构建 `linux/amd64` 镜像并校验压缩包哈希。
2. 导入同一镜像，先只重建测试应用，确认运行时没有 PMC 变量以及本机/公网健康。
3. 备份正式镜像与应用环境配置；仅更新正式镜像引用和正式 PMC 变量，配置校验后只重建 `new-api`。
4. 回读容器身份、变量存在性、依赖状态、本机与公网健康；不创建 Checkout 或付款。

## 已执行

1. 本地构建并导出镜像 `new-api:new-api-release-20260908T012416Z-369141e27`；镜像 ID 为 `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b`，平台为 `linux/amd64`。传输包 `/tmp/new-api-pmc-20260908T012416Z-369141e27/new-api-release-20260908T012416Z-369141e27.tar.gz` 的 SHA-256 为 `91f64d88fca0ed17cf1fc480e7e9cfbdba439e7cff255986b24b6d909245f259`；本地和远端均通过 `gzip -t` 与哈希核验。
2. 将该镜像加载到 GreenCloud，并为测试加 tag `new-api:new-api-test-20260908T012416Z-369141e27`。测试发布前将 `/srv/new-api-test/compose.yaml` 备份到 `/srv/new-api-test/backups/new-api-release-20260908T012416Z-369141e27/compose.yaml.before`，然后只替换测试应用镜像并执行 `docker compose -f /srv/new-api-test/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api-test`。
3. 正式发布前将 `/srv/new-api/env/images.env`、`/srv/new-api/env/new-api.env` 与 `/srv/new-api/compose.yaml` 备份到 `/srv/new-api/backups/new-api-release-20260908T012416Z-369141e27`，并通过该目录的 `SHA256SUMS` 校验。随后将正式 `NEW_API_IMAGE` 指向本次不可变 tag，并在正式 `new-api.env` 添加订阅 Checkout PMC 变量。`docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml config -q` 通过后，只重建 `new-api`。
4. 没有重建 PostgreSQL、Redis 或 keeper；没有修改 Stripe Dashboard 支付方式、Webhook、Tax、DNS、Caddy、代理或防火墙。

## 已验证

| 检查 | 测试 | 正式 |
| --- | --- | --- |
| 应用镜像 | `new-api:new-api-test-20260908T012416Z-369141e27` | `new-api:new-api-release-20260908T012416Z-369141e27` |
| 镜像 ID | `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b` | 相同 |
| 容器状态 | `running/healthy`，重启 `0` | `running/healthy`，重启 `0` |
| 订阅 Checkout PMC 运行时状态 | 不存在 | 已配置，未输出标识 |
| 本机状态接口 | `127.0.0.1:3001/api/status` HTTP `200` | `127.0.0.1:3000/api/status` HTTP `200` |
| 公网状态接口 | `https://test.tryvalo.com/api/status` HTTP `200` | `https://api.tryvalo.com/api/status`、`https://new.tryvalo.com/api/status` 均为 HTTP `200` |
| 套餐接口 | 本机返回 3 个 Plan `2/3/4`，均 `stripe_checkout_available=true` | 本机返回 3 个 Plan `1/2/3`，均 `stripe_checkout_available=true` |

正式 PostgreSQL 与 Redis 均保持 `running/healthy`，启动时间分别仍为 `2026-07-11T01:00:44Z` 与 `2026-07-12T00:26:12Z`。当前配置 SHA-256 为：测试 Compose `4e20b6f01b83bac689862fe9eb07e1d6f4b00517400c91851dd410db276a78c4`，正式 Compose `639aff8994bef03e69bafde58093fe013ac9ed0b01d73586f8e89a274e44d279`，正式镜像环境 `dbad8e894fc879b03e88beadbe8848450b2b8f56681fe538fb3e633b6ca1fb4d`，正式应用环境 `b8933864ea508bccb85ed25e78df29704dcb4bad616908427b0caeac72acdf9b`。环境文件内容和 PMC 标识未记录。

## 未回滚与回滚路径

- 未执行回滚。
- 测试回滚：恢复 `/srv/new-api-test/backups/new-api-release-20260908T012416Z-369141e27/compose.yaml.before`，通过测试 Compose `config -q` 后只重建 `new-api-test`。
- 正式回滚：恢复同名正式备份中的 `images.env.before`、`new-api.env.before` 与 `compose.yaml.before`，通过正式 Compose `config -q` 后只重建 `new-api`。
- 两种回滚均不得重建 PostgreSQL、Redis 或 keeper；应用镜像回滚不替代数据库恢复。

## 已知边界

- 本次证明了代码、镜像、环境变量边界、容器健康和状态/套餐接口，并不证明 Stripe 对某一笔新 Live Checkout 实际展示微信支付。
- 旧 `cs_live_...` Checkout Session 是历史对象，不能证明本次代码或本次 PMC 生效。应单独、明确授权后创建一笔新的 Live Checkout，回读其 `payment_method_types` 与 `payment_method_configuration_details`，再决定是否进行真实付款、Webhook、权益、退款与争议验收。
